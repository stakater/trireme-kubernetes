# How to deploy Kubernetes Integration with Trireme ?

## Deploy it directly on top of Kubernetes
You have multiple choice. The most obvious is to deploy it as a Kubernetes pod itself. By using a DaemonSet, Kubernetes will ensure that there is exactly one pod running per Kubernetes node.
The requirement to deploy it directly on top of Kubernetes, is that your Kubernetes Cluster must accept privileged containers and be able to perform an "InCluster authentication".

To deploy the Kubernetes DaemonSet, simply create the DaemonSet from the file:
```
kubectl create -f kubernetes/daemonSet.yaml
```

In order to have Kubernetes allow pods with Containers in privileged mode, the following parameter must be given to the KubeAPI and Kubelet: `--allow-privileged`


## Deploy it with Docker

Another solution is to directly launch it with Docker API. This needs to be done once per node. The deplyment can be done easily by running the `docker.sh` script

## Deploy it as a standalone process
