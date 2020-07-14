package main

import (
	"fmt"
	"github.com/jessevdk/go-flags"
	log "github.com/sirupsen/logrus"
	"github.com/webdevops/azure-scheduledevents-manager/azuremetadata"
	"github.com/webdevops/azure-scheduledevents-manager/config"
	"net/url"
	"os"
	"runtime"
	"strings"
)

const (
	Author = "webdevops.io"
)

var (
	argparser *flags.Parser

	azureMetadata *azuremetadata.AzureMetadata
	kubectl       *KubernetesClient

	// Git version information
	gitCommit = "<unknown>"
	gitTag    = "<unknown>"
)

var opts config.Opts

func main() {
	initArgparser()

	log.Infof("starting Azure ScheduledEvents manager v%s (%s; %s; by %v)", gitTag, gitCommit, runtime.Version(), Author)
	log.Infof("starting azure metadata client")

	azureMetadata = &azuremetadata.AzureMetadata{
		ScheduledEventsUrl:  opts.AzureScheduledEventsApiUrl,
		InstanceMetadataUrl: opts.AzureInstanceApiUrl,
		Timeout:             &opts.AzureTimeout,
	}
	azureMetadata.Init()

	if opts.VmNodeName == "" {
		instanceMetadata, err := azureMetadata.FetchInstanceMetadata()
		if err != nil {
			panic(err)
		}
		log.Infof("detecting VM resource name")
		opts.VmNodeName = instanceMetadata.Compute.Name
	} else {
		log.Infof("using VM resource name from env")
	}
	log.Infof("  node: %v", opts.VmNodeName)

	log.Infof("init kubernetes")
	log.Infof("  Nodename: %v", opts.KubeNodeName)
	kubectl = &KubernetesClient{}
	kubectl.SetNode(opts.KubeNodeName)
	if opts.DrainEnable {
		log.Infof("  enabled automatic drain/uncordon")
		if opts.DrainDryRun {
			log.Infof("  DRYRUN enabled")
		}
		log.Infof("  drain not before: %v", opts.DrainNotBefore)
		kubectl.Enable()
	} else {
		log.Infof("  disabled automatic drain/uncordon")
	}
	log.Infof("  checking API server access")
	kubectl.CheckConnection()

	log.Infof("starting metrics collection")
	log.Infof("  MetadataInstance URL: %v", opts.AzureInstanceApiUrl)
	log.Infof("  ScheduledEvents URL: %v", opts.AzureScheduledEventsApiUrl)
	log.Infof("  API timeout: %v", opts.AzureTimeout)
	log.Infof("  scrape time: %v", opts.ScrapeTime)
	if opts.AzureErrorThreshold > 0 {
		log.Infof("  error threshold: %v", opts.AzureErrorThreshold)
	} else {
		log.Infof("  error threshold: disabled")
	}
	setupMetricsCollection()
	startMetricsCollection()

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
	}

	// json log format
	if opts.Logger.LogJson {
		log.SetFormatter(&log.JSONFormatter{})
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
		fmt.Println(fmt.Sprintf("ApiURL scheme not allowed (must be http or https), got %v", opts.AzureInstanceApiUrl))
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
		fmt.Println(fmt.Sprintf("ApiURL scheme not allowed (must be http or https), got %v", opts.AzureScheduledEventsApiUrl))
		fmt.Println()
		argparser.WriteHelp(os.Stdout)
		os.Exit(1)
	}
}
