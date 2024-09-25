package drainmanager

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"go.uber.org/zap"
	"go.uber.org/zap/zapio"

	"github.com/webdevops/azure-scheduledevents-manager/azuremetadata"
	"github.com/webdevops/azure-scheduledevents-manager/config"
)

type DrainManagerKubernetes struct {
	DrainManager
	Conf   config.Opts
	Logger *zap.SugaredLogger

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
	m.Logger.Infof(fmt.Sprintf("label node %v", m.nodeName))
	if !m.exec("label", "node", m.nodeName, "--overwrite=true", fmt.Sprintf("webdevops.io/azure-scheduledevents-manager=%v", m.nodeName)) {
		return false
	}

	// DRAIN
	m.Logger.Infof(fmt.Sprintf("drain node %v", m.nodeName))
	kubectlDrainOpts := []string{"drain", m.nodeName}
	kubectlDrainOpts = append(kubectlDrainOpts, m.Conf.Kubernetes.Drain.Args...)
	return m.exec(kubectlDrainOpts...)
}

func (m *DrainManagerKubernetes) Uncordon() bool {
	m.Logger.Infof(fmt.Sprintf("uncordon node %v", m.nodeName))
	if !m.exec("uncordon", "-l", fmt.Sprintf("webdevops.io/azure-scheduledevents-manager=%v", m.nodeName)) {
		return false
	}

	m.Logger.Infof(fmt.Sprintf("remove label node %v", m.nodeName))
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
		args = append(args, "--dry-run")
	}

	return m.runComand(exec.Command("kubectl", args...))
}

func (m *DrainManagerKubernetes) runComand(cmd *exec.Cmd) bool {
	cmd.Env = os.Environ()

	cmdLogger := m.Logger.With(zap.String("command", "kubectl")).Desugar()
	cmdLogger = cmdLogger.WithOptions(zap.AddStacktrace(zap.PanicLevel), zap.WithCaller(false))
	m.Logger.Debugf("EXEC: %v", cmd.String())

	stdOutWriter := &zapio.Writer{Log: cmdLogger, Level: zap.InfoLevel}
	defer stdOutWriter.Close()

	stdErrWriter := &zapio.Writer{Log: cmdLogger, Level: zap.ErrorLevel}
	defer stdErrWriter.Close()

	cmd.Stdout = stdOutWriter
	cmd.Stderr = stdErrWriter
	err := cmd.Run()
	if err != nil {
		cmdLogger.Error(err.Error())
		return false
	}
	return true
}
