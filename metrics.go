package main

import (
	"encoding/json"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"strings"
	"time"
)

type AzureScheduledEventResponse struct {
	DocumentIncarnation int                   `json:"DocumentIncarnation"`
	Events              []AzureScheduledEvent `json:"Events"`
}

type AzureScheduledEvent struct {
	EventId      string   `json:"EventId"`
	EventType    string   `json:"EventType"`
	ResourceType string   `json:"ResourceType"`
	Resources    []string `json:"Resources"`
	EventStatus  string   `json:"EventStatus"`
	NotBefore    string   `json:"NotBefore"`
}

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

	timeFormatList = []string{
		time.RFC3339,
		time.RFC1123,
		time.RFC822Z,
		time.RFC850,
	}


	apiErrorCount = 0
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

	scheduledEvents, err := fetchApiUrl()
	if err != nil {
		apiErrorCount++

		if opts.ApiErrorThreshold <= 0 || apiErrorCount <= opts.ApiErrorThreshold {
			ErrorLogger.Error("Failed API call:", err)
			return
		} else {
			panic(err.Error())
		}
	}

	// reset error count and metrics
	apiErrorCount = 0
	scheduledEvent.Reset()

	for _, event := range scheduledEvents.Events {
		eventValue := float64(1)

		if event.NotBefore != "" {
			notBefore, err := parseTime(event.NotBefore)
			if err == nil {
				eventValue = float64(notBefore.Unix())
			} else {
				ErrorLogger.Error(fmt.Sprintf("Unable to parse time \"%s\" of eventid \"%v\"", event.NotBefore, event.EventId), err)
				eventValue = 0
			}
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

func fetchApiUrl() (*AzureScheduledEventResponse, error) {
	ret := &AzureScheduledEventResponse{}

	startTime := time.Now()
	req, err := http.NewRequest("GET", opts.ScheduledEventsApiUrl, nil)
	if err != nil {
		scheduledEventRequestError.With(prometheus.Labels{}).Inc()
		return nil, err
	}
	req.Header.Add("Metadata", "true")

	resp, err := httpClient.Do(req)
	if err != nil {
		scheduledEventRequestError.With(prometheus.Labels{}).Inc()
		return nil, err
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&ret)
	if err != nil {
		scheduledEventRequestError.With(prometheus.Labels{}).Inc()
		return nil, err
	}

	if opts.MetricsRequestStats {
		duration := time.Now().Sub(startTime)
		scheduledEventRequest.With(prometheus.Labels{}).Observe(duration.Seconds())
	}

	return ret, nil
}

func parseTime(value string) (parsedTime time.Time, err error) {
	for _, format := range timeFormatList {
		parsedTime, err = time.Parse(format, value)
		if err == nil {
			break
		}
	}

	return
}
