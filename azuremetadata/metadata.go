package azuremetadata

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

type AzureMetadata struct {
	Timeout             *time.Duration
	InstanceMetadataUrl string
	ScheduledEventsUrl  string
	HttpClient          *http.Client
}

func (m *AzureMetadata) Init() {
	if m.Timeout == nil {
		timeout := time.Duration(30 * time.Second)
		m.Timeout = &timeout
	}

	// Init http client
	m.HttpClient = &http.Client{
		Timeout: *m.Timeout,
	}
}

func (m *AzureMetadata) FetchScheduledEvents() (*AzureScheduledEventResponse, error) {
	ret := &AzureScheduledEventResponse{}

	req, err := http.NewRequest("GET", m.ScheduledEventsUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Metadata", "true")

	resp, err := m.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("Expected HTTP status 200, got %v", resp.StatusCode))
	}

	err = json.NewDecoder(resp.Body).Decode(&ret)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (m *AzureMetadata) FetchInstanceMetadata() (*AzureMetadataInstanceResponse, error) {
	ret := &AzureMetadataInstanceResponse{}

	req, err := http.NewRequest("GET", m.InstanceMetadataUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Metadata", "true")

	resp, err := m.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf("Expected HTTP status 200, got %v", resp.StatusCode))
	}

	err = json.NewDecoder(resp.Body).Decode(&ret)
	if err != nil {
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

	req, err := http.NewRequest("POST", m.ScheduledEventsUrl, bytes.NewBuffer(payloadBody))
	if err != nil {
		return err
	}
	req.Header.Add("Metadata", "true")
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.HttpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New(fmt.Sprintf("Expected HTTP status 200, got %v", resp.StatusCode))
	}

	return nil
}
