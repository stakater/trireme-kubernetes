# trireme-kubernetes

Trireme integration with Kubernetes

## How to use it ?

Launch the agent on your Kubernetes-Node to implement the network policy API for PODS.
As for now, all the PODS that are already present on the platform are fully allowed.

## Kubernetes API connection
As of today, the agent reads the .kubectl file in your home directory, and will use the default context/cluster to connect.

## What policies does it implement ?

It implements the complete set  provisioned Kubernetes-policies. The Trireme code allows for additional
controls, that are currently not supported in the Kubernetes API. 
If you don't have any policies provisioned, your newly activated pods will be unreachable.

## How can I try it ?

build everything (you will need K8S whole repo), and launch the agent (in debug mode for more info):

```
go build
./trireme-kubernetes --logtostderr=1 --v=6
```

Provision example policies and example pods:

```
kubectl create -f deployment/example/yaml/3TierPolicies.yaml
kubectl create -f deployment/example/yaml/3TierPods.yaml
```

Try to  connect to your pods from other pods:

```
External --> Backend: Forbidden
Frontend --> Backend: Allowed
```
