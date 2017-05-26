package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/aporeto-inc/trireme-kubernetes/auth"
	"github.com/aporeto-inc/trireme-kubernetes/config"
	"github.com/aporeto-inc/trireme-kubernetes/resolver"

	"github.com/aporeto-inc/trireme"
	"github.com/aporeto-inc/trireme/cmd/remoteenforcer"
	"github.com/aporeto-inc/trireme/configurator"
	"github.com/aporeto-inc/trireme/enforcer"
	"github.com/aporeto-inc/trireme/enforcer/utils/tokens"
	"github.com/aporeto-inc/trireme/monitor"

	"github.com/golang/glog"

	"go.uber.org/zap"
)

func main() {
	config := config.LoadConfig()

	zapConfig := zap.NewDevelopmentConfig()
	zapConfig.DisableStacktrace = true

	// Set statitcally for now
	level := "info"

	// Set the logger
	switch level {
	case "trace", "debug":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	case "fatal":
		zapConfig.Level = zap.NewAtomicLevelAt(zap.FatalLevel)
	default:
		zapConfig.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	logger, err := zapConfig.Build()
	if err != nil {
		panic(err)
	}

	zap.ReplaceGlobals(logger)

	if config.Enforcer {
		fmt.Println("Launching enforcer:")

		glog.V(2).Infof("Launching enforcer: %+v ", config)
		remoteenforcer.LaunchRemoteEnforcer(nil)
		return
	}

	glog.V(2).Infof("Config used: %+v ", config)

	// Create New PolicyEngine for Kubernetes
	kubernetesPolicy, err := resolver.NewKubernetesPolicy(config.KubeConfigLocation, config.KubeNodeName, config.TriremeNets)
	if err != nil {
		fmt.Printf("Error initializing KubernetesPolicy, exiting: %s \n", err)
		return
	}

	var trireme trireme.Trireme
	var monitor monitor.Monitor
	var publicKeyAdder enforcer.PublicKeyAdder

	// Checking statically if the node name is not more than the maximum ServerID
	// length supported by Trireme.
	if len(config.KubeNodeName) > tokens.MaxServerName {
		config.KubeNodeName = config.KubeNodeName[:tokens.MaxServerName]
	}

	if config.AuthType == "PSK" {
		glog.V(2).Infof("Starting Trireme PSK")

		// Starting PSK Trireme
		trireme, monitor, _ = configurator.NewPSKHybridTriremeWithMonitor(config.KubeNodeName,
			config.TriremeNets,
			kubernetesPolicy,
			nil,
			nil,
			config.ExistingContainerSync,
			[]byte(config.TriremePSK),
			nil,
			false)
	}

	if config.AuthType == "PKI" {
		glog.V(2).Infof("Starting Trireme PKI")

		// Load the PKI Certs/Keys based on config.
		pki, err := auth.LoadPKI(config.PKIDirectory)
		if err != nil {
			fmt.Printf("Error loading Certificates for PKI Trireme, exiting: %s \n", err)
			return
		}

		// Starting PKI Trireme
		trireme, monitor, publicKeyAdder = configurator.NewPKITriremeWithDockerMonitor(config.KubeNodeName, kubernetesPolicy, nil, nil, config.ExistingContainerSync, pki.KeyPEM, pki.CertPEM, pki.CaCertPEM, nil, config.RemoteEnforcer, false)

		// Sync the Trireme certs over all the Kubernetes Cluster. Annotations on the
		// node object are used to hold those certs.
		// 1) Adds the localCert on the localNode annotation
		// 2) Sync All the Certs from the other nodes to the CertCache (interface)
		// 3) Waits and listen for new nodes coming up.
		certs := auth.NewCertsWatcher(*kubernetesPolicy.KubernetesClient, publicKeyAdder, config.NodeAnnotationKey)
		certs.AddCertToNodeAnnotation(*kubernetesPolicy.KubernetesClient, pki.CertPEM)
		certs.SyncNodeCerts(*kubernetesPolicy.KubernetesClient)
		go certs.StartWatchingCerts()

	}

	// Register Trireme to the Kubernetes policy resolver
	kubernetesPolicy.SetPolicyUpdater(trireme)

	// Start all the go routines.
	trireme.Start()
	monitor.Start()
	kubernetesPolicy.Run()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	// Waiting for a Sig
	<-c

	fmt.Println("Bye Kubernetes!")
	kubernetesPolicy.Stop()
	monitor.Stop()
	trireme.Stop()

}
