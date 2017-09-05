package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/aporeto-inc/trireme-kubernetes/auth"
	"github.com/aporeto-inc/trireme-kubernetes/config"
	"github.com/aporeto-inc/trireme-kubernetes/resolver"
	"github.com/aporeto-inc/trireme-kubernetes/version"

	"github.com/aporeto-inc/trireme"
	"github.com/aporeto-inc/trireme/cmd/remoteenforcer"
	"github.com/aporeto-inc/trireme/configurator"
	"github.com/aporeto-inc/trireme/enforcer"
	"github.com/aporeto-inc/trireme/enforcer/utils/tokens"
	"github.com/aporeto-inc/trireme/monitor"

	"go.uber.org/zap"
)

func banner(version, revision string) {
	fmt.Printf(`


	  _____     _
	 |_   _| __(_)_ __ ___ _ __ ___   ___
	   | || '__| | '__/ _ \ '_'' _ \ / _ \
	   | || |  | | | |  __/ | | | | |  __/
	   |_||_|  |_|_|  \___|_| |_| |_|\___|


_______________________________________________________________
             %s - %s
                                                 🚀  by Aporeto

`, version, revision)
}

func main() {

	config, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Error loading config: %s", err)
	}

	if !config.Enforce {
		banner(version.VERSION, version.REVISION)
	} else {
		config.LogLevel = "debug"
	}

	err = setLogs(config.LogLevel)
	if err != nil {
		log.Fatalf("Error setting up logs: %s", err)
	}

	zap.L().Debug("Config used", zap.Any("Config", config))

	if config.Enforce {
		zap.L().Info("Launching in enforcer mode")
		remoteenforcer.LaunchRemoteEnforcer(nil)
		return
	}

	// Create New PolicyEngine based on Kubernetes rules.
	kubernetesPolicy, err := resolver.NewKubernetesPolicy(config.KubeconfigPath, config.KubeNodeName, config.ParsedTriremeNetworks, config.BetaNetPolicies)
	if err != nil {
		zap.L().Fatal("Error initializing KubernetesPolicy: ", zap.Error(err))
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
		zap.L().Info("Initializing Trireme with PSK Auth")

		// Starting PSK Trireme
		trireme, monitor, _ = configurator.NewPSKHybridTriremeWithMonitor(config.KubeNodeName,
			config.ParsedTriremeNetworks,
			kubernetesPolicy,
			nil,
			nil,
			true,
			[]byte(config.PSK),
			nil,
			false)
	}

	if config.AuthType == "PKI" {
		zap.L().Info("Initializing Trireme with PKI Auth")

		// Load the PKI Certs/Keys based on config.
		pki, err := auth.LoadPKI(config.PKIDirectory)
		if err != nil {
			zap.L().Fatal("Error loading Certificates for PKI Trireme", zap.Error(err))
		}

		// Starting PKI Trireme
		trireme, monitor, publicKeyAdder = configurator.NewPKITriremeWithDockerMonitor(config.KubeNodeName,
			kubernetesPolicy,
			nil,
			nil,
			true,
			pki.KeyPEM,
			pki.CertPEM,
			pki.CaCertPEM,
			nil,
			config.RemoteEnforcer,
			false)

		// Sync the Trireme certs over all the Kubernetes Cluster. Annotations on the
		// node object are used to hold those certs.
		// 1) Adds the localCert on the localNode annotation
		// 2) Sync All the Certs from the other nodes to the CertCache (interface)
		// 3) Waits and listen for new nodes coming up.
		certs := auth.NewCertsWatcher(*kubernetesPolicy.KubernetesClient, publicKeyAdder, "abcd")
		certs.AddCertToNodeAnnotation(*kubernetesPolicy.KubernetesClient, pki.CertPEM)
		certs.SyncNodeCerts(*kubernetesPolicy.KubernetesClient)
		go certs.StartWatchingCerts()

	}

	// Register Trireme to the Kubernetes policy resolver
	kubernetesPolicy.SetPolicyUpdater(trireme)

	// Start all the go routines.
	trireme.Start()
	zap.L().Debug("Trireme started")
	monitor.Start()
	zap.L().Debug("Monitor started")
	kubernetesPolicy.Run()
	zap.L().Debug("PolicyResolver started")

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	zap.L().Info("Everything started. Waiting for Stop signal")
	// Waiting for a Sig
	<-c

	zap.L().Debug("Stop signal received")
	kubernetesPolicy.Stop()
	zap.L().Debug("KubernetesPolicy stopped")
	monitor.Stop()
	zap.L().Debug("Monitor stopped")
	trireme.Stop()
	zap.L().Debug("Trireme stopped")

	zap.L().Info("Everything stopped. Bye Kubernetes!")
}

// setLogs setups Zap to
func setLogs(logLevel string) error {
	zapConfig := zap.NewDevelopmentConfig()
	zapConfig.DisableStacktrace = true

	// Set the logger
	switch logLevel {
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
		return err
	}

	zap.ReplaceGlobals(logger)
	return nil
}
