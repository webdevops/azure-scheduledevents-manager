package drainmanager

import "github.com/webdevops/azure-scheduledevents-manager/azuremetadata"

type (
	DrainManager interface {
		SetInstanceName(name string)
		InstanceName() string
		Enable()
		IsEnabled() bool
		Test()
		Drain(event *azuremetadata.AzureScheduledEvent)
		Uncordon()
	}
)
