package drainmanager

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/webdevops/azure-scheduledevents-manager/azuremetadata"
	"github.com/webdevops/azure-scheduledevents-manager/config"
	"os"
	"os/exec"
	"strings"
)

type DrainManagerCommand struct {
	DrainManager
	Conf         config.Opts
	instanceName string
}

func (m *DrainManagerCommand) SetInstanceName(name string) {
	m.instanceName = name
}

func (m *DrainManagerCommand) InstanceName() string {
	return m.instanceName
}

func (m *DrainManagerCommand) Test() {
	if m.Conf.Command.Test.Cmd != "" {
		m.exec(m.Conf.Command.Test.Cmd, nil)
	}
}

func (m *DrainManagerCommand) Drain(event *azuremetadata.AzureScheduledEvent) {
	if m.Conf.Command.Drain.Cmd != "" {
		m.exec(m.Conf.Command.Drain.Cmd, event)
	}
}

func (m *DrainManagerCommand) Uncordon() {
	if m.Conf.Command.Uncordon.Cmd != "" {
		m.exec(m.Conf.Command.Uncordon.Cmd, nil)
	}
}

func (m *DrainManagerCommand) exec(command string, event *azuremetadata.AzureScheduledEvent) {
	env := os.Environ()
	if event != nil {
		env = append(env, fmt.Sprintf("EVENT_ID=%v", event.EventId))
		env = append(env, fmt.Sprintf("EVENT_SOURCE=%v", event.EventSource))
		env = append(env, fmt.Sprintf("EVENT_STATUS=%v", event.EventStatus))
		env = append(env, fmt.Sprintf("EVENT_TYPE=%v", event.EventType))
		env = append(env, fmt.Sprintf("EVENT_NOTBEFORE=%v", event.NotBefore))
		env = append(env, fmt.Sprintf("EVENT_RESOURCES=%v", strings.Join(event.Resources, " ")))
		env = append(env, fmt.Sprintf("EVENT_RESOURCETYPE=%v", event.ResourceType))
	}

	cmd := exec.Command("sh", "-c", command)
	cmd.Env = env
	cmdLogger := log.WithField("command", "sh")
	log.Debugf("EXEC: %v", cmd.String())
	cmd.Stdout = cmdLogger.WriterLevel(log.InfoLevel)
	cmd.Stderr = cmdLogger.WriterLevel(log.ErrorLevel)
	err := cmd.Run()
	if err != nil {
		cmdLogger.Panic(err)
	}
}
