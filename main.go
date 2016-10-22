package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/aporeto-inc/kubernetes-integration/auth"
	"github.com/aporeto-inc/kubernetes-integration/config"
	"github.com/aporeto-inc/kubernetes-integration/resolver"
	"github.com/aporeto-inc/trireme"
	"github.com/aporeto-inc/trireme/configurator"
	"github.com/aporeto-inc/trireme/enforcer"
	"github.com/aporeto-inc/trireme/monitor"
	"github.com/golang/glog"
)

func main() {
	config := config.LoadConfig()
	networks := []string{"0.0.0.0/0"}

	glog.V(2).Infof("Config used: %+v ", config)

	namespace := "default"
	// Create New PolicyEngine for  Kubernetes
	kubernetesPolicy, err := resolver.NewKubernetesPolicy(config.KubeConfigLocation, namespace, config.KubeNodeName)
	if err != nil {
		panic(err)
	}

	// Naive implementation for PKI:
	// Trying to load the PKI infra from Kube Secret.
	// If successful, use it, if not, revert to SharedSecret.
	pki, err := auth.LoadPKI(config.PKIDirectory)
	var trireme trireme.Trireme
	var monitor monitor.Monitor
	var publicKeyAdder enforcer.PublicKeyAdder

	if err != nil {
		// Starting PSK
		glog.V(2).Infof("Error reading KubeSecret: %s . Falling back to PSK", err)
		trireme, monitor = configurator.NewPSKTriremeWithDockerMonitor(config.KubeNodeName, networks, kubernetesPolicy, nil, config.ExistingContainerSync, []byte(config.TriremePSK))

	} else {
		// Starting PKI
		trireme, monitor, publicKeyAdder = configurator.NewPKITriremeWithDockerMonitor(config.KubeNodeName, networks, kubernetesPolicy, nil, config.ExistingContainerSync, pki.KeyPEM, pki.CertPEM, pki.CaCertPEM)

		// Sync the certs over all the Kubernetes Cluster.
		// 1) Adds the localCert on the localNode annotation
		// 2) Sync All the Certs from the other nodes to the CertCache (interface)
		// 3) Waits and listen for new nodes coming up.
		certs := auth.NewCertsWatcher(*kubernetesPolicy.Kubernetes, publicKeyAdder, config.NodeAnnotationKey)
		certs.AddCertToNodeAnnotation(*kubernetesPolicy.Kubernetes, pki.CertPEM)
		certs.SyncNodeCerts(*kubernetesPolicy.Kubernetes)
		go certs.StartWatchingCerts()

	}

	// Start all the go routines.
	trireme.Start()
	monitor.Start()
	kubernetesPolicy.Start()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	<-c

	fmt.Println("Bye Kubernetes!")
	kubernetesPolicy.Stop()
	monitor.Stop()
	trireme.Stop()

}
