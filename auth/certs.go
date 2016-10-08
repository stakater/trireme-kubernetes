package auth

import (
	"os"

	"github.com/aporeto-inc/kubernetes-integration/kubernetes"
)

// EnvNodeName is the default env. name used for the Kubernetes node name.
const EnvNodeName = "KUBERNETES_NODE"

// NodeAnnotationKey is the env variable used as a key for the annotation containing the
// node cert.
const NodeAnnotationKey = "TRIREME_CERT"

// RegisterPKI registers the Cert of this node as an annotation on the KubeAPI.
func RegisterPKI(client kubernetes.Client, cert []byte) {
	nodeName := os.Getenv(EnvNodeName)
	client.AddNodeAnnotation(nodeName, NodeAnnotationKey, certToString(cert))
}

func certToString(cert []byte) string {
	return string(cert)
}
