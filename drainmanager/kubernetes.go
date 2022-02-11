package drainmanager

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/webdevops/azure-scheduledevents-manager/azuremetadata"
	"github.com/webdevops/azure-scheduledevents-manager/config"
	"os"
	"os/exec"
)

type DrainManagerKubernetes struct {
	DrainManager
	Conf config.Opts

	nodeName string
}

func (m *DrainManagerKubernetes) SetInstanceName(name string) {
	m.nodeName = name
}

func (m *DrainManagerKubernetes) InstanceName() string {
	return m.nodeName
}

func (m *DrainManagerKubernetes) Test() {
	m.execGet("node", m.nodeName)
}

func (m *DrainManagerKubernetes) Drain(event *azuremetadata.AzureScheduledEvent) bool {
	// Label
	log.Infof(fmt.Sprintf("label node %v", m.nodeName))
	if !m.exec("label", "node", m.nodeName, "--overwrite=true", fmt.Sprintf("webdevops.io/azure-scheduledevents-manager=%v", m.nodeName)) {
		return false
	}

	// DRAIN
	log.Infof(fmt.Sprintf("drain node %v", m.nodeName))
	kubectlDrainOpts := []string{"drain", m.nodeName}
	kubectlDrainOpts = append(kubectlDrainOpts, m.Conf.Kubernetes.Drain.Args...)
	return m.exec(kubectlDrainOpts...)
}

func (m *DrainManagerKubernetes) Uncordon() bool {
	log.Infof(fmt.Sprintf("uncordon node %v", m.nodeName))
	if !m.exec("uncordon", "-l", fmt.Sprintf("webdevops.io/azure-scheduledevents-manager=%v", m.nodeName)) {
		return false
	}

	log.Infof(fmt.Sprintf("remove label node %v", m.nodeName))
	return m.exec("label", "node", m.nodeName, "--overwrite=true", "webdevops.io/azure-scheduledevents-manager-")
}

func (m *DrainManagerKubernetes) execGet(resourceType string, args ...string) bool {
	kubectlArgs := []string{
		"get",
		"--no-headers=true",
		resourceType,
	}
	kubectlArgs = append(kubectlArgs, args...)
	return m.runComand(exec.Command("/kubectl", kubectlArgs...)) // #nosec G204
}

func (m *DrainManagerKubernetes) exec(args ...string) bool {
	if m.Conf.Kubernetes.Drain.DryRun {
		args = append(args, "--dry-run")
	}

	return m.runComand(exec.Command("/kubectl", args...))
}

func (m *DrainManagerKubernetes) runComand(cmd *exec.Cmd) bool {
	cmd.Env = os.Environ()

	cmdLogger := log.WithField("command", "kubectl")
	log.Debugf("EXEC: %v", cmd.String())
	cmd.Stdout = cmdLogger.WriterLevel(log.InfoLevel)
	cmd.Stderr = cmdLogger.WriterLevel(log.ErrorLevel)
	err := cmd.Run()
	if err != nil {
		cmdLogger.Error(err)
		return false
	}
	return true
}
