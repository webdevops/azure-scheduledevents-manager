package drainmanager

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	slogio "github.com/utkuozdemir/go-slogio"
	"github.com/webdevops/go-common/log/slogger"

	"github.com/webdevops/azure-scheduledevents-manager/azuremetadata"
	"github.com/webdevops/azure-scheduledevents-manager/config"
)

type DrainManagerCommand struct {
	DrainManager
	Conf         config.Opts
	Logger       *slogger.Logger
	instanceName string
}

func (m *DrainManagerCommand) SetInstanceName(name string) {
	m.instanceName = name
}

func (m *DrainManagerCommand) InstanceName() string {
	return m.instanceName
}

func (m *DrainManagerCommand) Test() error {
	if m.Conf.Command.Test.Cmd != "" {
		m.exec(m.Conf.Command.Test.Cmd, nil)
	}

	return nil
}

func (m *DrainManagerCommand) Drain(event *azuremetadata.AzureScheduledEvent) bool {
	if m.Conf.Command.Drain.Cmd != "" {
		return m.exec(m.Conf.Command.Drain.Cmd, event)
	}
	return true
}

func (m *DrainManagerCommand) Uncordon() bool {
	if m.Conf.Command.Uncordon.Cmd != "" {
		return m.exec(m.Conf.Command.Uncordon.Cmd, nil)
	}
	return true
}

func (m *DrainManagerCommand) exec(command string, event *azuremetadata.AzureScheduledEvent) bool {
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

	cmdLogger := m.Logger.With(slog.String("command", "sh"))
	writer := &slogio.Writer{Log: cmdLogger.Slog(), Level: slogger.LevelInfo}
	defer writer.Close()

	cmd.Stdout = writer
	cmd.Stderr = writer

	m.Logger.Debugf("EXEC: %v", cmd.String())
	err := cmd.Run()
	if err != nil {
		cmdLogger.Error(err.Error())
		return false
	}

	return true
}
