package main

import (
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

// EnvNodeName is the default env. name used for the Kubernetes node name.
const EnvNodeName = "KUBERNETES_NODE"

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
	nodeName := os.Getenv(EnvNodeName)
	namespace := "default"
	// Create New PolicyEngine for Kubernetes
	kubernetesPolicy, err := resolver.NewKubernetesPolicy(kubeconfig, namespace, nodeName)
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
		isolator = trireme.NewPSKIsolator(nodeName, networks, kubernetesPolicy, nil, []byte(DefaultTriremePSK))
	} else {

		isolator = trireme.NewPKIIsolator(nodeName, networks, kubernetesPolicy, nil, pki.KeyPEM, pki.CertPEM, pki.CaCertPEM)
		auth.RegisterPKI(*kubernetesPolicy.Kubernetes, pki.CertPEM)
		certs := auth.NewCertsWatcher(*kubernetesPolicy.Kubernetes, isolator)
		certs.SyncNodeCerts(*kubernetesPolicy.Kubernetes)
		go certs.StartWatchingCerts()
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
