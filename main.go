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

// KubeConfigLocation is the default location of the KubeConfig file.
const KubeConfigLocation = "/.kube/config"

func usage() {
	fmt.Fprintf(os.Stderr, "usage: example -stderrthreshold=[INFO|WARN|FATAL] -log_dir=[string]\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func init() {
	flag.Usage = usage
	flag.Parse()
}

func main() {
	var wg sync.WaitGroup
	networks := []string{"0.0.0.0/0"}

	// Check if running inside a PoD
	// Get location of the Kubeconfig file. By default in your home.
	var kubeconfig string
	if os.Getenv("KUBERNETES_PORT") == "" {
		kubeconfig = os.Getenv("HOME") + KubeConfigLocation
	} else {
		kubeconfig = ""
	}
	namespace := "default"
	// Create New PolicyEngine for Kubernetes
	kubernetesPolicy, err := resolver.NewKubernetesPolicy(kubeconfig, namespace)
	if err != nil {
		panic(err)
	}

	// Naive implementation for PKI:
	// Trying to load the PKI infra from Kube Secret.
	// If successful, use it, if not, revert to SharedSecret.
	pki, err := auth.LoadPKI()
	var isolator trireme.Isolator
	if err != nil {
		glog.V(2).Infof("Error reading KubeSecret: %s . Falling back to PSK", err)
		isolator = trireme.NewPSKIsolator("Kubernetes", networks, kubernetesPolicy, nil, []byte(DefaultTriremePSK))
	} else {
		certCache := map[string]*ecdsa.PublicKey{}
		isolator = trireme.NewPKIIsolator("Kubernetes", networks, kubernetesPolicy, nil, pki.KeyPEM, pki.CertPEM, pki.CaCertPEM, certCache)
		auth.RegisterPKI(*kubernetesPolicy.Kubernetes, pki.CertPEM)
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
