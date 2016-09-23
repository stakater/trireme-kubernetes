# kubernetes-integration

Aporeto integration with Kubernetes

## How to use it ?

Launch the agent on your Kubernetes-Node to start policing containers that comes up.
As for now, all the containers that are already present on the platform are fully allowed.

## Kubernetes API connection
As of today, the agent reads the .kubectl file in your home directory, and will use the default context/cluster to connect.

## What does it police on ?

It polices based on the provisioned Kubernetes-policies.
If you don't have any policies provisioned, your newly activated pods will be unreachable.

## How can I try it ?

build everything (you will need K8S whole repo), and launch the agent (in debug mode for more info):

```
go build
./kubernetes-integration --logtostderr=1 --v=6
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
