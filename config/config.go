package config

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/golang/glog"
)

// EnvTrue is the string for true
const EnvTrue = "true"

// EnvFalse is the string for false
const EnvFalse = "false"

// EnvNodeName is the default env. name used for the Kubernetes node name.
const EnvNodeName = "KUBERNETES_NODE"

// EnvNodeAnnotationKey is the env variable used as a key for the annotation containing the
// node cert.
const EnvNodeAnnotationKey = "TRIREME_CERT_ANNOTATION"

// DefaultNodeAnnotationKey is the env variable used as a key for the annotation containing the
// node cert.
const DefaultNodeAnnotationKey = "TRIREME"

// EnvAuthType is used as the Auth Type.
const EnvAuthType = "TRIREME_AUTH_TYPE"

// DefaultAuthType is the env variable used as a key for the annotation containing the
// node cert.
const DefaultAuthType = "PSK"

// EnvPKIDirectory is the env. variable name for the location of the directory where
// the PKI files are expected to be found.
const EnvPKIDirectory = "TRIREME_PKI_MOUNT"

// DefaultPKIDirectory is the directory where the PEMs are mounted.
const DefaultPKIDirectory = "/var/trireme/"

// EnvTriremePSK is used as the default PSK for trireme if not overriden by the user.
const EnvTriremePSK = "TRIREME_PSK"

// DefaultTriremePSK is used as the default PSK for trireme if not overriden by the user.
const DefaultTriremePSK = "Trireme"

// DefaultKubeConfigLocation is the default location of the KubeConfig file.
const DefaultKubeConfigLocation = "/.kube/config"

// EnvSyncExistingContainers is the env variable that will define if you sync the existing containers.
const EnvSyncExistingContainers = "SYNC_EXISTING_CONTAINERS"

// DefaultSyncExistingContainers is the default value if you need to sync all existing containers.
const DefaultSyncExistingContainers = true

// EnvTriremeNets is the env. variable that will contain the value for the Trireme Networks.
const EnvTriremeNets = "TRIREME_NETS"

// DefaultTriremeNets is the default Kubernetes Network subnet.
const DefaultTriremeNets = "10.0.0.0/8"

// EnvTriremeEnforcer is the env. variable that will contain the value for the Trireme Enforcer
const EnvTriremeEnforcer = "ENFORCER"

// DefaultTriremeEnforcer is the default Kubernetes Enforcer status.
const DefaultTriremeEnforcer = false

// RemoteEnforcer is the env. variable that will contain the value for the Remote Trireme Enforcer
const RemoteEnforcer = "REMOTE_ENFORCER"

// DefaultRemoteEnforcer is the default Kubernetes Remote Enforcer status.
const DefaultRemoteEnforcer = true

