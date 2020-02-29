package main

import (
	"fmt"
	"os"
	"os/exec"
)

type KubernetesClient struct {
	nodeName string
}

func (k *KubernetesClient) SetNode(nodeName string) {
	k.nodeName = nodeName
}

func (k *KubernetesClient) CheckConnection() {
	k.exec("get", "node", k.nodeName, "--no-headers=true")
}

func (k *KubernetesClient) NodeDrain() {
	// Label
	Logger.Println("label node %v", k.nodeName)
	k.exec("label", "node", k.nodeName, "--overwrite=true", fmt.Sprintf("webdevops.io/azure-scheduledevents-manager=%v", k.nodeName))

	// DRAIN
	Logger.Println("drain node %v", k.nodeName)
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
	Logger.Println("uncordon node %v", k.nodeName)
	k.exec("uncordon", "-l", fmt.Sprintf("webdevops.io/azure-scheduledevents-manager=%v", k.nodeName))

	Logger.Println("label node %v", k.nodeName)
	k.exec("label", "node", k.nodeName, "--overwrite=true", "webdevops.io/azure-scheduledevents-manager-")
}

func (k *KubernetesClient) exec(args ...string) {
	cmd := exec.Command("/kubectl", args...)
	Logger.Verbose("EXEC: %v", cmd.String())

	cmd.String()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		panic(err)
	}
}
