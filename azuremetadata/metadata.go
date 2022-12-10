package azuremetadata

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	resty "github.com/go-resty/resty/v2"
)

type AzureMetadata struct {
	Timeout             *time.Duration
	InstanceMetadataUrl string
	ScheduledEventsUrl  string
	UserAgent           string

	restClient *resty.Client
}

func (m *AzureMetadata) Init() {
	if m.Timeout == nil {
		timeout := 30 * time.Second
		m.Timeout = &timeout
	}

	m.restClient = resty.New()
	m.restClient.SetHeader("User-Agent", m.UserAgent)
	m.restClient.SetHeader("Metadata", "true")
	m.restClient.SetHeader("Accept", "application/json")
	m.restClient.SetTimeout(*m.Timeout)

	// retry
	m.restClient.SetRetryCount(5)
	m.restClient.SetRetryMaxWaitTime(30 * time.Second)
	m.restClient.SetRetryWaitTime(10 * time.Second)
	m.restClient.AddRetryCondition(resty.RetryConditionFunc(func(r *resty.Response, err error) bool {
		// retry for 4xx and 5xx
		return r.StatusCode() >= http.StatusBadRequest
	}))
}

func (m *AzureMetadata) FetchScheduledEvents() (*AzureScheduledEventResponse, error) {
	ret := &AzureScheduledEventResponse{}

	resp, err := m.restClient.R().Get(m.ScheduledEventsUrl)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("expected HTTP status 200, got %v", resp.StatusCode())
	}

	if err = json.Unmarshal(resp.Body(), ret); err != nil {
		return nil, err
	}

	return ret, nil
}

func (m *AzureMetadata) FetchInstanceMetadata() (*AzureMetadataInstanceResponse, error) {
	ret := &AzureMetadataInstanceResponse{}

	resp, err := m.restClient.R().Get(m.InstanceMetadataUrl)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("expected HTTP status 200, got %v", resp.StatusCode())
	}

	if err = json.Unmarshal(resp.Body(), ret); err != nil {
		return nil, err
	}

	return ret, nil
}

func (m *AzureMetadata) ApproveScheduledEvent(event *AzureScheduledEvent) error {
	approvePayload := AzureScheduledEventApproval{}
	approvePayload.StartRequests = []AzureScheduledEventApprovalEvent{
		{EventId: event.EventId},
	}
	payloadBody, _ := json.Marshal(approvePayload)

	resp, err := m.restClient.R().SetBody(payloadBody).Post(m.ScheduledEventsUrl)
	if err != nil {
		return err
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("expected HTTP status 200, got %v", resp.StatusCode())
	}

	return nil
}
