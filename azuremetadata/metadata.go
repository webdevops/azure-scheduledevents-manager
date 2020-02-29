package azuremetadata

import (
	"encoding/json"
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

	err = json.NewDecoder(resp.Body).Decode(&ret)
	if err != nil {
		return nil, err
	}

	return ret, nil
}
