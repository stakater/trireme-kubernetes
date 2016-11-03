#!/bin/bash

echo "This script generates a simple PKI valid for one single node."
echo "If you wish to run on multiple workers, you will need to manage and generate a set of keys under a centralized CA."
echo ""

if [ "$1" == "--skip-defaults" ]; then
    default_subj=
else
    default_subj="-subj /C=US/ST=CA/L=SJC/O=Trireme/CN=$HOSTNAME"
fi

# Create CA
openssl ecparam -name prime256v1 -genkey -noout -out ca.key

# Create CA cert
openssl req -x509 -new -SHA256 -nodes -key ca.key -days 3650 -out ca.crt $default_subj

# Create key
openssl ecparam -name prime256v1 -genkey -noout -out key.pem

# Create sign request
openssl req -new -SHA256 -key key.pem -nodes -out cert.csr $default_subj

# Sign the key
openssl x509 -req -SHA256 -days 3650 -in cert.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out cert.pem

mkdir /var/trireme
mv ca.crt key.pem cert.pem /var/trireme

# Cleanup
rm -rf ca.* cert.* key.*
