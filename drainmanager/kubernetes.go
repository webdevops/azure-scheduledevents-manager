package drainmanager

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/webdevops/azure-scheduledevents-manager/azuremetadata"
	"github.com/webdevops/azure-scheduledevents-manager/config"
	"os/exec"
)

type DrainManagerKubernetes struct {
	DrainManager
	Conf config.Opts

	nodeName string
	enabled  bool
}

func (m *DrainManagerKubernetes) SetInstanceName(name string) {
	m.nodeName = name
}

func (m *DrainManagerKubernetes) InstanceName() string {
	return m.nodeName
}

func (m *DrainManagerKubernetes) Enable() {
	m.enabled = true
}

func (m *DrainManagerKubernetes) IsEnabled() bool {
	return m.enabled
}

func (m *DrainManagerKubernetes) Test() {
	m.execGet("node", m.nodeName)
}

func (m *DrainManagerKubernetes) Drain(event *azuremetadata.AzureScheduledEvent) {
	if !m.enabled {
		return
	}

	// Label
	log.Infof(fmt.Sprintf("label node %v", m.nodeName))
	m.exec("label", "node", m.nodeName, "--overwrite=true", fmt.Sprintf("webdevops.io/azure-scheduledevents-manager=%v", m.nodeName))

	// DRAIN
	log.Infof(fmt.Sprintf("drain node %v", m.nodeName))
	kubectlDrainOpts := []string{"drain", m.nodeName}
	kubectlDrainOpts = append(kubectlDrainOpts, fmt.Sprintf("--timeout=%v", m.Conf.Kubernetes.Drain.Timeout.String()))

	if m.Conf.Kubernetes.Drain.DeleteLocalData {
		kubectlDrainOpts = append(kubectlDrainOpts, "--delete-local-data=true")
	}

	if m.Conf.Kubernetes.Drain.Force {
		kubectlDrainOpts = append(kubectlDrainOpts, "--force=true")
	}

	if m.Conf.Kubernetes.Drain.GracePeriod != 0 {
		kubectlDrainOpts = append(kubectlDrainOpts, fmt.Sprintf("--grace-period=%v", m.Conf.Kubernetes.Drain.GracePeriod))
	}

	if m.Conf.Kubernetes.Drain.IgnoreDaemonsets {
		kubectlDrainOpts = append(kubectlDrainOpts, "--ignore-daemonsets=true")
	}

	if m.Conf.Kubernetes.Drain.PodSelector != "" {
		kubectlDrainOpts = append(kubectlDrainOpts, fmt.Sprintf("--pod-selector=%v", m.Conf.Kubernetes.Drain.PodSelector))
	}

	m.exec(kubectlDrainOpts...)
}

func (m *DrainManagerKubernetes) Uncordon() {
	if !m.enabled {
		return
	}

	log.Infof(fmt.Sprintf("uncordon node %v", m.nodeName))
	m.exec("uncordon", "-l", fmt.Sprintf("webdevops.io/azure-scheduledevents-manager=%v", m.nodeName))

	log.Infof(fmt.Sprintf("remove label node %v", m.nodeName))
	m.exec("label", "node", m.nodeName, "--overwrite=true", "webdevops.io/azure-scheduledevents-manager-")
}

func (m *DrainManagerKubernetes) execGet(resourceType string, args ...string) {
	kubectlArgs := []string{
		"get",
		"--no-headers=true",
		resourceType,
	}
	kubectlArgs = append(kubectlArgs, args...)
	m.runComand(exec.Command("/kubectl", kubectlArgs...))
}

func (m *DrainManagerKubernetes) exec(args ...string) {
	if m.Conf.Kubernetes.Drain.DryRun {
		args = append(args, "--dry-run")
	}

	m.runComand(exec.Command("/kubectl", args...))
}

func (m *DrainManagerKubernetes) runComand(cmd *exec.Cmd) {
	cmdLogger := log.WithField("command", "kubectl")
	log.Debugf("EXEC: %v", cmd.String())
	cmd.Stdout = cmdLogger.WriterLevel(log.InfoLevel)
	cmd.Stderr = cmdLogger.WriterLevel(log.ErrorLevel)
	err := cmd.Run()
	if err != nil {
		cmdLogger.Panic(err)
	}
}
