package main

import (
	"crypto/ecdsa"
	"flag"
	"fmt"
	"os"
	"sync"

	"github.com/aporeto-inc/kubernetes-integration/auth"
	"github.com/aporeto-inc/kubernetes-integration/resolver"
	"github.com/aporeto-inc/trireme"
	"github.com/golang/glog"
)

// DefaultTriremePSK is used fas the default PSK for trireme if not overriden by the user.
const DefaultTriremePSK = "Trireme"

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
	kubeconfig := os.Getenv("HOME") + "/.kube/config"
	namespace := "default"
	// Create New PolicyEngine for Kubernetes
	kubernetesPolicy, err := resolver.NewKubernetesPolicy(kubeconfig, namespace)
	if err != nil {
		panic(err)
	}

	// Naive implementation for PKI:
	// Trying to load the PKI infra from Kube Secret.
	// If successful, use it, if not, revert to SharedSecret.
	pki, err := auth.LoadPKIFromKubeSecret()
	var isolator trireme.Isolator
	if err != nil {
		glog.V(2).Infof("Error reading KubeSecret: %s . Falling back to PSK", err)
		isolator = trireme.NewPSKIsolator("Kubernetes", networks, kubernetesPolicy, nil, []byte(DefaultTriremePSK))
	} else {
		certCache := map[string]*ecdsa.PublicKey{}
		isolator = trireme.NewPKIIsolator("Kubernetes", networks, kubernetesPolicy, nil, pki.KeyPEM, pki.CertPEM, pki.CaCertPEM, certCache)
	}

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
