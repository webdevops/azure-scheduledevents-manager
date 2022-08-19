package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"runtime"
	"strings"
	"sync/atomic"

	"github.com/jessevdk/go-flags"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"

	"github.com/webdevops/azure-scheduledevents-manager/azuremetadata"
	"github.com/webdevops/azure-scheduledevents-manager/config"
	"github.com/webdevops/azure-scheduledevents-manager/drainmanager"
	"github.com/webdevops/azure-scheduledevents-manager/manager"
)

const (
	Author = "webdevops.io"
)

var (
	argparser *flags.Parser

	// Git version information
	gitCommit = "<unknown>"
	gitTag    = "<unknown>"

	readyzStatus = int64(0)
	drainzStatus = int64(0)
)

var opts config.Opts

func main() {
	initArgparser()

	log.Infof("starting azure-scheduledevents-manager v%s (%s; %s; by %v)", gitTag, gitCommit, runtime.Version(), Author)
	log.Info(string(opts.GetJson()))

	log.Infof("starting azure metadata client")
	azureMetadataClient := &azuremetadata.AzureMetadata{
		ScheduledEventsUrl:  opts.Azure.ScheduledEventsApiUrl,
		InstanceMetadataUrl: opts.Azure.InstanceApiUrl,
		Timeout:             &opts.Azure.Timeout,
		UserAgent:           fmt.Sprintf("azure-scheduledevents-manager/%v", gitTag),
	}
	azureMetadataClient.Init()

	if opts.Instance.VmNodeName == "" {
		instanceMetadata, err := azureMetadataClient.FetchInstanceMetadata()
		if err != nil {
			panic(err)
		}
		log.Infof("detecting VM resource name")
		opts.Instance.VmNodeName = instanceMetadata.Compute.Name
	} else {
		log.Infof("using VM resource name from env")
	}
	log.Infof("using VM node: %v", opts.Instance.VmNodeName)

	scheduledEventsManager := manager.ScheduledEventsManager{
		Conf:                opts,
		AzureMetadataClient: azureMetadataClient,
	}
	scheduledEventsManager.Init()
	scheduledEventsManager.OnClear = func() {
		atomic.StoreInt64(&readyzStatus, 0)
	}
	scheduledEventsManager.OnScheduledEvent = func() {
		atomic.StoreInt64(&readyzStatus, 1)
	}
	scheduledEventsManager.OnAfterDrainEvent = func() {
		atomic.StoreInt64(&drainzStatus, 1)
	}

	if opts.Drain.Enable {
		switch opts.Drain.Mode {
		case "kubernetes":
			log.Infof("start \"kubernetes\" mode")
			log.Infof("using Kubernetes nodename: %v", opts.Kubernetes.NodeName)
			drain := &drainmanager.DrainManagerKubernetes{
				Conf: opts,
			}
			drain.SetInstanceName(opts.Kubernetes.NodeName)
			scheduledEventsManager.DrainManager = drain
		case "command":
			log.Infof("start \"command\" mode")
			drain := drainmanager.DrainManagerCommand{
				Conf: opts,
			}
			drain.SetInstanceName(opts.Instance.VmNodeName)
			scheduledEventsManager.DrainManager = &drain
		default:
			log.Panicf("drain mode \"%v\" is not valid", opts.Drain.Mode)
		}

		scheduledEventsManager.DrainManager.Test()
	}

	log.Infof("starting manager")
	scheduledEventsManager.Start()

	log.Infof("starting http server on %s", opts.General.ServerBind)
	startHttpServer()
}

func initArgparser() {
	argparser = flags.NewParser(&opts, flags.Default)
	_, err := argparser.Parse()

	// check if there is an parse error
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			fmt.Println()
			argparser.WriteHelp(os.Stdout)
			os.Exit(1)
		}
	}

	// verbose level
	if opts.Logger.Verbose {
		log.SetLevel(log.DebugLevel)
	}

	// debug level
	if opts.Logger.Debug {
		log.SetReportCaller(true)
		log.SetLevel(log.TraceLevel)
		log.SetFormatter(&log.TextFormatter{
			CallerPrettyfier: func(f *runtime.Frame) (string, string) {
				s := strings.Split(f.Function, ".")
				funcName := s[len(s)-1]
				return funcName, fmt.Sprintf("%s:%d", path.Base(f.File), f.Line)
			},
		})
	}

	// json log format
	if opts.Logger.LogJson {
		log.SetReportCaller(true)
		log.SetFormatter(&log.JSONFormatter{
			DisableTimestamp: true,
			CallerPrettyfier: func(f *runtime.Frame) (string, string) {
				s := strings.Split(f.Function, ".")
				funcName := s[len(s)-1]
				return funcName, fmt.Sprintf("%s:%d", path.Base(f.File), f.Line)
			},
		})
	}

	// validate instanceUrl url
	instanceUrl, err := url.Parse(opts.Azure.InstanceApiUrl)
	if err != nil {
		fmt.Println(err)
		fmt.Println()
		argparser.WriteHelp(os.Stdout)
		os.Exit(1)
	}

	switch strings.ToLower(instanceUrl.Scheme) {
	case "http":
		break
	case "https":
		break
	default:
		fmt.Printf("ApiURL scheme not allowed (must be http or https), got %v\n", opts.Azure.InstanceApiUrl)
		fmt.Println()
		argparser.WriteHelp(os.Stdout)
		os.Exit(1)
	}

	// validate scheduledEventsUrl url
	scheduledEventsUrl, err := url.Parse(opts.Azure.ScheduledEventsApiUrl)
	if err != nil {
		fmt.Println(err)
		fmt.Println()
		argparser.WriteHelp(os.Stdout)
		os.Exit(1)
	}

	// validate --api-url scheme
	switch strings.ToLower(scheduledEventsUrl.Scheme) {
	case "http":
		break
	case "https":
		break
	default:
		fmt.Printf("ApiURL scheme not allowed (must be http or https), got %v\n", opts.Azure.ScheduledEventsApiUrl)
		fmt.Println()
		argparser.WriteHelp(os.Stdout)
		os.Exit(1)
	}

	if opts.Drain.Enable {
		switch opts.Drain.Mode {
		case "kubernetes":
			if opts.Kubernetes.NodeName == "" {
				fmt.Println("kubernetes node name must be set in kubernetes drain mode")
				fmt.Println()
				argparser.WriteHelp(os.Stdout)
				os.Exit(1)
			}
		case "command":
		default:
			fmt.Println("drain enabled but no drain mode set")
			fmt.Println()
			argparser.WriteHelp(os.Stdout)
			os.Exit(1)
		}
	}
}

func startHttpServer() {
	// healthz
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprint(w, "Ok"); err != nil {
			log.Error(err)
		}
	})

	// readyz
	http.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if readyzStatus == 0 {
			if _, err := fmt.Fprint(w, "Ok"); err != nil {
				log.Error(err)
			}
		} else {
			w.WriteHeader(503)
			if _, err := fmt.Fprint(w, "Drain in progress"); err != nil {
				log.Error(err)
			}
		}
	})

	// drainz
	http.HandleFunc("/drainz", func(w http.ResponseWriter, r *http.Request) {
		if drainzStatus == 0 {
			if _, err := fmt.Fprint(w, "Ok"); err != nil {
				log.Error(err)
			}
		} else {
			w.WriteHeader(503)
			if _, err := fmt.Fprint(w, "Instance is drained"); err != nil {
				log.Error(err)
			}
		}
	})

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(opts.General.ServerBind, nil))
}
