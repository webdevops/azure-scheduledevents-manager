package manager

import (
	"fmt"
	"github.com/containrrr/shoutrrr"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/webdevops/azure-scheduledevents-manager/azuremetadata"
	"github.com/webdevops/azure-scheduledevents-manager/config"
	"github.com/webdevops/azure-scheduledevents-manager/kubectl"
	"time"
)

type (
	ScheduledEventsManager struct {
		apiErrorCount int
		nodeDrained   bool
		nodeUncordon  bool

		Conf                config.Opts
		AzureMetadataClient *azuremetadata.AzureMetadata
		KubectlClient       *kubectl.KubernetesClient

		prometheus struct {
			documentIncarnation *prometheus.GaugeVec
			event               *prometheus.GaugeVec
			eventDrain          *prometheus.GaugeVec
			eventApproval       *prometheus.GaugeVec
			request             *prometheus.HistogramVec
			requestErrors       *prometheus.CounterVec
		}
	}
)

func (m *ScheduledEventsManager) Init() {
	m.initMetrics()
}

func (m *ScheduledEventsManager) initMetrics() {
	m.prometheus.documentIncarnation = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azure_scheduledevent_document_incarnation",
			Help: "Azure ScheduledEvent document incarnation",
		},
		[]string{},
	)
	prometheus.MustRegister(m.prometheus.documentIncarnation)

	m.prometheus.event = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azure_scheduledevent_event",
			Help: "Azure ScheduledEvent",
		},
		[]string{"eventID", "eventType", "resourceType", "resource", "eventStatus", "notBefore"},
	)
	prometheus.MustRegister(m.prometheus.event)

	m.prometheus.eventApproval = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azure_scheduledevent_event_approval",
			Help: "Azure ScheduledEvent timestamp of approval",
		},
		[]string{"eventID"},
	)
	prometheus.MustRegister(m.prometheus.eventApproval)

	m.prometheus.eventDrain = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azure_scheduledevent_event_drain",
			Help: "Azure ScheduledEvent timestamp of drain",
		},
		[]string{"eventID", "type"},
	)
	prometheus.MustRegister(m.prometheus.eventDrain)

	m.prometheus.request = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "azure_scheduledevent_request",
			Help: "Azure ScheduledEvent requests",
		},
		[]string{},
	)
	prometheus.MustRegister(m.prometheus.request)

	m.prometheus.requestErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "azure_scheduledevent_request_error",
			Help: "Azure ScheduledEvent failed requests",
		},
		[]string{},
	)
	prometheus.MustRegister(m.prometheus.requestErrors)
}

func (m *ScheduledEventsManager) Start() {
	go func() {
		for {
			m.collect()
			time.Sleep(m.Conf.ScrapeTime)
		}
	}()
}

