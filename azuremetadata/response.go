package azuremetadata

import (
	"time"
)

var (
	timeFormatList = []string{
		time.RFC3339,
		time.RFC1123,
		time.RFC822Z,
		time.RFC850,
	}
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

type AzureMetadataInstanceResponse struct {
	Compute struct {
		Location             string `json:"location"`
		Name                 string `json:"name"`
		Offer                string `json:"offer"`
		OsType               string `json:"osType"`
		PlacementGroupID     string `json:"placementGroupId"`
		PlatformFaultDomain  string `json:"platformFaultDomain"`
		PlatformUpdateDomain string `json:"platformUpdateDomain"`
		Publisher            string `json:"publisher"`
		ResourceGroupName    string `json:"resourceGroupName"`
		Sku                  string `json:"sku"`
		SubscriptionID       string `json:"subscriptionId"`
		Tags                 string `json:"tags"`
		Version              string `json:"version"`
		VMID                 string `json:"vmId"`
		VMSize               string `json:"vmSize"`
	} `json:"compute"`
}

func (e *AzureScheduledEvent) NotBeforeUnixTimestamp() (eventValue float64, err error) {
	// default
	eventValue = 1
	err = nil

	if e.NotBefore != "" {
		notBefore, parseErr := parseTime(e.NotBefore)
		if parseErr == nil {
			eventValue = float64(notBefore.Unix())
		} else {
			err = parseErr
			eventValue = 0
		}
	}

	return
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
