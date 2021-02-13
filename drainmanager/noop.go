package drainmanager

import "github.com/webdevops/azure-scheduledevents-manager/azuremetadata"

type DrainManagerNoop struct {
	DrainManager
}

func (m *DrainManagerNoop) SetInstanceName(name string)                    {}
func (m *DrainManagerNoop) Enable()                                        {}
func (m *DrainManagerNoop) Test()                                          {}
func (m *DrainManagerNoop) Drain(event *azuremetadata.AzureScheduledEvent) {}
func (m *DrainManagerNoop) Uncordon()                                      {}

func (m *DrainManagerNoop) IsEnabled() bool {
	return false
}
