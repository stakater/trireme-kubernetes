package config

import (
	"flag"
	"fmt"
	"os"
)

// EnvNodeName is the default env. name used for the Kubernetes node name.
const EnvNodeName = "KUBERNETES_NODE"

// EnvNodeAnnotationKey is the env variable used as a key for the annotation containing the
// node cert.
const EnvNodeAnnotationKey = "TRIREME_CERT"

// DefaultNodeAnnotationKey is the env variable used as a key for the annotation containing the
// node cert.
const DefaultNodeAnnotationKey = "TRIREME"

// EnvPKIDirectory is the env. variable name for the location of the directory where
// the PKI files are expected to be found.
const EnvPKIDirectory = "TRIREME_PKI"

// DefaultPKIDirectory is the directory where the PEMs are mounted.
const DefaultPKIDirectory = "/var/trireme/"

// DefaultTriremePSK is used as the default PSK for trireme if not overriden by the user.
const DefaultTriremePSK = "Trireme"

// KubeConfigLocation is the default location of the KubeConfig file.
const KubeConfigLocation = "/.kube/config"

// EnvSyncExistingContainers is the env variable that will define if you sync the existing containers.
const EnvSyncExistingContainers = "SYNC_EXISTING_CONTAINERS"

// DefaultSyncExistingContainers is the default value if you need to sync all existing containers.
const DefaultSyncExistingContainers = true

// TriKubeConfig maintains the Configuration of Kubernetes Integration
type TriKubeConfig struct {
	KubeEnv               bool
	KubeNodeName          string
	NodeAnnotationKey     string
	PKIDirectory          string
	KubeConfigLocation    string
	TriremePSK            string
	ExistingContainerSync bool
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: example -stderrthreshold=[INFO|WARN|FATAL] -log_dir=[string]\n")
	flag.PrintDefaults()
	os.Exit(2)
}

// LoadConfig loads config:
// 1) If presents flags are used
// 2) If no flags, Env Variables are used
// 3) If no Env Variables, defaults are used when possible.
func LoadConfig() *TriKubeConfig {

	var flagNodeName = flag.String("node", "", "Node name in Kubernetes")
	var flagNodeAnnotationKey = flag.String("annotation", "", "Trireme Node Annotation key in Kubernetes")
	var flagPKIDirectory = flag.String("pki", "", "Directory where the Trireme PKIs are")
	var flagKubeConfigLocation = flag.String("kubeconfig", "", "KubeConfig used to connect to Kubernetes")
	var flagtSyncExistingContainers = flag.Bool("syncexisting", true, "Sync existing containers")

	flag.Usage = usage
	flag.Parse()

	config := &TriKubeConfig{}

	if os.Getenv("KUBERNETES_PORT") == "" {
		config.KubeEnv = false
		config.KubeConfigLocation = *flagKubeConfigLocation
		if config.KubeConfigLocation == "" {
			config.KubeConfigLocation = os.Getenv("HOME") + KubeConfigLocation
		}
	} else {
		config.KubeEnv = true
		config.KubeConfigLocation = ""
	}

	config.KubeNodeName = *flagNodeName
	if config.KubeNodeName == "" {
		config.KubeNodeName = os.Getenv(EnvNodeName)
	}
	if config.KubeNodeName == "" {
		panic("Couldn't load NodeName")
	}

	config.NodeAnnotationKey = *flagNodeAnnotationKey
	if config.NodeAnnotationKey == "" {
		config.NodeAnnotationKey = os.Getenv(EnvNodeAnnotationKey)
	}
	if config.NodeAnnotationKey == "" {
		config.NodeAnnotationKey = DefaultNodeAnnotationKey
	}

	config.PKIDirectory = *flagPKIDirectory
	if config.PKIDirectory == "" {
		config.PKIDirectory = os.Getenv(EnvPKIDirectory)
	}
	if config.PKIDirectory == "" {
		config.PKIDirectory = DefaultPKIDirectory
	}

	config.TriremePSK = DefaultTriremePSK

	if os.Getenv(EnvSyncExistingContainers) == "" {
		config.ExistingContainerSync = *flagtSyncExistingContainers
	}
	if os.Getenv(EnvSyncExistingContainers) == "true" {
		config.ExistingContainerSync = true
	}
	if os.Getenv(EnvSyncExistingContainers) == "false" {
		config.ExistingContainerSync = false
	}

	return config
}
