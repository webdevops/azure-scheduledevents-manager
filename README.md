Azure ScheduledEvents Manager
==============================

[![license](https://img.shields.io/github/license/webdevops/azure-scheduledevents-manager.svg)](https://github.com/webdevops/azure-scheduledevents-manager/blob/master/LICENSE)
[![Docker](https://img.shields.io/badge/docker-webdevops%2Fazure--scheduledevents--manager-blue.svg?longCache=true&style=flat&logo=docker)](https://hub.docker.com/r/webdevops/azure-scheduledevents-manager/)
[![Docker Build Status](https://img.shields.io/docker/cloud/automated/webdevops/azure-scheduledevents-manager)](https://hub.docker.com/r/webdevops/azure-scheduledevents-manager/)

Manages Kubernetes nodes in specific [Azure ScheduledEvents](https://docs.microsoft.com/en-us/azure/virtual-machines/linux/scheduled-events) (planned VM maintenance) and exports the status as metric.
Drains nodes automatically when `Redeploy`, `Reboot` or `Preemt` is detected and is able to approve (start event ASAP) the event automatically.

It fetches informations from `http://169.254.169.254/metadata/scheduledevents?api-version=2017-08-01`
and exports the parsed information as metric to Prometheus.

Supports [shoutrrr](https://containrrr.github.io/shoutrrr/) notifications.

Configuration
-------------

```
Usage:
  azure-scheduledevents-manager [OPTIONS]

Application Options:
      --bind=                         Server address (default: :8080)
                                      [$SERVER_BIND]
      --scrape-time=                  Scrape time in seconds (default: 1m)
                                      [$SCRAPE_TIME]
  -v, --verbose                       Verbose mode [$VERBOSE]
      --azure.metadatainstance-url=   Azure ScheduledEvents API URL (default:
                                      http://169.254.169.254/metadata/instance?-

                                      api-version=2017-08-01)
                                      [$AZURE_METADATAINSTANCE_URL]
      --azure.scheduledevents-url=    Azure ScheduledEvents API URL (default:
                                      http://169.254.169.254/metadata/scheduled-

                                      events?api-version=2017-11-01)
                                      [$AZURE_SCHEDULEDEVENTS_URL]
      --azure.timeout=                Azure API timeout (seconds) (default:
                                      30s) [$AZURE_TIMEOUT]
      --azure.error-threshold=        Azure API error threshold (after which
                                      app will panic) (default: 0)
                                      [$AZURE_ERROR_THRESHOLD]
      --azure.approve-scheduledevent  Approve ScheduledEvent and start (if
                                      possible) start them ASAP
                                      [$AZURE_APPROVE_SCHEDULEDEVENT]
      --vm.nodename=                  VM node name [$VM_NODENAME]
      --kube.nodename=                Kubernetes node name [$KUBE_NODENAME]
      --drain.enable                  Enable drain handling [$DRAIN_ENABLE]
      --drain.not-before=             Dont drain before this time (default: 5m)
                                      [$DRAIN_NOT_BEFORE]
      --drain.delete-local-data       Continue even if there are pods using
                                      emptyDir (local data that will be deleted
                                      when the node is drained)
                                      [$DRAIN_DELETE_LOCAL_DATA]
      --drain.force                   Continue even if there are pods not
                                      managed by a ReplicationController,
                                      ReplicaSet, Job, DaemonSet or StatefulSet
                                      [$DRAIN_FORCE]
      --drain.grace-period=           Period of time in seconds given to each
                                      pod to terminate gracefully. If negative,
                                      the default value specified in the pod
                                      will be used. [$DRAIN_GRACE_PERIOD]
      --drain.ignore-daemonsets       Ignore DaemonSet-managed pods.
                                      [$DRAIN_IGNORE_DAEMONSETS]
      --drain.pod-selector=           Label selector to filter pods on the node
                                      [$DRAIN_POD_SELECTOR]
      --drain.timeout=                The length of time to wait before giving
                                      up, zero means infinite (default: 0s)
                                      [$DRAIN_TIMEOUT]
      --drain.dry-run                 Do not drain, uncordon or label any node
                                      [$DRAIN_DRY_RUN]
      --notification=                 Shoutrrr url for notifications
                                      (https://containrrr.github.io/shoutrrr/)
                                      [$NOTIFICATION]
      --notification.messagetemplate= Notification template (default: %v)
                                      [$NOTIFICATION_MESSAGE_TEMPLATE]
      --metrics-requeststats          Enable request stats metrics
                                      [$METRICS_REQUESTSTATS]

Help Options:
  -h, --help                          Show this help message
```

Metrics
-------

| Metric                                      | Description                                                                           |
|---------------------------------------------|---------------------------------------------------------------------------------------|
| `azure_scheduledevent_document_incarnation` | Document incarnation number (version)                                                 |
| `azure_scheduledevent_event`                | Fetched events from API                                                               |
| `azure_scheduledevent_request`              | Request histogram (count and request duration; disabled by default)                   |
| `azure_scheduledevent_request_error`        | Counter for failed requests                                                           |


Kubernetes Usage
----------------

```
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: azure-scheduledevents
spec:
  updateStrategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 100%
  selector:
    matchLabels:
      app: azure-scheduledevents
  template:
    metadata:
      labels:
        app: azure-scheduledevents
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/path: /metrics
        prometheus.io/port: "8080"
    spec:
      terminationGracePeriodSeconds: 15
      nodeSelector:
        beta.kubernetes.io/os: linux
      tolerations:
      - effect: NoSchedule
        operator: Exists
      containers:
      - name: azure-scheduledevents
        image: webdevops/azure-scheduledevents-manager
        env:
          - name: NOTIFICATION
            value: "slack://...."
          - name: AZURE_APPROVE_SCHEDULEDEVENT
            value: "true"
          - name: DRAIN_ENABLE
            value: "true"
          - name: DRAIN_NOT_BEFORE
            value: "5m"
          - name: DRAIN_DELETE_LOCAL_DATA
            value: "true"
          - name: DRAIN_FORCE
            value: "true"
          - name: DRAIN_IGNORE_DAEMONSETS
            value: "true"
          - name: DRAIN_DELETE_LOCAL_DATA
            value: "true"
          - name: KUBE_NODENAME
            valueFrom:
              fieldRef:
                fieldPath: spec.nodeName
        securityContext:
          readOnlyRootFilesystem: true
          runAsNonRoot: true
          capabilities:
            drop: ['ALL']
        ports:
        - containerPort: 8080
          name: metrics
          protocol: TCP
        resources:
          limits:
            cpu: 100m
            memory: 50Mi
          requests:
            cpu: 1m
            memory: 50Mi
```
