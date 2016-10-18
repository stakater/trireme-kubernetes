package main

import (
	"sync"

	"github.com/aporeto-inc/kubernetes-integration/auth"
	"github.com/aporeto-inc/kubernetes-integration/config"
	"github.com/aporeto-inc/kubernetes-integration/resolver"
	"github.com/aporeto-inc/trireme"
	"github.com/golang/glog"
)

func main() {
	var wg sync.WaitGroup
	config := config.LoadConfig()
	networks := []string{"0.0.0.0/0"}

	glog.V(2).Infof("Config used: %+v ", config)

	namespace := "default"
	// Create New PolicyEngine for Kubernetes
	kubernetesPolicy, err := resolver.NewKubernetesPolicy(config.KubeConfigLocation, namespace, config.KubeNodeName)
	if err != nil {
		panic(err)
	}

	// Naive implementation for PKI:
	// Trying to load the PKI infra from Kube Secret.
	// If successful, use it, if not, revert to SharedSecret.
	pki, err := auth.LoadPKI(config.PKIDirectory)
	var helper *trireme.Helper
	if err != nil {
		// Starting PSK
		glog.V(2).Infof("Error reading KubeSecret: %s . Falling back to PSK", err)
		helper = trireme.NewPSKTrireme(config.KubeNodeName, networks, kubernetesPolicy, nil, []byte(config.TriremePSK))
	} else {
		// Starting PKI
		helper = trireme.NewPKITrireme(config.KubeNodeName, networks, kubernetesPolicy, nil, pki.KeyPEM, pki.CertPEM, pki.CaCertPEM)
		certs := auth.NewCertsWatcher(*kubernetesPolicy.Kubernetes, helper.PkAdder, config.NodeAnnotationKey)
		certs.RegisterPKI(*kubernetesPolicy.Kubernetes, pki.CertPEM)
		certs.SyncNodeCerts(*kubernetesPolicy.Kubernetes)
		go certs.StartWatchingCerts()
	}

	// Start all the go routines.
	wg.Add(2)
	// Start monitoring Docker policies.
	helper.Trireme.Start()
	// Start monitoring Kubernetes Policies.
	kubernetesPolicy.Start()
	wg.Wait()
}
