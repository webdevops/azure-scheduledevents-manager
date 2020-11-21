package config

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"time"
)

type (
	Opts struct {
		// logger
		Logger struct {
			Debug   bool `           long:"debug"        env:"DEBUG"    description:"debug mode"`
			Verbose bool `short:"v"  long:"verbose"      env:"VERBOSE"  description:"verbose mode"`
			LogJson bool `           long:"log.json"     env:"LOG_JSON" description:"Switch log output to json format"`
		}

		// general options
		ServerBind string        `long:"bind"                env:"SERVER_BIND"   description:"Server address"                default:":8080"`
		ScrapeTime time.Duration `long:"scrape-time"         env:"SCRAPE_TIME"   description:"Scrape time in seconds"        default:"1m"`

		// Api options
		AzureInstanceApiUrl        string        `long:"azure.metadatainstance-url"    env:"AZURE_METADATAINSTANCE_URL"    description:"Azure ScheduledEvents API URL" default:"http://169.254.169.254/metadata/instance?api-version=2019-08-01"`
		AzureScheduledEventsApiUrl string        `long:"azure.scheduledevents-url"     env:"AZURE_SCHEDULEDEVENTS_URL"     description:"Azure ScheduledEvents API URL" default:"http://169.254.169.254/metadata/scheduledevents?api-version=2019-08-01"`
		AzureTimeout               time.Duration `long:"azure.timeout"                 env:"AZURE_TIMEOUT"                 description:"Azure API timeout (seconds)"   default:"30s"`
		AzureErrorThreshold        int           `long:"azure.error-threshold"         env:"AZURE_ERROR_THRESHOLD"         description:"Azure API error threshold (after which app will panic)"   default:"0"`
		AzureApproveScheduledEvent bool          `long:"azure.approve-scheduledevent"  env:"AZURE_APPROVE_SCHEDULEDEVENT"  description:"Approve ScheduledEvent and start (if possible) start them ASAP"`

		VmNodeName   string `long:"vm.nodename"    env:"VM_NODENAME"     description:"VM node name"`
		KubeNodeName string `long:"kube.nodename"  env:"KUBE_NODENAME"   description:"Kubernetes node name" required:"true"`

		// drain opts
		DrainEnable           bool          `long:"drain.enable"             env:"DRAIN_ENABLE"                description:"Enable drain handling"`
		DrainEvents           []string      `long:"drain.events"             env:"DRAIN_EVENTS" env-delim:" "  description:"Enable drain handling" default:"reboot" default:"redeploy" default:"preempt" default:"terminate"` //nolint:staticcheck
		DrainNotBefore        time.Duration `long:"drain.not-before"         env:"DRAIN_NOT_BEFORE"            description:"Dont drain before this time" default:"5m"`
		DrainDeleteLocalData  bool          `long:"drain.delete-local-data"  env:"DRAIN_DELETE_LOCAL_DATA"     description:"Continue even if there are pods using emptyDir (local data that will be deleted when the node is drained)"`
		DrainForce            bool          `long:"drain.force"              env:"DRAIN_FORCE"                 description:"Continue even if there are pods not managed by a ReplicationController, ReplicaSet, Job, DaemonSet or StatefulSet"`
		DrainGracePeriod      int64         `long:"drain.grace-period"       env:"DRAIN_GRACE_PERIOD"          description:"Period of time in seconds given to each pod to terminate gracefully. If negative, the default value specified in the pod will be used."`
		DrainIgnoreDaemonsets bool          `long:"drain.ignore-daemonsets"  env:"DRAIN_IGNORE_DAEMONSETS"     description:"Ignore DaemonSet-managed pods."`
		DrainPodSelector      string        `long:"drain.pod-selector"       env:"DRAIN_POD_SELECTOR"          description:"Label selector to filter pods on the node"`
		DrainTimeout          time.Duration `long:"drain.timeout"            env:"DRAIN_TIMEOUT"               description:"The length of time to wait before giving up, zero means infinite" default:"0s"`
		DrainDryRun           bool          `long:"drain.dry-run"            env:"DRAIN_DRY_RUN"               description:"Do not drain, uncordon or label any node"`

		Notification            []string `long:"notification"                 env:"NOTIFICATION"              description:"Shoutrrr url for notifications (https://containrrr.github.io/shoutrrr/)" env-delim:" "`
		NotificationMsgTemplate string   `long:"notification.messagetemplate" env:"NOTIFICATION_MESSAGE_TEMPLATE"  description:"Notification template" default:"%v"`

		// metrics
		MetricsRequestStats bool `long:"metrics-requeststats" env:"METRICS_REQUESTSTATS" description:"Enable request stats metrics"`
	}
)

func (o *Opts) GetJson() []byte {
	jsonBytes, err := json.Marshal(o)
	if err != nil {
		log.Panic(err)
	}
	return jsonBytes
}
