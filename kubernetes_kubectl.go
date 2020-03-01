package main

import (
	"fmt"
	"github.com/kvz/logstreamer"
	"os/exec"
)

type KubernetesClient struct {
	nodeName string
	enabled  bool
}

func (k *KubernetesClient) SetNode(nodeName string) {
	k.nodeName = nodeName
}

func (k *KubernetesClient) Enable() {
	k.enabled = true
}

func (k *KubernetesClient) CheckConnection() {
	k.execGet("node", k.nodeName)
}

func (k *KubernetesClient) NodeDrain() {
	if !k.enabled {
		return
	}

	// Label
	Logger.Println(fmt.Sprintf("label node %v", k.nodeName))
	k.exec("label", "node", k.nodeName, "--overwrite=true", fmt.Sprintf("webdevops.io/azure-scheduledevents-manager=%v", k.nodeName))

	// DRAIN
	Logger.Println(fmt.Sprintf("drain node %v", k.nodeName))
	kubectlDrainOpts := []string{"drain", k.nodeName}
	kubectlDrainOpts = append(kubectlDrainOpts, fmt.Sprintf("--timeout=%v", opts.DrainTimeout.String()))

	if opts.DrainDeleteLocalData {
		kubectlDrainOpts = append(kubectlDrainOpts, "--delete-local-data=true")
	}

	if opts.DrainForce {
		kubectlDrainOpts = append(kubectlDrainOpts, "--force=true")
	}

	if opts.DrainGracePeriod != 0 {
		kubectlDrainOpts = append(kubectlDrainOpts, fmt.Sprintf("--grace-period=%v", opts.DrainGracePeriod))
	}

	if opts.DrainIgnoreDaemonsets {
		kubectlDrainOpts = append(kubectlDrainOpts, "--ignore-daemonsets=true")
	}

	if opts.DrainPodSelector != "" {
		kubectlDrainOpts = append(kubectlDrainOpts, fmt.Sprintf("--pod-selector=%v", opts.DrainPodSelector))
	}

	k.exec(kubectlDrainOpts...)
}

func (k *KubernetesClient) NodeUncordon() {
	if !k.enabled {
		return
	}

	Logger.Println(fmt.Sprintf("uncordon node %v", k.nodeName))
	k.exec("uncordon", "-l", fmt.Sprintf("webdevops.io/azure-scheduledevents-manager=%v", k.nodeName))

	Logger.Println(fmt.Sprintf("label node %v", k.nodeName))
	k.exec("label", "node", k.nodeName, "--overwrite=true", "webdevops.io/azure-scheduledevents-manager-")
}

func (k *KubernetesClient) execGet(resourceType string, args ...string) {
	kubectlArgs := []string{
		"get",
		"--no-headers=true",
		resourceType,
	}
	kubectlArgs = append(kubectlArgs, args...)
	k.runComand(exec.Command("/kubectl", kubectlArgs...))
}

func (k *KubernetesClient) exec(args ...string) {
	if opts.DrainDryRun {
		args = append(args, "--dry-run")
	}

	k.runComand(exec.Command("/kubectl", args...))
}

func (k *KubernetesClient) runComand(cmd *exec.Cmd) {
	logStreamerOut := logstreamer.NewLogstreamer(Logger.Logger, "kubectl >> ", false)
	defer logStreamerOut.Close()

	logStreamerErr := logstreamer.NewLogstreamer(ErrorLogger.Logger, "kubectl >> ", true)
	defer logStreamerErr.Close()

	Logger.Verbose("EXEC: %v", cmd.String())
	cmd.Stdout = logStreamerOut
	cmd.Stderr = logStreamerErr
	err := cmd.Run()
	if err != nil {
		panic(err)
	}
}
