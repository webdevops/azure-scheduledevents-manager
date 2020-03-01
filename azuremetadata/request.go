package azuremetadata

type (
	AzureScheduledEventApproval struct {
		StartRequests []AzureScheduledEventApprovalEvent `json:"StartRequests"`
	}

	AzureScheduledEventApprovalEvent struct {
		EventId string `json:"EventId"`
	}
)
