package main

import (
	"fmt"
	"github.com/jessevdk/go-flags"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/webdevops/azure-scheduledevents-manager/azuremetadata"
	"github.com/webdevops/azure-scheduledevents-manager/config"
	"github.com/webdevops/azure-scheduledevents-manager/kubectl"
	"github.com/webdevops/azure-scheduledevents-manager/manager"
	"net/http"
	"net/url"
	"os"
	"path"
	"runtime"
	"strings"
)

const (
	Author = "webdevops.io"
)

var (
	argparser *flags.Parser

	// Git version information
	gitCommit = "<unknown>"
	gitTag    = "<unknown>"
)

var opts config.Opts

func main() {
	initArgparser()

	log.Infof("starting Azure ScheduledEvents manager v%s (%s; %s; by %v)", gitTag, gitCommit, runtime.Version(), Author)
	log.Info(string(opts.GetJson()))

	log.Infof("starting azure metadata client")
	azureMetadataClient := &azuremetadata.AzureMetadata{
		ScheduledEventsUrl:  opts.AzureScheduledEventsApiUrl,
		InstanceMetadataUrl: opts.AzureInstanceApiUrl,
		Timeout:             &opts.AzureTimeout,
	}
	azureMetadataClient.Init()

	if opts.VmNodeName == "" {
		instanceMetadata, err := azureMetadataClient.FetchInstanceMetadata()
		if err != nil {
			panic(err)
		}
		log.Infof("detecting VM resource name")
		opts.VmNodeName = instanceMetadata.Compute.Name
	} else {
		log.Infof("using VM resource name from env")
	}
	log.Infof("using VM node: %v", opts.VmNodeName)

	log.Infof("init Kubernetes")
	log.Infof("using Kubernetes nodename: %v", opts.KubeNodeName)
	kubectlClient := &kubectl.KubernetesClient{
		Conf: opts,
	}
	kubectlClient.SetNode(opts.KubeNodeName)
	if opts.DrainEnable {
		kubectlClient.Enable()
	}
	log.Infof("checking Kubernetes API server access")
	kubectlClient.CheckConnection()

	log.Infof("starting metrics collection")
	manager := manager.ScheduledEventsManager{
		Conf:                opts,
		AzureMetadataClient: azureMetadataClient,
		KubectlClient:       kubectlClient,
	}
	manager.Init()
	manager.Start()

	log.Infof("starting http server on %s", opts.ServerBind)
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
	instanceUrl, err := url.Parse(opts.AzureInstanceApiUrl)
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
		fmt.Printf("ApiURL scheme not allowed (must be http or https), got %v\n", opts.AzureInstanceApiUrl)
		fmt.Println()
		argparser.WriteHelp(os.Stdout)
		os.Exit(1)
	}

	// validate scheduledEventsUrl url
	scheduledEventsUrl, err := url.Parse(opts.AzureScheduledEventsApiUrl)
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
		fmt.Printf("ApiURL scheme not allowed (must be http or https), got %v\n", opts.AzureScheduledEventsApiUrl)
		fmt.Println()
		argparser.WriteHelp(os.Stdout)
		os.Exit(1)
	}
}

func startHttpServer() {
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(opts.ServerBind, nil))
}
