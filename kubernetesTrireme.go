package main

import (
	"flag"
	"fmt"
	"os"
	"sync"

	"github.com/aporeto-inc/kubernetes-integration/resolver"
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
	// TODO: Change the way the Kuebrnetes config get loaded
	//kubeconfig := os.Getenv("HOME") + "/.kube/config"
	kubeconfig := ""
	namespace := "default"
	// Create New PolicyEngine for Kubernetes
	kubernetesPolicy, err := resolver.NewKubernetesPolicy(kubeconfig, namespace)
	if err != nil {
		panic(err)
	}

	// Register the PolicyEngine to the Monitor
	isolator := trireme.NewPSKIsolator("Kubernetes", networks, kubernetesPolicy, nil, []byte("SharedKey"))

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
