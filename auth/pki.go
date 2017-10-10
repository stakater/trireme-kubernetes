package auth

import (
	"fmt"
	"time"

	"github.com/aporeto-inc/trireme-csr/certificates"
	certificateclient "github.com/aporeto-inc/trireme-csr/client"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// TriremePKI contains all the keys and cert for the local Trireme node.
type TriremePKI struct {
	KeyPEM    []byte
	CertPEM   []byte
	CaCertPEM []byte
}

// LoadPKI issue a CSR to Trireme-CSR and returns all the
func LoadPKI(nodeName string, kubeconfigPath string) (*TriremePKI, error) {

	// Get the Kube API interface for Certificates up
	kubeconfig, err := buildConfig(kubeconfigPath)
	if err != nil {
		panic("Error generating Kubeconfig " + err.Error())
	}

	certClient, _, err := certificateclient.NewClient(kubeconfig)
	if err != nil {
		panic("Error creating REST Kube Client for certificates: " + err.Error())
	}

	certManager, err := certificates.NewCertManager(nodeName, certClient)

	err = certManager.GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("Error generating privateKey %s", err)
	}

	err = certManager.GenerateCSR()
	if err != nil {
		return nil, fmt.Errorf("Error generating CSR %s", err)
	}

	err = certManager.SendAndWaitforCert(time.Minute)
	if err != nil {
		return nil, fmt.Errorf("Error Sending and waiting %s", err)
	}

	keyPEM, err := certManager.GetKeyPEM()
	if err != nil {
		return nil, fmt.Errorf("Error Getting Key PEM %s", err)
	}

	certPEM, err := certManager.GetCertPEM()
	if err != nil {
		return nil, fmt.Errorf("Error Getting cert PEM %s", err)
	}

	caCertPEM, err := certManager.GetCaCertPEM()
	if err != nil {
		return nil, fmt.Errorf("Error Getting cert PEM %s", err)
	}

	return &TriremePKI{
		KeyPEM:    keyPEM,
		CertPEM:   certPEM,
		CaCertPEM: caCertPEM,
	}, nil
}

func buildConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}
