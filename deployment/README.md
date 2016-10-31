# How to deploy Kubernetes Integration with Trireme ?

## InCluster deployment with DaemonSets.

This is the recommended deployment method. The Kubernetes `daemonSet` ensures that one agent (as a system pod) runs on every node part of the cluster. As the Trireme agent runs as `privileged: true` and into the Host network, your cluster must accept to run privileged containers.
The [provided ](https://github.com/aporeto-inc/trireme) `daemonSet` yaml  should almost work out of the box.
Two version of the daemonSet are provided. One for using PSK (simpler) and one for PKI (require certificate generation, but recommended).
The following env variables need to be adapted out of the provided daemonSet:

* For the PSK Version:
```yaml

env:
  - name: SYNC_EXISTING_CONTAINERS
    value: "true"
  - name: TRIREME_AUTH_TYPE
    value: PSK
  - name: TRIREME_PSK
    valueFrom:
      secretKeyRef:
        name: mysecret
        key: username
  - name: KUBERNETES_NODE
    valueFrom:
      fieldRef:
        fieldPath: spec.host
```

* For the PKI Version:

```yaml
env:
  - name: SYNC_EXISTING_CONTAINERS
    value: "true"
  - name: TRIREME_AUTH_TYPE
    value: PKI
  - name: TRIREME_PKI_MOUNT
    value: /var/trireme/
  - name: TRIREME_CERT_ANNOTATION
    value: TRIREME
  - name: KUBERNETES_NODE
    valueFrom:
      fieldRef:
        fieldPath: spec.host
```



* `SYNC_EXISTING_CONTAINERS` is `true` by default. Defines if already running pods also need to be policed.
* `TRIREME_AUTH_TYPE` is `PKI` by default. Can also be `PSK`.
* `TRIREME_PKI_MOUNT` is only needed if TRIREME_AUTH_TYPE is `PKI`. It defines where the certificates and private key are mounted on the system.
* `TRIREME_PSK` is only needed if TRIREME_AUTH_TYPE is `PSK`. It defines the shared password used for node authentication.
* `TRIREME_CERT_ANNOTATION` defines which key is used for the node certificate.
* `KUBERNETES_NODE` defines the local node name on which the agent runs. When running as a DaemonSet, this value should be filled-in automatically.

Typically, the only values that you should have to change are the ones related to authentication. By default the daemonSet will  mount the local `/var/trireme/` on each local node (if choosing PKI).

some helpers are also provided:
* For PSK: createPSK.sh assists with the creation of the PSK and Kubernetes secret provisioning.
* For PKI: createPKI.sh assists with the generation of certificate and moves them to `/var/trireme`. This needs to be performed with the same CA on each node part of the cluster. This script is only an example to get started quickly.

To deloy the daemonSet:

```
kubectl create -f
```

## Docker deployment outside Kubernetes.

Another popular way to deploy the agent is to use Docker directly. You would typically use this if privileged containers are blocked on your cluster.
An helper script is provided here. The environment variables are the same as the ones defined for Kubernetes.
When running the agent directly on docker, you need to keep track of your deployment accross your whole Kubernetes cluster.

## deployment as a daemon/process directly on the host.

Finally, another option is to directly launch the binary on the host.
a helper script is provided here with the same configuration variable as in Kubernetes.
