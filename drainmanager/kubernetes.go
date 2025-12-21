package drainmanager

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"github.com/utkuozdemir/go-slogio"
	"github.com/webdevops/go-common/log/slogger"

	"github.com/webdevops/azure-scheduledevents-manager/azuremetadata"
	"github.com/webdevops/azure-scheduledevents-manager/config"
)

type DrainManagerKubernetes struct {
	DrainManager
	Conf   config.Opts
	Logger *slogger.Logger

	nodeName string
}

func (m *DrainManagerKubernetes) SetInstanceName(name string) {
	m.nodeName = name
}

func (m *DrainManagerKubernetes) InstanceName() string {
	return m.nodeName
}

func (m *DrainManagerKubernetes) Test() error {
	if !m.execGet("node", m.nodeName) {
		return errors.New(`unable to get node from kubernetes api`)
	}
	return nil
}

func (m *DrainManagerKubernetes) Drain(event *azuremetadata.AzureScheduledEvent) bool {
	// Label
	m.Logger.Info("label node", slog.String("node", m.nodeName))
	if !m.exec("label", "node", m.nodeName, "--overwrite=true", fmt.Sprintf("webdevops.io/azure-scheduledevents-manager=%v", m.nodeName)) {
		return false
	}

	// DRAIN
	m.Logger.Info("drain node", slog.String("node", m.nodeName))
	kubectlDrainOpts := []string{"drain", m.nodeName}
	kubectlDrainOpts = append(kubectlDrainOpts, m.Conf.Kubernetes.Drain.Args...)
	return m.exec(kubectlDrainOpts...)
}

func (m *DrainManagerKubernetes) Uncordon() bool {
	m.Logger.Info("uncordon node", slog.String("node", m.nodeName))
	if !m.exec("uncordon", "-l", fmt.Sprintf("webdevops.io/azure-scheduledevents-manager=%v", m.nodeName)) {
		return false
	}

	m.Logger.Info("remove label node", slog.String("node", m.nodeName))
	return m.exec("label", "node", m.nodeName, "--overwrite=true", "webdevops.io/azure-scheduledevents-manager-")
}

func (m *DrainManagerKubernetes) execGet(resourceType string, args ...string) bool {
	kubectlArgs := []string{
		"get",
		"--no-headers=true",
		resourceType,
	}
	kubectlArgs = append(kubectlArgs, args...)
	return m.runComand(exec.Command("kubectl", kubectlArgs...)) // #nosec G204
}

func (m *DrainManagerKubernetes) exec(args ...string) bool {
	if m.Conf.Kubernetes.Drain.DryRun {
		args = append(args, "--dry-run=client")
	}

	return m.runComand(exec.Command("kubectl", args...))
}

func (m *DrainManagerKubernetes) runComand(cmd *exec.Cmd) bool {
	cmd.Env = os.Environ()

	cmdLogger := m.Logger.With(slog.String("command", "kubectl"))
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