// TriKubeConfig maintains the Configuration of Kubernetes Integration
type TriKubeConfig struct {
	KubeEnv               bool
	AuthType              string
	KubeNodeName          string
	NodeAnnotationKey     string
	PKIDirectory          string
	KubeConfigLocation    string
	TriremePSK            string
	TriremeNets           []string
	ExistingContainerSync bool
	Enforcer              bool
	RemoteEnforcer        bool
	LogLevel              string
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
	var flagAuthType = flag.String("auth", "", "Authentication type: PKI/PSK")
	var flagPKIDirectory = flag.String("pki", "", "Directory where the Trireme PKIs are")
	var flagPSK = flag.String("psk", "", "PSK to use")
	var flagKubeConfigLocation = flag.String("kubeconfig", "", "KubeConfig used to connect to Kubernetes")
	var flagtSyncExistingContainers = flag.Bool("syncexisting", true, "Sync existing containers")
	var flagTriremeNets = flag.String("triremenets", "", "Subnets with Trireme endpoints.")
	var flagEnforcer = flag.Bool("enforcer", false, "Use the Trireme Enforcer.")
	var flagRemoteEnforcer = flag.Bool("remote", true, "Use the Trireme Remote Enforcer.")

	flag.Usage = usage
	flag.Parse()

	config := &TriKubeConfig{}

	// TODO: Based on the V Level
	config.LogLevel = "debug"

	if os.Getenv(EnvTriremeEnforcer) == "" {
		config.Enforcer = *flagEnforcer
	}
	if os.Getenv(EnvTriremeEnforcer) == EnvTrue {
		config.Enforcer = true
	}
	if os.Getenv(EnvTriremeEnforcer) == EnvFalse {
		config.Enforcer = false
	}

	// TODO: Remove once we move to DocOps
	if len(os.Args) >= 2 {
		if os.Args[1] == "enforce" {
			config.Enforcer = true
		}
	}

	if config.Enforcer {
		return config
	}

	if os.Getenv("KUBERNETES_PORT") == "" {
		config.KubeEnv = false
		config.KubeConfigLocation = *flagKubeConfigLocation
		if config.KubeConfigLocation == "" {
			config.KubeConfigLocation = os.Getenv("HOME") + DefaultKubeConfigLocation
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
		panic("Couldn't load NodeName. Ensure Kubernetes Nodename is given as a parameter ( -node) if not running in a KubernetesCluster ")
	}

	config.AuthType = *flagAuthType
	if config.AuthType == "" {
		config.AuthType = os.Getenv(EnvNodeAnnotationKey)
	}
	if config.AuthType == "" {
		config.AuthType = DefaultAuthType
	}
	if config.AuthType != "PSK" && config.AuthType != "PKI" {
		config.AuthType = DefaultAuthType
	}

	// If PSK,load the PSK.
	if config.AuthType == "PSK" {
		config.TriremePSK = *flagPSK
		if config.TriremePSK == "" {
			config.TriremePSK = os.Getenv(EnvTriremePSK)
		}
		if config.TriremePSK == "" {
			config.TriremePSK = DefaultTriremePSK
		}
	}

	// Is PKI, Load the Certs and The annotation key.
	if config.AuthType == "PKI" {
		config.PKIDirectory = *flagPKIDirectory
		if config.PKIDirectory == "" {
			config.PKIDirectory = os.Getenv(EnvPKIDirectory)
		}
		if config.PKIDirectory == "" {
			config.PKIDirectory = DefaultPKIDirectory
		}

		config.NodeAnnotationKey = *flagNodeAnnotationKey
		if config.NodeAnnotationKey == "" {
			config.NodeAnnotationKey = os.Getenv(EnvNodeAnnotationKey)
		}
		if config.NodeAnnotationKey == "" {
			config.NodeAnnotationKey = DefaultNodeAnnotationKey
		}
	}

	if os.Getenv(EnvSyncExistingContainers) == "" {
		config.ExistingContainerSync = *flagtSyncExistingContainers
	}
	if os.Getenv(EnvSyncExistingContainers) == EnvTrue {
		config.ExistingContainerSync = true
	}
	if os.Getenv(EnvSyncExistingContainers) == EnvFalse {
		config.ExistingContainerSync = false
	}

	if os.Getenv(RemoteEnforcer) == "" {
		config.RemoteEnforcer = *flagRemoteEnforcer
	}
	if os.Getenv(RemoteEnforcer) == EnvTrue {
		config.RemoteEnforcer = true
	}
	if os.Getenv(RemoteEnforcer) == EnvFalse {
		config.RemoteEnforcer = false
	}

	triremeNets := *flagTriremeNets
	if triremeNets == "" {
		triremeNets = os.Getenv(EnvTriremeNets)
	}
	if triremeNets == "" {
		triremeNets = DefaultTriremeNets
	}
	parseResult, err := parseTriremeNets(triremeNets)
	if err != nil {
		glog.Fatalf("Error parsing TriremeNets: %s", err)
	}
	config.TriremeNets = parseResult
	return config
}

func parseTriremeNets(nets string) ([]string, error) {
	resultNets := strings.Fields(nets)

	// Validation of parameter networks.
	for _, network := range resultNets {
		_, _, err := net.ParseCIDR(network)
		if err != nil {
			return nil, fmt.Errorf("Invalid CIDR: %s", err)
		}
	}
	return resultNets, nil
}
