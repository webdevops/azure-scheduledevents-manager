package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync/atomic"

	flags "github.com/jessevdk/go-flags"
	"github.com/prometheus/client_golang/prometheus/promhttp"

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
	Opts      config.Opts

	// Git version information
	gitCommit = "<unknown>"
	gitTag    = "<unknown>"
	buildDate = "<unknown>"

	readyzStatus = int64(0)
	drainzStatus = int64(0)
)

func main() {
	initArgparser()
	initLogger()

	logger.Infof("starting azure-scheduledevents-manager v%s (%s; %s; by %v at %v)", gitTag, gitCommit, runtime.Version(), Author, buildDate)
	logger.Info(string(Opts.GetJson()))
	initSystem()

	logger.Infof("starting azure metadata client")
	azureMetadataClient := &azuremetadata.AzureMetadata{
		ScheduledEventsUrl:  Opts.Azure.ScheduledEventsApiUrl,
		InstanceMetadataUrl: Opts.Azure.InstanceApiUrl,
		Timeout:             &Opts.Azure.Timeout,
		UserAgent:           fmt.Sprintf("azure-scheduledevents-manager/%v", gitTag),
	}
	azureMetadataClient.Init()

	if Opts.Instance.VmNodeName == "" {
		instanceMetadata, err := azureMetadataClient.FetchInstanceMetadata()
		if err != nil {
			logger.Fatal(err.Error())
		}
		logger.Infof("detecting VM resource name")
		Opts.Instance.VmNodeName = instanceMetadata.Compute.Name
	} else {
		logger.Infof("using VM resource name from env")
	}
	logger.Infof("using VM node: %v", Opts.Instance.VmNodeName)

	scheduledEventsManager := manager.ScheduledEventsManager{
		Conf:                Opts,
		Logger:              logger,
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

	if Opts.Drain.Enable {
		switch Opts.Drain.Mode {
		case "kubernetes":
			logger.Infof("start \"kubernetes\" mode")
			logger.Infof("using Kubernetes nodename: %v", Opts.Kubernetes.NodeName)
			drain := &drainmanager.DrainManagerKubernetes{
				Conf:   Opts,
				Logger: logger,
			}
			drain.SetInstanceName(Opts.Kubernetes.NodeName)
			scheduledEventsManager.DrainManager = drain
		case "command":
			logger.Infof("start \"command\" mode")
			drain := drainmanager.DrainManagerCommand{
				Conf:   Opts,
				Logger: logger,
			}
			drain.SetInstanceName(Opts.Instance.VmNodeName)
			scheduledEventsManager.DrainManager = &drain
		default:
			logger.Fatalf("drain mode \"%v\" is not valid", Opts.Drain.Mode)
		}

		if err := scheduledEventsManager.DrainManager.Test(); err != nil {
			logger.Fatalf(`failed to test drain manager: %v`, err)
		}
	}

	logger.Infof("starting manager")
	scheduledEventsManager.Start()

	logger.Infof("starting http server on %s", Opts.Server.Bind)
	startHttpServer()
}

func initArgparser() {
	argparser = flags.NewParser(&Opts, flags.Default)
	_, err := argparser.Parse()

	// check if there is an parse error
	if err != nil {
		var flagsErr *flags.Error
		if ok := errors.As(err, &flagsErr); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			fmt.Println()
			argparser.WriteHelp(os.Stdout)
			os.Exit(1)
		}
	}

	// validate instanceUrl url
	instanceUrl, err := url.Parse(Opts.Azure.InstanceApiUrl)
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
		fmt.Printf("ApiURL scheme not allowed (must be http or https), got %v\n", Opts.Azure.InstanceApiUrl)
		fmt.Println()
		argparser.WriteHelp(os.Stdout)
		os.Exit(1)
	}

	// validate scheduledEventsUrl url
	scheduledEventsUrl, err := url.Parse(Opts.Azure.ScheduledEventsApiUrl)
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
		fmt.Printf("ApiURL scheme not allowed (must be http or https), got %v\n", Opts.Azure.ScheduledEventsApiUrl)
		fmt.Println()
		argparser.WriteHelp(os.Stdout)
		os.Exit(1)
	}

	if Opts.Drain.Enable {
		switch Opts.Drain.Mode {
		case "kubernetes":
			if Opts.Kubernetes.NodeName == "" {
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
	mux := http.NewServeMux()

	// healthz
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprint(w, "Ok"); err != nil {
			logger.Error(err.Error())
		}
	})

	// readyz
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if readyzStatus == 0 {
			if _, err := fmt.Fprint(w, "Ok"); err != nil {
				logger.Error(err.Error())
			}
		} else {
			w.WriteHeader(503)
			if _, err := fmt.Fprint(w, "Drain in progress"); err != nil {
				logger.Error(err.Error())
			}
		}
	})

	// drainz
	mux.HandleFunc("/drainz", func(w http.ResponseWriter, r *http.Request) {
		if drainzStatus == 0 {
			if _, err := fmt.Fprint(w, "Ok"); err != nil {
				logger.Error(err.Error())
			}
		} else {
			w.WriteHeader(503)
			if _, err := fmt.Fprint(w, "Instance is drained"); err != nil {
				logger.Error(err.Error())
			}
		}
	})

	mux.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{
		Addr:         Opts.Server.Bind,
		Handler:      mux,
		ReadTimeout:  Opts.Server.ReadTimeout,
		WriteTimeout: Opts.Server.WriteTimeout,
	}
	if err := srv.ListenAndServe(); err != nil {
		logger.Fatal(err.Error())
	}
}
