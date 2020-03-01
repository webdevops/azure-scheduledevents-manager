package main

import (
	"fmt"
	"github.com/jessevdk/go-flags"
	"github.com/webdevops/azure-scheduledevents-manager/azuremetadata"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	Author  = "webdevops.io"
	Version = "0.1.0"
)

var (
	argparser   *flags.Parser
	args        []string
	Logger      *DaemonLogger
	ErrorLogger *DaemonLogger

	azureMetadata *azuremetadata.AzureMetadata
	kubectl       *KubernetesClient
)

var opts struct {
	// general options
	ServerBind string        `long:"bind"                env:"SERVER_BIND"   description:"Server address"                default:":8080"`
	ScrapeTime time.Duration `long:"scrape-time"         env:"SCRAPE_TIME"   description:"Scrape time in seconds"        default:"1m"`
	Verbose    []bool        `long:"verbose" short:"v"   env:"VERBOSE"       description:"Verbose mode"`

	// Api options
	AzureInstanceApiUrl        string        `long:"azure.metadatainstance-url" env:"AZURE_METADATAINSTANCE_URL"  description:"Azure ScheduledEvents API URL" default:"http://169.254.169.254/metadata/instance?api-version=2017-08-01"`
	AzureScheduledEventsApiUrl string        `long:"azure.scheduledevents-url"  env:"AZURE_SCHEDULEDEVENTS_URL"   description:"Azure ScheduledEvents API URL" default:"http://169.254.169.254/metadata/scheduledevents?api-version=2017-11-01"`
	AzureTimeout               time.Duration `long:"azure.timeout"              env:"AZURE_TIMEOUT"               description:"Azure API timeout (seconds)"   default:"30s"`
	AzureErrorThreshold        int           `long:"azure.error-threshold"      env:"AZURE_ERROR_THRESHOLD"       description:"Azure API error threshold (after which app will panic)"   default:"0"`

	VmNodeName   string `long:"vm.nodename"    env:"VM_NODENAME"     description:"VM node name"`
	KubeNodeName string `long:"kube.nodename"  env:"KUBE_NODENAME"   description:"Kubernetes node name" required:"true"`

	DrainEnable           bool          `long:"drain.enable"             env:"DRAIN_ENABLE"             description:"Enable drain handling"`
	DrainNotBefore        time.Duration `long:"drain.not-before"         env:"DRAIN_NOT_BEFORE"         description:"Dont drain before this time" default:"5m"`
	DrainDeleteLocalData  bool          `long:"drain.delete-local-data"  env:"DRAIN_DELETE_LOCAL_DATA"  description:"Continue even if there are pods using emptyDir (local data that will be deleted when the node is drained)"`
	DrainForce            bool          `long:"drain.force"              env:"DRAIN_FORCE"              description:"Continue even if there are pods not managed by a ReplicationController, ReplicaSet, Job, DaemonSet or StatefulSet"`
	DrainGracePeriod      int64         `long:"drain.grace-period"       env:"DRAIN_GRACE_PERIOD"       description:"Period of time in seconds given to each pod to terminate gracefully. If negative, the default value specified in the pod will be used."`
	DrainIgnoreDaemonsets bool          `long:"drain.ignore-daemonsets"  env:"DRAIN_IGNORE_DAEMONSETS"  description:"Ignore DaemonSet-managed pods."`
	DrainPodSelector      string        `long:"drain.pod-selector"       env:"DRAIN_POD_SELECTOR"       description:"Label selector to filter pods on the node"`
	DrainTimeout          time.Duration `long:"drain.timeout"            env:"DRAIN_TIMEOUT"            description:"The length of time to wait before giving up, zero means infinite" default:"0s"`
	DrainDryRun           bool          `long:"drain.dry-run"            env:"DRAIN_DRY_RUN"            description:"Do not drain, uncordon or label any node"`

	// metrics
	MetricsRequestStats bool `long:"metrics-requeststats" env:"METRICS_REQUESTSTATS" description:"Enable request stats metrics"`
}

func main() {
	initArgparser()

	// Init logger
	Logger = CreateDaemonLogger(0)
	ErrorLogger = CreateDaemonErrorLogger(0)

	// set verbosity
	Verbose = len(opts.Verbose) >= 1

	Logger.Messsage("Init Azure ScheduledEvents manager v%s (written by %v)", Version, Author)
	Logger.Messsage("init azure metadata client")

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
		Logger.Messsage("detecting nodename")
		opts.VmNodeName = instanceMetadata.Compute.Name
	} else {
		Logger.Messsage("using nodename from env")
	}
	Logger.Messsage("  node: %v", opts.VmNodeName)

	Logger.Messsage("Init kubernetes")
	Logger.Messsage("  Nodename: %v", opts.KubeNodeName)
	kubectl = &KubernetesClient{}
	kubectl.SetNode(opts.KubeNodeName)
	if opts.DrainEnable {
		Logger.Messsage("  enabled automatic drain/uncordon")
		kubectl.Enable()
	} else {
		Logger.Messsage("  disabled automatic drain/uncordon")
	}
	kubectl.CheckConnection()

	Logger.Messsage("Starting metrics collection")
	Logger.Messsage("  MetadataInstance URL: %v", opts.AzureInstanceApiUrl)
	Logger.Messsage("  ScheduledEvents URL: %v", opts.AzureScheduledEventsApiUrl)
	Logger.Messsage("  API timeout: %v", opts.AzureTimeout)
	Logger.Messsage("  scape time: %v", opts.ScrapeTime)
	if opts.AzureErrorThreshold > 0 {
		Logger.Messsage("  error threshold: %v", opts.AzureErrorThreshold)
	} else {
		Logger.Messsage("  error threshold: disabled")
	}
	setupMetricsCollection()
	startMetricsCollection()

	Logger.Messsage("Starting http server on %s", opts.ServerBind)
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
