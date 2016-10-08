package auth

import "github.com/aporeto-inc/kubernetes-integration/kubernetes"

// NodeAnnotationKey is the env variable used as a key for the annotation containing the
// node cert.
const NodeAnnotationKey = "TRIREME_CERT"

// RegisterPKI registers the Cert of this node as an annotation on the KubeAPI.
func RegisterPKI(client kubernetes.Client, cert []byte) {
	client.AddLocalNodeAnnotation(NodeAnnotationKey, certToString(cert))
}

func certToString(cert []byte) string {
	return string(cert)
}
