Azure ScheduledEvents Manager
==============================

[![license](https://img.shields.io/github/license/webdevops/azure-scheduledevents-manager.svg)](https://github.com/webdevops/azure-scheduledevents-manager/blob/master/LICENSE)
[![Docker](https://img.shields.io/badge/docker-webdevops%2Fazure--scheduledevents--manager-blue.svg?longCache=true&style=flat&logo=docker)](https://hub.docker.com/r/webdevops/azure-scheduledevents-manager/)
[![Docker Build Status](https://img.shields.io/docker/cloud/automated/webdevops/azure-scheduledevents-manager)](https://hub.docker.com/r/webdevops/azure-scheduledevents-manager/)

Manages Kubernetes nodes in specific [Azure ScheduledEvents](https://docs.microsoft.com/en-us/azure/virtual-machines/linux/scheduled-events) (planned VM maintenance) and exports the status as metric.

It fetches informations from `http://169.254.169.254/metadata/scheduledevents?api-version=2017-08-01`
and exports the parsed information as metric to Prometheus.

Configuration
-------------

| Environment variable              | DefaultValue                                                              | Description                                                       |
|-----------------------------------|---------------------------------------------------------------------------|-------------------------------------------------------------------|
| `AZURE_METADATAINSTANCE_URL`      | `http://169.254.169.254/metadata/instance?api-version=2017-08-01`  | Azure API url                                                            |
| `AZURE_SCHEDULEDEVENTS_URL`       | `http://169.254.169.254/metadata/scheduledevents?api-version=2017-08-01`  | Azure API url                                                     |
| `AZURE_TIMEOUT`                   | `30s` (time.Duration)                                                     | API call timeout                                                  |
| `AZURE_ERROR_THRESHOLD`           | `0` (disabled)                                                            | API error threshold after which app will panic (`0` = dislabed)   |
| `VM_NODENAME`                     | `empty for autodetection using instance metadata`                         | Azure resource name of VM (empty for autodetection)               |
| `KUBE_NODENAME`                   | `empty`                                                                   | Kubernetes node name (required)                                   |
| `DRAIN_ENABLE`                    | `disabled`                                                                | Enable drain handling                                             |
| `DRAIN_NOT_BEFORE`                | `5m`                                                                      | Dont drain before this time                                       |
| `DRAIN_DELETE_LOCAL_DATA`         | `5m`                                                                      | Continue even if there are pods using emptyDir (local data that will be deleted when the node is drained)                                   |
| `DRAIN_FORCE`                     | `disabled`                                                                | Continue even if there are pods not managed by a ReplicationController, ReplicaSet, Job, DaemonSet or StatefulSet                                   |
| `DRAIN_GRACE_PERIOD`              | `0`                                                                       | Period of time in seconds given to each pod to terminate gracefully. If negative, the default value specified in the pod will be used                                   |
| `DRAIN_IGNORE_DAEMONSETS`         | `disabled`                                                                | Ignore DaemonSet-managed pods                                     |
| `DRAIN_POD_SELECTOR`              | `empty`                                                                   | Label selector to filter pods on the node                         |
| `DRAIN_TIMEOUT`                   | `0s`                                                                      | The length of time to wait before giving up, zero means infinite  |
| `DRAIN_DRY_RUN`                   | `disabled`                                                                | Dry run, do not drain, uncordon or label any node                 |
| `SCRAPE_TIME`                     | `1m` (time.Duration)                                                      | Time between API calls                                            |
| `SERVER_BIND`                     | `:8080`                                                                   | IP/Port binding                                                   |
| `METRICS_REQUESTSTATS`            | `empty`                                                                   | Enable metric `azure_scheduledevent_request`                      |
| `VERBOSE`                         | `disabled`                                                                | Verbose mode                                                      |

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
