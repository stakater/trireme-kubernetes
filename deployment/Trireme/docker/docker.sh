SYNC_EXISTING_CONTAINERS="true"
TRIREME_AUTH_TYPE="PSK"
TRIREME_PSK="defaultPSKkey"
KUBERNETES_NODE="127.0.0.1"
TRIREME_PKI_MOUNT="/var/trireme/"
TRIREME_CERT_ANNOTATION="TRIREME"
TRIREME_NETS="10.0.0.0/8"


docker run \
  --name "Trireme" \
  --privileged \
  --net host \
  -t \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /root/.kube/config:/root/.kube/config \
  -e SYNC_EXISTING_CONTAINERS=$SYNC_EXISTING_CONTAINERS
  -e TRIREME_AUTH_TYPE=$TRIREME_AUTH_TYPE
  -e TRIREME_PSK=$TRIREME_PSK
  -e KUBERNETES_NODE=$KUBERNETES_NODE
  -e TRIREME_PKI_MOUNT=$TRIREME_PKI_MOUNT
  -e TRIREME_CERT_ANNOTATION=$TRIREME_CERT_ANNOTATION
  -e TRIREME_NETS=$TRIREME_NETS
  --restart always \
aporeto/trireme-kubernetes