func (m *ScheduledEventsManager) collect() {
	var approveEvent *azuremetadata.AzureScheduledEvent
	triggerDrain := false

	drainTimeThreshold := float64(time.Now().Add(m.Conf.DrainNotBefore).Unix())

	startTime := time.Now()
	scheduledEvents, err := m.AzureMetadataClient.FetchScheduledEvents()
	if err != nil {
		m.apiErrorCount++
		m.prometheus.requestErrors.With(prometheus.Labels{}).Inc()

		if m.Conf.AzureErrorThreshold <= 0 || m.apiErrorCount <= m.Conf.AzureErrorThreshold {
			log.Errorf("failed API call: %s", err)
			return
		} else {
			panic(err.Error())
		}
	}

	if m.Conf.MetricsRequestStats {
		duration := time.Since(startTime)
		m.prometheus.request.With(prometheus.Labels{}).Observe(duration.Seconds())
	}

	// reset error count and metrics
	m.apiErrorCount = 0
	m.prometheus.event.Reset()

	if len(scheduledEvents.Events) == 0 {
		m.prometheus.eventDrain.Reset()
		m.prometheus.eventApproval.Reset()
	}

	for _, row := range scheduledEvents.Events {
		event := row
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

				m.prometheus.event.With(
					prometheus.Labels{
						"eventID":      event.EventId,
						"eventType":    event.EventType,
						"resourceType": event.ResourceType,
						"resource":     resource,
						"eventStatus":  event.EventStatus,
						"notBefore":    event.NotBefore,
					}).Set(eventValue)

				if m.Conf.VmNodeName != "" && resource == m.Conf.VmNodeName {
					log.WithFields(log.Fields{
						"eventID":      event.EventId,
						"eventType":    event.EventType,
						"resourceType": event.ResourceType,
						"resource":     resource,
						"eventStatus":  event.EventStatus,
						"notBefore":    event.NotBefore,
					}).Infof("detected ScheduledEvent %v with %v in %v for current node", event.EventId, event.EventType, time.Unix(int64(eventValue), 0).Sub(time.Now()).String()) //nolint:gosimple
					approveEvent = &event
					if eventValue == 1 || drainTimeThreshold >= eventValue {
						if stringArrayContainsCi(m.Conf.DrainEvents, event.EventType) {
							triggerDrain = true
						}
					}
				}
			}
		} else {
			m.prometheus.event.With(
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

	m.prometheus.documentIncarnation.With(prometheus.Labels{}).Set(float64(scheduledEvents.DocumentIncarnation))

	if len(scheduledEvents.Events) > 0 {
		log.Infof("fetched %v Azure ScheduledEvents", len(scheduledEvents.Events))
	} else {
		log.Debugf("fetched %v Azure ScheduledEvents", len(scheduledEvents.Events))

		// if event is gone, ensure uncordon of node
		if !m.nodeUncordon {
			log.Infof("ensuring uncordon of node %v", m.Conf.KubeNodeName)
			m.KubectlClient.NodeUncordon()
			m.nodeDrained = false
			m.nodeUncordon = true
		}
	}

	if m.Conf.KubeNodeName != "" {
		if approveEvent != nil && triggerDrain {
			eventLogger := log.WithField("eventID", approveEvent.EventId)

			if !m.nodeDrained {
				eventLogger.Infof("ensuring drain of node %v", m.Conf.KubeNodeName)
				m.sendNotification("draining K8s node %v (upcoming Azure ScheduledEvent %v with %s)", m.Conf.KubeNodeName, approveEvent.EventId, approveEvent.EventType)
				m.prometheus.eventDrain.WithLabelValues(approveEvent.EventId, "start").SetToCurrentTime()
				m.KubectlClient.NodeDrain()
				m.prometheus.eventDrain.WithLabelValues(approveEvent.EventId, "finish").SetToCurrentTime()
				eventLogger.Infof("drained successfully")
				m.nodeDrained = true
				m.nodeUncordon = false
			}

			if m.Conf.AzureApproveScheduledEvent {
				eventLogger.Infof("approving ScheduledEvent %v with %v", approveEvent.EventId, approveEvent.EventType)
				if err := m.AzureMetadataClient.ApproveScheduledEvent(approveEvent); err == nil {
					m.prometheus.eventApproval.WithLabelValues(approveEvent.EventId).SetToCurrentTime()
					eventLogger.Infof("event approved")
				} else {
					eventLogger.Infof("approval failed: %v", err)
				}
			}
		} else {
			if !m.nodeUncordon {
				log.Infof("ensuring uncordon of node %v", m.Conf.KubeNodeName)
				m.KubectlClient.NodeUncordon()
				m.nodeDrained = false
				m.nodeUncordon = true
			}
		}
	}
}

func (m *ScheduledEventsManager) sendNotification(message string, args ...interface{}) {
	message = fmt.Sprintf(message, args...)
	message = fmt.Sprintf(m.Conf.NotificationMsgTemplate, message)

	for _, url := range m.Conf.Notification {
		if err := shoutrrr.Send(url, message); err != nil {
			log.Errorf("unable to send shoutrrr notification: %v", err)
		}
	}
}
