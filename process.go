package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/webdevops/azure-scheduledevents-manager/azuremetadata"
	"net/http"
	"time"
)

var (
	scheduledEventDocumentIncarnation = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azure_scheduledevent_document_incarnation",
			Help: "Azure ScheduledEvent document incarnation",
		},
		[]string{},
	)

	scheduledEvent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azure_scheduledevent_event",
			Help: "Azure ScheduledEvent",
		},
		[]string{"eventID", "eventType", "resourceType", "resource", "eventStatus", "notBefore"},
	)

	scheduledEventRequest = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "azure_scheduledevent_request",
			Help: "Azure ScheduledEvent requests",
		},
		[]string{},
	)

	scheduledEventRequestError = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "azure_scheduledevent_request_error",
			Help: "Azure ScheduledEvent failed requests",
		},
		[]string{},
	)

	apiErrorCount = 0
	nodeDrained   bool
	nodeUncordon  bool
)

func setupMetricsCollection() {
	prometheus.MustRegister(scheduledEvent)
	prometheus.MustRegister(scheduledEventDocumentIncarnation)
	prometheus.MustRegister(scheduledEventRequest)
	prometheus.MustRegister(scheduledEventRequestError)

	apiErrorCount = 0
}

func startMetricsCollection() {
	go func() {
		for {
			probeCollect()
			time.Sleep(opts.ScrapeTime)
		}
	}()
}

func startHttpServer() {
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(opts.ServerBind, nil))
}

func probeCollect() {
	var approveEvent *azuremetadata.AzureScheduledEvent
	triggerDrain := false

	drainTimeThreshold := float64(time.Now().Add(opts.DrainNotBefore).Unix())

	startTime := time.Now()
	scheduledEvents, err := azureMetadata.FetchScheduledEvents()
	if err != nil {
		apiErrorCount++
		scheduledEventRequestError.With(prometheus.Labels{}).Inc()

		if opts.AzureErrorThreshold <= 0 || apiErrorCount <= opts.AzureErrorThreshold {
			log.Errorf("failed API call: %s", err)
			return
		} else {
			panic(err.Error())
		}
	}

	if opts.MetricsRequestStats {
		duration := time.Now().Sub(startTime)
		scheduledEventRequest.With(prometheus.Labels{}).Observe(duration.Seconds())
	}

	// reset error count and metrics
	apiErrorCount = 0
	scheduledEvent.Reset()

	for _, event := range scheduledEvents.Events {
		eventValue, err := event.NotBeforeUnixTimestamp()

		if err != nil {
			log.Errorf("unable to parse time \"%s\" of eventid \"%v\": %v", event.NotBefore, event.EventId, err)
			eventValue = 0
		}

		if len(event.Resources) >= 1 {
			for _, resource := range event.Resources {
				log.WithFields(log.Fields{
					"eventID":      event.EventId,
					"eventType":    event.EventType,
					"resourceType": event.ResourceType,
					"resource":     resource,
					"eventStatus":  event.EventStatus,
					"notBefore":    event.NotBefore,
				}).Debugf("found ScheduledEvent")

				scheduledEvent.With(
					prometheus.Labels{
						"eventID":      event.EventId,
						"eventType":    event.EventType,
						"resourceType": event.ResourceType,
						"resource":     resource,
						"eventStatus":  event.EventStatus,
						"notBefore":    event.NotBefore,
					}).Set(eventValue)

				if opts.VmNodeName != "" && resource == opts.VmNodeName {
					log.WithFields(log.Fields{
						"eventID":      event.EventId,
						"eventType":    event.EventType,
						"resourceType": event.ResourceType,
						"resource":     resource,
						"eventStatus":  event.EventStatus,
						"notBefore":    event.NotBefore,
					}).Infof("detected ScheduledEvent %v with %v in %v for current node", event.EventId, event.EventType, time.Unix(int64(eventValue), 0).Sub(time.Now()).String())
					approveEvent = &event
					if eventValue == 1 || drainTimeThreshold >= eventValue {
						if stringArrayContainsCi(opts.DrainEvents, event.EventType) {
							triggerDrain = true
						}
					}
				}
			}
		} else {
			scheduledEvent.With(
				prometheus.Labels{
					"eventID":      event.EventId,
					"eventType":    event.EventType,
					"resourceType": event.ResourceType,
					"resource":     "",
					"eventStatus":  event.EventStatus,
					"notBefore":    event.NotBefore,
				}).Set(eventValue)

			log.WithFields(log.Fields{
				"eventID":      event.EventId,
				"eventType":    event.EventType,
				"resourceType": event.ResourceType,
				"resource":     "",
				"eventStatus":  event.EventStatus,
				"notBefore":    event.NotBefore,
			}).Debugf("found ScheduledEvent")
		}
	}

	scheduledEventDocumentIncarnation.With(prometheus.Labels{}).Set(float64(scheduledEvents.DocumentIncarnation))

	if len(scheduledEvents.Events) > 0 {
		log.Infof("fetched %v Azure ScheduledEvents", len(scheduledEvents.Events))
	} else {
		log.Debugf("fetched %v Azure ScheduledEvents", len(scheduledEvents.Events))

		// if event is gone, ensure uncordon of node
		if !nodeUncordon {
			log.Infof("ensuring uncordon of node %v", opts.KubeNodeName)
			kubectl.NodeUncordon()
			nodeDrained = false
			nodeUncordon = true
		}
	}

	if opts.KubeNodeName != "" {
		if approveEvent != nil && triggerDrain {
			eventLogger := log.WithField("eventID" , approveEvent.EventId)

			if !nodeDrained {
				eventLogger.Infof("ensuring drain of node %v", opts.KubeNodeName)
				notificationMessage("draining K8s node %v (upcoming Azure ScheduledEvent %v with %s)", opts.KubeNodeName, approveEvent.EventId, approveEvent.EventType)
				kubectl.NodeDrain()
				eventLogger.Infof("drained successfully")
				nodeDrained = true
				nodeUncordon = false
			}

			if opts.AzureApproveScheduledEvent {
				eventLogger.Infof("approving ScheduledEvent %v with %v", approveEvent.EventId, approveEvent.EventType)
				if err := azureMetadata.ApproveScheduledEvent(approveEvent); err == nil {
					eventLogger.Infof("event approved")
				} else {
					eventLogger.Infof("approval failed: %v", err)
				}
			}
		} else {
			if !nodeUncordon {
				log.Infof("ensuring uncordon of node %v", opts.KubeNodeName)
				kubectl.NodeUncordon()
				nodeDrained = false
				nodeUncordon = true
			}
		}
	}
}
