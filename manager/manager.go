package manager

import (
	"fmt"
	"time"

	"github.com/containrrr/shoutrrr"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/webdevops/azure-scheduledevents-manager/azuremetadata"
	"github.com/webdevops/azure-scheduledevents-manager/config"
	"github.com/webdevops/azure-scheduledevents-manager/drainmanager"
)

type (
	ScheduledEventsManager struct {
		apiErrorCount int
		nodeDrained   bool
		nodeUncordon  bool

		OnClear           func()
		OnScheduledEvent  func()
		OnAfterDrainEvent func()

		Conf                config.Opts
		Logger              *zap.SugaredLogger
		AzureMetadataClient *azuremetadata.AzureMetadata
		DrainManager        drainmanager.DrainManager

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
		[]string{
			"eventID",
			"eventType",
			"resourceType",
			"resource",
			"eventStatus",
			"notBefore",
			"eventSource",
		},
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
			time.Sleep(m.Conf.Scrape.Time)
		}
	}()
}

func (m *ScheduledEventsManager) collect() {
	var approveEvent *azuremetadata.AzureScheduledEvent
	triggerDrain := false

	drainTimeThreshold := float64(time.Now().Add(m.Conf.Drain.NotBefore).Unix())

	startTime := time.Now()
	scheduledEvents, err := m.AzureMetadataClient.FetchScheduledEvents()
	if err != nil {
		m.apiErrorCount++
		m.prometheus.requestErrors.With(prometheus.Labels{}).Inc()

		if m.Conf.Azure.ErrorThreshold <= 0 || m.apiErrorCount <= m.Conf.Azure.ErrorThreshold {
			m.Logger.Errorf("failed API call: %s", err)
			return
		} else {
			panic(err.Error())
		}
	}

	if m.Conf.Metrics.RequestStats {
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
			m.Logger.Errorf("unable to parse time \"%s\" of eventid \"%v\": %v", event.NotBefore, event.EventId, err)
			eventValue = 0
		}

		if len(event.Resources) >= 1 {
			for _, resource := range event.Resources {
				m.Logger.With(
					zap.String("eventID", event.EventId),
					zap.String("eventType", event.EventType),
					zap.String("resourceType", event.ResourceType),
					zap.String("resource", resource),
					zap.String("eventStatus", event.EventStatus),
					zap.String("notBefore", event.NotBefore),
					zap.String("eventSource", event.EventSource),
				).Debugf("found ScheduledEvent")

				m.prometheus.event.With(
					prometheus.Labels{
						"eventID":      event.EventId,
						"eventType":    event.EventType,
						"resourceType": event.ResourceType,
						"resource":     resource,
						"eventStatus":  event.EventStatus,
						"notBefore":    event.NotBefore,
						"eventSource":  event.EventSource,
					}).Set(eventValue)

				if m.Conf.Instance.VmNodeName != "" && resource == m.Conf.Instance.VmNodeName {
					m.Logger.With(
						zap.String("eventID", event.EventId),
						zap.String("eventType", event.EventType),
						zap.String("resourceType", event.ResourceType),
						zap.String("resource", resource),
						zap.String("eventStatus", event.EventStatus),
						zap.String("notBefore", event.NotBefore),
						zap.String("eventSource", event.EventSource),
					).Infof("detected ScheduledEvent %v with %v by %v in %v for current node", event.EventId, event.EventSource, event.EventType, time.Unix(int64(eventValue), 0).Sub(time.Now()).String()) //nolint:gosimple
					approveEvent = &event
					if eventValue == 1 || drainTimeThreshold >= eventValue {
						if stringArrayContainsCi(m.Conf.Drain.Events, event.EventType) {
							if m.OnScheduledEvent != nil {
								m.OnScheduledEvent()
							}

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
					"eventSource":  event.EventSource,
				}).Set(eventValue)

			m.Logger.With(
				zap.String("eventID", event.EventId),
				zap.String("eventType", event.EventType),
				zap.String("resourceType", event.ResourceType),
				zap.String("resource", ""),
				zap.String("eventStatus", event.EventStatus),
				zap.String("notBefore", event.NotBefore),
				zap.String("eventSource", event.EventSource),
			).Debugf("found ScheduledEvent")
		}
	}

	m.prometheus.documentIncarnation.With(prometheus.Labels{}).Set(float64(scheduledEvents.DocumentIncarnation))

	if len(scheduledEvents.Events) > 0 {
		m.Logger.Infof("found %v Azure ScheduledEvents", len(scheduledEvents.Events))
	} else {
		m.Logger.Debugf("found %v Azure ScheduledEvents", len(scheduledEvents.Events))
		m.OnClear()

		// if event is gone, ensure uncordon of node
		if !m.nodeUncordon && m.DrainManager != nil {
			m.Logger.Infof("ensuring uncordon of instance %v", m.instanceName())
			if m.DrainManager.Uncordon() {
				m.Logger.Infof("uncordon finished")
				m.nodeDrained = false
				m.nodeUncordon = true
			} else {
				m.Logger.Infof("uncordon failed")
			}
		}
	}

	// trigger clear event if no approve event is found or no events at all
	if approveEvent == nil || len(scheduledEvents.Events) == 0 {
		m.OnClear()
	}

	if m.Conf.Drain.Enable {
		if approveEvent != nil && triggerDrain {
			eventLogger := m.Logger.With(zap.String("eventID", approveEvent.EventId))

			if !m.nodeDrained {
				eventLogger.Infof("ensuring drain of instance %v", m.instanceName())
				m.sendNotification("draining instance %v: upcoming Azure ScheduledEvent %v with %s by %s: %v", m.instanceName(), approveEvent.EventId, approveEvent.EventType, approveEvent.EventSource, approveEvent.Description)
				m.prometheus.eventDrain.WithLabelValues(approveEvent.EventId, "start").SetToCurrentTime()

				if m.Conf.Drain.WaitBeforeCmd.Seconds() >= 1 {
					eventLogger.Infof("wait %v before drain", m.Conf.Drain.WaitBeforeCmd.String())
					time.Sleep(m.Conf.Drain.WaitBeforeCmd)
				}

				if m.DrainManager != nil {
					if m.DrainManager.Drain(approveEvent) {
						eventLogger.Infof("drained successfully")
						m.nodeDrained = true
						m.nodeUncordon = false
					} else {
						eventLogger.Infof("drained failed")
					}
				}

				if m.Conf.Drain.WaitAfterCmd.Seconds() >= 1 {
					eventLogger.Infof("wait %v after drain", m.Conf.Drain.WaitBeforeCmd.String())
					time.Sleep(m.Conf.Drain.WaitAfterCmd)
				}

				if m.OnAfterDrainEvent != nil {
					m.OnAfterDrainEvent()
				}

				m.prometheus.eventDrain.WithLabelValues(approveEvent.EventId, "finish").SetToCurrentTime()
			}

			if m.Conf.Azure.ApproveScheduledEvent {
				eventLogger.Infof("approving ScheduledEvent %v with %v by %v", approveEvent.EventId, approveEvent.EventType, approveEvent.EventSource)
				if err := m.AzureMetadataClient.ApproveScheduledEvent(approveEvent); err == nil {
					m.prometheus.eventApproval.WithLabelValues(approveEvent.EventId).SetToCurrentTime()
					eventLogger.Infof("event approved")
				} else {
					eventLogger.Infof("approval failed: %v", err)
				}
			}
		} else {
			if !m.nodeUncordon && m.DrainManager != nil {
				m.Logger.Infof("ensuring uncordon of instance %v", m.instanceName())
				if m.DrainManager.Uncordon() {
					m.Logger.Infof("uncordon finished")
					m.nodeDrained = false
					m.nodeUncordon = true
				} else {
					m.Logger.Infof("uncordon failed")
				}
			}
		}
	}
}

func (m *ScheduledEventsManager) instanceName() string {
	if m.DrainManager != nil {
		drainManagerInstanceName := m.DrainManager.InstanceName()

		if drainManagerInstanceName == m.Conf.Instance.VmNodeName {
			return drainManagerInstanceName
		} else {
			return fmt.Sprintf("%v (vm: %v)", drainManagerInstanceName, m.Conf.Instance.VmNodeName)
		}
	}

	return m.Conf.Instance.VmNodeName
}

func (m *ScheduledEventsManager) sendNotification(message string, args ...interface{}) {
	message = fmt.Sprintf(message, args...)
	message = fmt.Sprintf(m.Conf.Notification.MsgTemplate, message)

	for _, url := range m.Conf.Notification.List {
		if err := shoutrrr.Send(url, message); err != nil {
			m.Logger.Errorf("unable to send shoutrrr notification: %v", err)
		}
	}
}
