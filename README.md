Azure ScheduledEvents Manager
==============================

[![license](https://img.shields.io/github/license/webdevops/azure-scheduledevents-manager.svg)](https://github.com/webdevops/azure-scheduledevents-manager/blob/master/LICENSE)
[![Docker](https://img.shields.io/badge/docker-webdevops%2Fazure--scheduledevents--manager-blue.svg?longCache=true&style=flat&logo=docker)](https://hub.docker.com/r/webdevops/azure-scheduledevents-manager/)
[![Docker Build Status](https://img.shields.io/docker/build/webdevops/azure-scheduledevents-manager.svg)](https://hub.docker.com/r/webdevops/azure-scheduledevents-manager/)

Manages Kubernetes nodes in specific [Azure ScheduledEvents](https://docs.microsoft.com/en-us/azure/virtual-machines/linux/scheduled-events) (planned VM maintenance) and exports the status as metric.

It fetches informations from `http://169.254.169.254/metadata/scheduledevents?api-version=2017-08-01`
and exports the parsed information as metric to Prometheus.

Configuration
-------------

Normally no configuration is needed but can be customized using environment variables.

| Environment variable   | DefaultValue                                                              | Description                                                       |
|------------------------|---------------------------------------------------------------------------|-------------------------------------------------------------------|
| `API_URL`              | `http://169.254.169.254/metadata/scheduledevents?api-version=2017-08-01`  | Azure API url                                                     |
| `API_TIMEOUT`          | `30s` (time.Duration)                                                     | API call timeout                                                  |
| `API_ERROR_THRESHOLD`  | `0` (disabled)                                                            | API error threshold after which app will panic (`0` = dislabed)   |
| `SCRAPE_TIME`          | `1m` (time.Duration)                                                      | Time between API calls                                            |
| `SERVER_BIND`          | `:8080`                                                                   | IP/Port binding                                                   |
| `METRICS_REQUESTSTATS` | `empty`                                                                   | Enable metric `azure_scheduledevent_request`                      |


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
