#! /bin/bash

PASS=$(openssl rand -base64 16)
NAME="ca"
SECRET_NAME="trireme-cacert"
TRIREME_CA_PASS_SECRET_ENTRY="ca-pass"

echo "Attempting to generate PKI"
tg cert --is-ca --auth-client --auth-server --pass $PASS --name $NAME
kubectl --namespace kube-system create secret generic $SECRET_NAME --from-file=$NAME-cert.pem --from-file=$NAME-key.pem --from-literal=$TRIREME_CA_PASS_SECRET_ENTRY=$PASS