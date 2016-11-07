package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/aporeto-inc/trireme-kubernetes/auth"
	"github.com/aporeto-inc/trireme-kubernetes/config"
	"github.com/aporeto-inc/trireme-kubernetes/resolver"

	"github.com/aporeto-inc/trireme"
	"github.com/aporeto-inc/trireme/configurator"
	"github.com/aporeto-inc/trireme/enforcer"
	"github.com/aporeto-inc/trireme/enforcer/tokens"
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
		fmt.Printf("Error initializing KubernetesPolicy, exiting: %s \n", err)
		return
	}

	var trireme trireme.Trireme
	var monitor monitor.Monitor
	var publicKeyAdder enforcer.PublicKeyAdder

	// Checking statically if the Node name is not more than the maximum ServerID supported in the token package.
	if len(config.KubeNodeName) > tokens.MaxServerName {
		config.KubeNodeName = config.KubeNodeName[:tokens.MaxServerName]
		fmt.Println(config.KubeNodeName)
	}

	if config.AuthType == "PSK" {
		// Starting PSK
		glog.V(2).Infof("Starting Trireme PSK")
		trireme, monitor = configurator.NewPSKTriremeWithDockerMonitor(config.KubeNodeName, networks, kubernetesPolicy, nil, nil, config.ExistingContainerSync, []byte(config.TriremePSK))

	}
	if config.AuthType == "PKI" {
		// Starting PKI
		glog.V(2).Infof("Starting Trireme PKI")
		// Load the PKI Certs/Keys.
		pki, err := auth.LoadPKI(config.PKIDirectory)
		if err != nil {
			fmt.Printf("Error loading Certificates for PKI Trireme, exiting: %s \n", err)
			return
		}
		// Starting PKI
		trireme, monitor, publicKeyAdder = configurator.NewPKITriremeWithDockerMonitor(config.KubeNodeName, networks, kubernetesPolicy, nil, nil, config.ExistingContainerSync, pki.KeyPEM, pki.CertPEM, pki.CaCertPEM)

		// Sync the certs over all the Kubernetes Cluster.
		// 1) Adds the localCert on the localNode annotation
		// 2) Sync All the Certs from the other nodes to the CertCache (interface)
		// 3) Waits and listen for new nodes coming up.
		certs := auth.NewCertsWatcher(*kubernetesPolicy.KubernetesClient, publicKeyAdder, config.NodeAnnotationKey)
		certs.AddCertToNodeAnnotation(*kubernetesPolicy.KubernetesClient, pki.CertPEM)
		certs.SyncNodeCerts(*kubernetesPolicy.KubernetesClient)
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
