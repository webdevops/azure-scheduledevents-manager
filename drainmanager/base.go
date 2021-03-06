package drainmanager

import "github.com/webdevops/azure-scheduledevents-manager/azuremetadata"

type (
	DrainManager interface {
		SetInstanceName(name string)
		InstanceName() string
		Test()
		Drain(event *azuremetadata.AzureScheduledEvent) bool
		Uncordon() bool
	}
)
