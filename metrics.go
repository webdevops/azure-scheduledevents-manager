package main

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"strings"
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
			go probeCollect()
			time.Sleep(opts.ScrapeTime)
		}
	}()
}

func startHttpServer() {
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(opts.ServerBind, nil))
}

func probeCollect() {
	triggerDrain := false

	drainTimeThreshold := float64(time.Now().Add(opts.DrainNotBefore).Unix())

	startTime := time.Now()
	scheduledEvents, err := azureMetadata.FetchScheduledEvents()
	if err != nil {
		apiErrorCount++
		scheduledEventRequestError.With(prometheus.Labels{}).Inc()

		if opts.ApiErrorThreshold <= 0 || apiErrorCount <= opts.ApiErrorThreshold {
			ErrorLogger.Error("Failed API call:", err)
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
			ErrorLogger.Error(fmt.Sprintf("Unable to parse time \"%s\" of eventid \"%v\": %v", event.NotBefore, event.EventId), err)
			eventValue = 0
		}

		if len(event.Resources) >= 1 {
			for _, resource := range event.Resources {
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
					if eventValue == 1 || drainTimeThreshold >= eventValue {
						switch strings.ToLower(event.EventType) {
						case "reboot":
							fallthrough
						case "redeploy":
							fallthrough
						case "preempt":
							Logger.Println(fmt.Sprintf("detected %v in %v seconds", event.EventType, time.Unix(int64(eventValue), 0).Sub(time.Now()).String()))
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
		}
	}

	scheduledEventDocumentIncarnation.With(prometheus.Labels{}).Set(float64(scheduledEvents.DocumentIncarnation))

	Logger.Verbose("Fetched %v Azure ScheduledEvents", len(scheduledEvents.Events))

	if opts.KubeNodeName != "" {
		if triggerDrain {
			if !nodeDrained {
				Logger.Println(fmt.Sprintf("ensuring drain of node %v", opts.KubeNodeName))
				kubectl.NodeDrain()
				nodeDrained = true
				nodeUncordon = false
			}
		} else {
			if !nodeUncordon {
				Logger.Println(fmt.Sprintf("ensuring uncordon of node %v", opts.KubeNodeName))
				kubectl.NodeUncordon()
				nodeDrained = false
				nodeUncordon = true
			}
		}
	}
}
