package auth

import (
	"fmt"
	"io/ioutil"
)

// KeyPEMFile is the name of the KeyPEMFile in the SecretDirectory directory.
const KeyPEMFile = "key.pem"

// CertPEMFile is the name of the CertPEsMFile in the SecretDirectory directory.
const CertPEMFile = "cert.pem"

// CaCertPEMFile is the name of the CaCertPEMFile in the SecretDirectory directory.
const CaCertPEMFile = "ca.crt"

// A PKI is used to
type PKI struct {
	KeyPEM    []byte
	CertPEM   []byte
	CaCertPEM []byte
}

// LoadPKI Create a new PKISecret from Kube Secret.
func LoadPKI(dir string) (*PKI, error) {
	keyPEM, err := ioutil.ReadFile(dir + KeyPEMFile)
	if err != nil {
		return nil, fmt.Errorf("Couldn't read KeyPEMFile: %s", err)
	}
	certPEM, err := ioutil.ReadFile(dir + CertPEMFile)
	if err != nil {
		return nil, fmt.Errorf("Couldn't read CertPEMFile: %s", err)
	}
	caCertPEM, err := ioutil.ReadFile(dir + CaCertPEMFile)
	if err != nil {
		return nil, fmt.Errorf("Couldn't read CaCertPEMFile %s", err)
	}

	return &PKI{
		KeyPEM:    keyPEM,
		CertPEM:   certPEM,
		CaCertPEM: caCertPEM,
	}, nil
}
