# Azure ScheduledEvents Manager

[![license](https://img.shields.io/github/license/webdevops/azure-scheduledevents-manager.svg)](https://github.com/webdevops/azure-scheduledevents-manager/blob/master/LICENSE)
[![DockerHub](https://img.shields.io/badge/DockerHub-webdevops%2Fazure--scheduledevents--manager-blue)](https://hub.docker.com/r/webdevops/azure-scheduledevents-manager/)
[![Quay.io](https://img.shields.io/badge/Quay.io-webdevops%2Fazure--scheduledevents--manager-blue)](https://quay.io/repository/webdevops/azure-scheduledevents-manager)

Manager for Linux VMs and Kubernetes clusters for [Azure ScheduledEvents](https://docs.microsoft.com/en-us/azure/virtual-machines/linux/scheduled-events) (planned VM maintenance) with Prometheus metrics support.
Drains nodes automatically when `Redeploy`, `Reboot`, `Preemt` or `Terminate` is detected and approves (start event ASAP) the event automatically.

#### Kubernetes support
Automatically drains and uncordon nodes before ScheduledEvents (Reboot, Redeploy, Terminate) to ensure service reliability.

AKS and custom Kubernetes clusters on Azure are supported.

#### VM support
Automatically executes commands for drain and uncordon before ScheduledEvents (Reboot, Redeploy, Terminate) to ensure service reliability.

#### Notification support

Supports [shoutrrr](https://containrrr.github.io/shoutrrr/) for notifications.

## Configuration

```
Usage:
  azure-scheduledevents-manager [OPTIONS]

Application Options:
      --debug                           debug mode [$DEBUG]
  -v, --verbose                         verbose mode [$VERBOSE]
      --log.json                        Switch log output to json format [$LOG_JSON]
      --bind=                           Server address (default: :8080) [$SERVER_BIND]
      --scrape-time=                    Scrape time in seconds (default: 1m) [$SCRAPE_TIME]
      --azure.metadatainstance-url=     Azure ScheduledEvents API URL (default:
                                        http://169.254.169.254/metadata/instance?api-version=2019-08-01)
                                        [$AZURE_METADATAINSTANCE_URL]
      --azure.scheduledevents-url=      Azure ScheduledEvents API URL (default:
                                        http://169.254.169.254/metadata/scheduledevents?api-version=2019-0-8-01)
                                        [$AZURE_SCHEDULEDEVENTS_URL]
      --azure.timeout=                  Azure API timeout (seconds) (default: 30s) [$AZURE_TIMEOUT]
      --azure.error-threshold=          Azure API error threshold (after which app will panic) (default:
                                        0) [$AZURE_ERROR_THRESHOLD]
      --azure.approve-scheduledevent    Approve ScheduledEvent and start (if possible) start them ASAP
                                        [$AZURE_APPROVE_SCHEDULEDEVENT]
      --vm.nodename=                    VM node name [$VM_NODENAME]
      --drain.enable                    Enable drain handling [$DRAIN_ENABLE]
      --drain.mode=[kubernetes|command] Mode [$DRAIN_MODE]
      --drain.not-before=               Dont drain before this time (default: 5m) [$DRAIN_NOT_BEFORE]
      --drain.events=                   Enable drain handling (default: reboot, redeploy, preempt,
                                        terminate) [$DRAIN_EVENTS]
      --command.test.cmd=               Test command in command mode [$COMMAND_TEST_CMD]
      --command.drain.cmd=              Drain command in command mode [$COMMAND_DRAIN_CMD]
      --command.uncordon.cmd=           Uncordon command in command mode [$COMMAND_UNCORDON_CMD]
      --kube.nodename=                  Kubernetes node name [$KUBE_NODENAME]
      --kube.drain.args=                Arguments for kubectl drain [$KUBE_DRAIN_ARGS]
      --kube.drain.dry-run              Do not drain, uncordon or label any node [$KUBE_DRAIN_DRY_RUN]
      --notification=                   Shoutrrr url for notifications
                                        (https://containrrr.github.io/shoutrrr/) [$NOTIFICATION]
      --notification.messagetemplate=   Notification template (default: %v)
                                        [$NOTIFICATION_MESSAGE_TEMPLATE]
      --metrics-requeststats            Enable request stats metrics [$METRICS_REQUESTSTATS]

Help Options:
  -h, --help                            Show this help message
```

## Metrics

| Metric                                      | Description                                                                           |
|---------------------------------------------|---------------------------------------------------------------------------------------|
| `azure_scheduledevent_document_incarnation` | Document incarnation number (version)                                                 |
| `azure_scheduledevent_event`                | Fetched events from API                                                               |
| `azure_scheduledevent_event_drain`          | Timestamp of drain (start and finish time)                                            |
| `azure_scheduledevent_event_approval`       | Timestamp of last event acknowledge                                                   |
| `azure_scheduledevent_request`              | Request histogram (count and request duration; disabled by default)                   |
| `azure_scheduledevent_request_error`        | Counter for failed requests                                                           |

## VM support

This example executes `/host-drain.sh` on the host when ScheduledEvent is received.
The docker container needs to access the host so it needs privileged permissions (privileged, pid=host, must run as root).
Container can be run as readonly container.

Run via docker:
```
docker run --restart=always --read-only --user=0 --privileged --pid=host \
    webdevops/azure-scheduledevents-manager:development \
    --drain.enable \
    --drain.mode=command \
    --drain.not-before=15m \
    --azure.approve-scheduledevent \
    --command.test.cmd="nsenter -m/proc/1/ns/mnt -- /usr/bin/test -x /host-drain.sh" \
    --command.drain.cmd="nsenter -m/proc/1/ns/mnt -- /host-drain.sh \$EVENT_TYPE"
```

This example will also pass

docker-compose:
```
version: "3"
services:
  scheduledEvents:
    image: webdevops/azure-scheduledevents-manager:development
    command:
    - --drain.enable
    - --drain.mode=command
    - --drain.not-before=15m
    - --azure.approve-scheduledevent
    - --command.test.cmd="nsenter -m/proc/1/ns/mnt -- /usr/bin/test -x /host-drain.sh"
    - --command.drain.cmd="nsenter -m/proc/1/ns/mnt -- /host-drain.sh $$EVENT_TYPE"
    user: 0:0
    privileged: true
    pid: "host"
    read_only: true
    restart: always
```

### Environment variables

all Docker environment variables are passed to drain command, also following event variables:

- EVENT_ID
- EVENT_SOURCE
- EVENT_STATUS
- EVENT_TYPE
- EVENT_NOTBEFORE
- EVENT_RESOURCES
- EVENT_RESOURCETYPE

## Kubernetes deployment

see [deployment](/deployment)
