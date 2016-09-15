package main

// This has to be refactored soon enough. For now, just using a simple
// Proof of concept of an example Kubernetes integration.
// This is based on the original example code from trireme/example/example.go

import (
	"flag"
	"fmt"
	"os"
	"sync"

	"github.com/aporeto-inc/kubernetes-integration/policy"
	"github.com/aporeto-inc/trireme"
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: example -stderrthreshold=[INFO|WARN|FATAL] -log_dir=[string]\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func init() {
	flag.Usage = usage
	// NOTE: This next line is key you have to call flag.Parse() for the command line
	// options or "flags" that are defined in the glog module to be picked up.
	flag.Parse()
}

func main() {
	var wg sync.WaitGroup
	networks := []string{"0.0.0.0/0"}
	// Get location of the Kubeconfig file. By default in your home.
	kubeconfig := os.Getenv("HOME") + "/.kube/config"
	namespace := "default"
	// Create New PolicyEngine for Kubernetes
	kubernetesPolicy, err := policy.NewKubernetesPolicy(kubeconfig, namespace)
	if err != nil {
		panic(err)
	}

	// Register the PolicyEngine to the Monitor
	isolator := trireme.NewIsolator(networks, kubernetesPolicy, nil)

	// Register the Isolator to KubernetesPolicy for UpdatePolicies callback
	kubernetesPolicy.RegisterIsolator(isolator)

	// Start all the go routines.
	wg.Add(2)
	// Start monitoring Docker policies.
	isolator.Start()
	// Start monitoring Kubernetes Policies.
	kubernetesPolicy.Start()
	wg.Wait()
}
