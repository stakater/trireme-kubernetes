# Aporeto Trireme on Red Hat OpenShift Deployment Guide

## Introduction
Welcome to the _Aporeto Trireme on Red Hat OpenShift Deployment Guide._  This guide will briefly describe the integration and will walk through each step of installing Aporeto Trireme on Red Hat OpenShift.

### Aporeto Trireme
Trireme is a process that can run as a standard Linux process but is commonly run as a Docker container.  Anywhere you can run Docker, you can run Trireme!  Trireme runs in Docker as a privileged container that adds an enforcer to the network stack of the host.  Trireme allows for network segmentation of containers within a Docker host and between multiple Docker hosts.

_**NOTE:** Currently, only enforcement of TCP connections are supported._

More information about Aporeto Trireme can be found at the [Trireme GitHub repository](https://github.com/aporeto-inc/trireme).

#### Network segmentation control
Trireme uses attributes in the form of key-value pairs along with user-defined policy logic to determine if two containers in the cluster may establish a connection.

Via the TCP 3-way handshake between containers on Docker nodes that are fronted by Trireme, Trireme transparently transfers these attributes and references associated policy to determine if the TCP connection should proceed or be dropped.

### Aporeto trireme-kubernetes
trireme-kuberenetes is an implementation of Trireme that runs on Google Kubernetes v1.3+ and provides enforcement of the Kubernetes networkpolicy resource.  More information on the Kubernetes networkpolicy resource can be found in the [Kubernetes Reference Documentation](https://kubernetes.io/docs/user-guide/networkpolicies/). 

trireme-kuberenetes adds to Trireme the capability to use Kubernetes labels (such as Roles) and the Kubernetes networkpolicy resource.  With this functionality, administrators no longer need to create attributes and policy for network segmentation of pods.  trireme-kubernetes extends the domain of Kubernetes automatically-enforced intended state with network segmentation.

trireme-kubernetes runs as a Kubernetes daemonset, creating a pod on each Kubernetes node.

More information about Aporeto trireme-kubernetes can be found at the [trireme-kubernetes GitHub repository](https://github.com/aporeto-inc/trireme-kubernetes).

### trireme-kubernetes and OpenShift 
Since OpenShift uses Kubernetes for container orchestation, you can add Trireme functionality to OpenShift.  By defining networkpolicy and adding appropriate labels to your containers, developers can continue to deploy application conatiners on OpenShift with automated network segmentation.

## Prerequisites
_**NOTE:** The example output in this guide is from an Aporeto lab configuration, using a fresh install of OpenShift via "oc cluster up" with default settings._

Since the trireme-kubernetes container(s) runs in privileged mode, the OpenShift host must be configured to allow privileged mode.  In addition, trireme-kubernetes requires access to network interfaces and storage, so the equivalent of the OpenShift cluster-admin role is required for the associated serviceaccount.  Due to these requirements, the only OpenShift deployment method supported is the OpenShift Container Platform.  OpenShift Dedicated and OpenShift Online are not supported.
 
You must have command-line access to OpenShift via _**oc**_ and _**oc adm**_.  The following steps assume system:admin access.  If you are not able to login as system:admin, consult with your OpenShift administrator to configure equivalent access.

Obtain trireme-kubernetes from Github.
```
% git clone https://github.com/aporeto-inc/trireme-kubernetes.git
Cloning into 'trireme-kubernetes'...
remote: Counting objects: 1029, done.
remote: Total 1029 (delta 0), reused 0 (delta 0), pack-reused 1028
Receiving objects: 100% (1029/1029), 241.37 KiB | 294.00 KiB/s, done.
Resolving deltas: 100% (554/554), done.
```

**Aporeto example environment - platform versions**
RHEL 7.3 with OpenShift via _oc cluster up_
``` 
% docker --version`
Docker version 1.12.6, build 96d83a5/1.12.6
```

```
% oc version
oc v1.4.0-alpha.1+f189ede
kubernetes v1.4.0+776c994
features: Basic-Auth GSSAPI Kerberos SPNEGO

Server https://192.168.88.131:8443
openshift v1.4.0-alpha.1+f189ede
kubernetes v1.4.0+776c994
``` 
## OpenShift preparatory commands
Login with admin credentials.
```
% oc login -u system:admin
Logged into "https://192.168.88.131:8443" as "system:admin" using existing credentials.

You have access to the following projects and can switch between them with 'oc project <projectname>':

    default
    kube-system
  * myproject
    openshift
    openshift-infra

Using project "myproject".
```
Change the OpenShift project scope to one in which you will run trireme-kubernetes. In this example, we use the _default_ project in OpenShift.  

_**NOTE:** If you want to use a different OpenShift project, adjust the parameters appropriately for the commands that deal with serviceaccounts and trireme secrets._
```
% oc project default
Now using project "default" on server "https://192.168.88.131:8443".
```
Add the appropriate serviceaccount to the _privileged_ security context constraint (SCC).  In this example, we use the _default_ service account (system:serviceaccount:default:default) for the _default_ project. 

**Aporeto example environment - privileged scc**
```
% oc edit scc privileged
...
users:
- system:serviceaccount:openshift-infra:build-controller
- system:serviceaccount:default:router
- system:serviceaccount:default:default
```

Verify the users for the "privileged" scc.
```
% oc describe scc privileged
Name:                                           privileged
Priority:                                       <none>
Access:
  Users:
system:serviceaccount:openshift-infra:build-controller,system:serviceaccount:default:router,system:serviceaccount:default:default
...
```
Add binding for the appropriate service account to the cluster-admin role.  In this example, we use the _default_ service account.

`% oc adm policy add-cluster-role-to-user cluster-admin system:serviceaccount:default:default`

Verify the cluster-admin role binding for the appropriate service account.

**Aporeto example environment - cluster-admin role binding verification for the _default_ service account**
```
% oc describe clusterPolicyBindings :default 
Name:                                       :default
Namespace:                                  <none>
Created:                                    About an hour ago
Labels:                                     <none>
Annotations:                                <none>
Last Modified:                              2017-03-16 13:07:21 -0700 PDT
Policy:                                     <none>
RoleBinding[basic-users]:
                                            Role:                   basic-user
                                            Users:                  <none>
                                            Groups:                 system:authenticated
                                            ServiceAccounts:        <none>
                                            Subjects:               <none>
RoleBinding[cluster-admins]:
                                            Role:                   cluster-admin
                                            Users:                  system:admin
                                            Groups:                 system:cluster-admins
                                            ServiceAccounts:        default/default
                                            Subjects:               <none>
...
```

## Trireme execution commands

_**NOTE:** This section is similar to the steps described in [trireme-kubernetes/deployment/README.md](https://github.com/aporeto-inc/trireme-kubernetes/tree/master/deployment), with some additional instructions for OpenShift.  In this example, we will utilize the easier PSK method for node authentication (vs PKI).  For the demonstration application, we will create a "demo" project.  However, trireme-kubernetes will run in the "default" project._

_**NOTE:** You should still be in the "default" OpenShift project._

In a command shell, navigate to the trireme-kubernetes/deployment/Trireme/KubeDaemonSet directory.

Edit _daemonSetPSK.yaml_ for PSK and proper TRIREME_NETS subnet value.  This value is hard-coded in the repository as 10.0.0.0/8, but you must change this to your OpenShift container subnet.  In the Aporeto lab environment (via _oc cluster up_), containers default to 172.17.0.0./16 for container IP addresses.


**Aporeto example environment - a portion of daemonSetPSK.yaml**
```
% cat daemonSetPSK.yaml
...
containers:
        -  name: trireme
           image: aporeto/trireme-kubernetes
           env:
             - name: SYNC_EXISTING_CONTAINERS
               value: "true"
             - name: TRIREME_NETS
               value: 172.17.0.0/16
             - name: TRIREME_AUTH_TYPE
               value: PSK
             - name: TRIREME_PSK
               valueFrom:
                 secretKeyRef:
                   name: trireme
                   key: triremepsk
...
```

Create and verify the trireme secret.
```
% sh createPSK.sh
Attempting to generate PSK
secret "trireme" created
% oc get secrets
NAME                       TYPE                                  DATA      AGE
builder-dockercfg-obib3    kubernetes.io/dockercfg               1         1h
builder-token-au3nw        kubernetes.io/service-account-token   4         1h
builder-token-s454w        kubernetes.io/service-account-token   4         1h
default-dockercfg-m12x8    kubernetes.io/dockercfg               1         1h
default-token-hmu84        kubernetes.io/service-account-token   4         1h
default-token-zjqkh        kubernetes.io/service-account-token   4         1h
deployer-dockercfg-e77ts   kubernetes.io/dockercfg               1         1h
deployer-token-dfhra       kubernetes.io/service-account-token   4         1h
deployer-token-o74u4       kubernetes.io/service-account-token   4         1h
registry-dockercfg-i72vx   kubernetes.io/dockercfg               1         1h
registry-token-eqarq       kubernetes.io/service-account-token   4         1h
registry-token-r90bq       kubernetes.io/service-account-token   4         1h
router-certs               kubernetes.io/tls                     2         1h
router-dockercfg-n1dh3     kubernetes.io/dockercfg               1         1h
router-token-e0ry3         kubernetes.io/service-account-token   4         1h
router-token-flh98         kubernetes.io/service-account-token   4         1h
trireme                    Opaque                                1         9s
```

Create and verify the trireme-kubernetes daemonset.
```
% oc create -f daemonSetPSK.yaml
daemonset "trireme" created
% oc get daemonset
NAME      DESIRED   CURRENT   NODE-SELECTOR   AGE
trireme   1         1         <none>          2m
% oc get pods
NAME                      READY     STATUS    RESTARTS   AGE
docker-registry-1-objnf   1/1       Running   231        59m
router-1-gbf5j            1/1       Running   227        59m
trireme-4ejgm             1/1       Running   5          3m
```

_**WARNING:** Improper permission configuration for the equivalent of the cluster-admin role binding will cause the OpenShift docker-registry and router pods to fail.  Improper privileged container creation access will prevent the creation of the trireme pod._
 
Things to look out for:
- daemonSet yaml: TRIREME_NETS proper value
- Proper project/namespace scoping for secrets and where you run pods

## Sample application for demonstrating network segmentation

_**NOTE:** This section is similar to the demo example steps in the [trireme-kubernetes GitHub repository](https://github.com/aporeto-inc/trireme-kubernetes)._

In a command shell, navigate to the trireme-kubernetes/deployment/PolicyExample directory.

Create the _demo_ namespace with _DefaultDeny_ networkpolicy.
```
% oc create -f DemoNamespace.yaml
namespace "demo" created
```
Create and verify the sample networkpolicy resource for Kubernetes in the _demo_ namespace.  In this example, our _backend-policy_ permits ingress traffic to pods with the _BusinessBackend_ role from pods with the _WebFrontend_ and _BusinessBackend_ roles.  Any other pods may not access the _BusinessBackend_ pods.
```
% oc create -f Demo3TierPolicy.yaml
networkpolicy "frontend-policy" created
networkpolicy "backend-policy" created
networkpolicy "database-policy" created
% oc get networkpolicy --namespace=demo
NAME              POD-SELECTOR           AGE
backend-policy    role=BusinessBackend   1m
database-policy   role=Database          1m
frontend-policy   role=WebFrontend       1m
```
Create the sample _External_, _WebFrontend_, and _BusinessBackend_ pods in the _demo_ namespace.
```
% oc create -f Demo3TierPods.yaml
pod "external" created
pod "frontend" created
pod "backend" created
% oc get pods --namespace=demo
NAME       READY     STATUS    RESTARTS   AGE
backend    1/1       Running   0          12s
external   1/1       Running   0          12s
frontend   1/1       Running   0          12s
```
Obtain the IP addresses for the demonstration pods.
```
% oc describe pods --namespace=demo | grep '^Name:\|IP'
Name:                   backend
IP:                     172.17.0.3
Name:                   external
IP:                     172.17.0.2
Name:                   frontend
IP:                     172.17.0.5
```
Verify connectivity from _WebFrontend_ to _BusinessBackend_ via _wget_.
```
% oc exec --namespace=demo -it frontend /bin/bash
root@frontend:/data# wget http://172.17.0.3
converted 'http://172.17.0.3' (ANSI_X3.4-1968) -> 'http://172.17.0.3' (UTF-8)
--2017-03-16 22:12:11--  http://172.17.0.3/
Connecting to 172.17.0.3:80... connected.
HTTP request sent, awaiting response... 200 OK
Length: 612 [text/html]
Saving to: 'index.html'

index.html          100%[=====================>]     612  --.-KB/s   in 0s

2017-03-16 22:12:11 (191 MB/s) - 'index.html' saved [612/612]

root@frontend:/data# exit
exit
```
Verify that connectivity cannot be established between _External_ and _BusinessBackend_ via _wget_.
```
% oc exec --namespace=demo -it external /bin/bash
root@external:/data# wget http://172.17.0.3
converted 'http://172.17.0.3' (ANSI_X3.4-1968) -> 'http://172.17.0.3' (UTF-8)
--2017-03-16 22:14:55--  http://172.17.0.3/
Connecting to 172.17.0.3:80... ^C
root@external:/data# exit
```
Try modifying and resubmitting the Demo3TierPolicy.yaml file to allow _External_ to access _BusinessBackend_.

Thank you for evaluating Aporeto Trireme on Red Hat OpenShift!  For more information about how Aporeto can help secure your cloud native application, visit us at https://www.aporeto.com/ .   
