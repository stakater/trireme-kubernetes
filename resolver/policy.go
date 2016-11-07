package resolver

import (
	"encoding/json"
	"fmt"

	"github.com/aporeto-inc/kubepox"
	"github.com/aporeto-inc/trireme"
	"github.com/aporeto-inc/trireme-kubernetes/kubernetes"

	"github.com/aporeto-inc/trireme/policy"
	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/watch"
)

// KubernetesPodName is the label used by Docker for the K8S pod name.
const KubernetesPodName = "io.kubernetes.pod.name"

// KubernetesPodNamespace is the label used by Docker for the K8S namespace.
const KubernetesPodNamespace = "io.kubernetes.pod.namespace"

// KubernetesContainerName is the label used by Docker for the K8S container name.
const KubernetesContainerName = "io.kubernetes.container.name"

// KubernetesInfraContainerName is the name of the infra POD.
const KubernetesInfraContainerName = "POD"

// KubernetesNetworkPolicyAnnotationID is the string used as an annotation key
// to define if a namespace should have the networkpolicy framework enabled.
const KubernetesNetworkPolicyAnnotationID = "net.beta.kubernetes.io/network-policy"

// KubernetesPolicy represents a Trireme Policer for Kubernetes.
// It implements the Trireme Resolver interface and implements the policies defined
// by Kubernetes NetworkPolicy API.
type KubernetesPolicy struct {
	policyUpdater     trireme.PolicyUpdater
	KubernetesClient  *kubernetes.Client
	cache             *cache
	stopAll           chan bool
	stopNamespaceChan chan bool
	routineCount      int
}

// NewKubernetesPolicy creates a new policy engine for the Trireme package
func NewKubernetesPolicy(kubeconfig string, namespace string, nodename string) (*KubernetesPolicy, error) {
	client, err := kubernetes.NewClient(kubeconfig, namespace, nodename)
	if err != nil {
		return nil, fmt.Errorf("Couldn't create KubernetesClient: %v ", err)
	}

	return &KubernetesPolicy{
		cache:            newCache(),
		KubernetesClient: client,
		routineCount:     0,
	}, nil
}

// isNamespaceNetworkPolicyActive returns true if the namespace has NetworkPolicies
// activated on the annotation
func isNamespaceNetworkPolicyActive(namespace *api.Namespace) bool {
	// Statically never actvating anything into Kube-System namespace.
	// TODO: Allow KubeSystem to have networking policies enabled ?
	if namespace.GetName() == "kube-system" {
		return false
	}

	// Check if annotation is present. As NetworkPolicies in K8s are still beta
	// The format needs to be manually parsed out of JSON.
	value, ok := namespace.GetAnnotations()[KubernetesNetworkPolicyAnnotationID]

	if !ok {
		return false
	}
	networkPolicyAnnotation := &NamespaceNetworkPolicy{}
	if err := json.Unmarshal([]byte(value), networkPolicyAnnotation); err != nil {
		return false
	}
	//
	if networkPolicyAnnotation != nil &&
		networkPolicyAnnotation.Ingress != nil &&
		networkPolicyAnnotation.Ingress.Isolation != nil &&
		*networkPolicyAnnotation.Ingress.Isolation == DefaultDeny {
		return true
	}
	return false
}

// isNamespaceKubeSystem returns true if the namespace is kube-system
func isNamespaceKubeSystem(namespace string) bool {
	return namespace == "kube-system"
}

// SetPolicyUpdater registers the interface used for updating Policies explicitely.
func (k *KubernetesPolicy) SetPolicyUpdater(p trireme.PolicyUpdater) error {
	k.policyUpdater = p
	return nil
}

// ResolvePolicy generates the Policy for the target PU.
// The policy for the PU will be based on the defined
// Kubernetes NetworkPolicies on the Pod to which the PU belongs.
func (k *KubernetesPolicy) ResolvePolicy(contextID string, runtimeGetter policy.RuntimeReader) (*policy.PUPolicy, error) {

	// Only the Infra Container should be policed. All the others should be AllowAll.
	// The Infra container can be found by checking env. variable.
	value, ok := runtimeGetter.Tag(KubernetesContainerName)
	if !ok || value != KubernetesInfraContainerName {
		// return AllowAll
		return notInfraContainerPolicy(), nil
	}

	podName, ok := runtimeGetter.Tag(KubernetesPodName)
	if !ok {
		return nil, fmt.Errorf("Error getting Kubernetes Pod name")
	}
	podNamespace, ok := runtimeGetter.Tag(KubernetesPodNamespace)
	if !ok {
		return nil, fmt.Errorf("Error getting Kubernetes Pod namespace")
	}
	k.cache.addPodToCache(contextID, podName, podNamespace)
	glog.V(2).Infof("Create pod Policy for %s , namespace %s ", podName, podNamespace)
	return k.resolvePodPolicy(podName, podNamespace)
}

// HandleDeletePU  is called by Trireme for notification that a specific PU is deleted.
// No action is taken based on this.
func (k *KubernetesPolicy) HandleDeletePU(contextID string) error {
	glog.V(5).Infof("Deleting Container %s", contextID)
	return nil
}

// HandleDestroyPU  is called by Trireme for notification that a specific PU is destroyed.
// No action is taken based on this.
func (k *KubernetesPolicy) HandleDestroyPU(contextID string) error {
	glog.V(6).Infof("destroying Container %s", contextID)
	return nil
}

// resolvePodPolicy generates the Trireme Policy for a specific Kube Pod and Namespace.
func (k *KubernetesPolicy) resolvePodPolicy(kubernetesPod string, kubernetesNamespace string) (*policy.PUPolicy, error) {

	// We don't want to actiate anything from Kube-System.
	if isNamespaceKubeSystem(kubernetesNamespace) {
		return notInfraContainerPolicy(), nil
	}

	// Query Kube API to get the Pod's label and IP.
	podLabels, podIP, err := k.KubernetesClient.PodLabelsAndIP(kubernetesPod, kubernetesNamespace)
	if err != nil {
		return nil, fmt.Errorf("Couldn't get labels for pod %s : %v", kubernetesPod, err)
	}

	// If IP is empty, wait for an UpdatePodEvent with the Actual PodIP. Not ready to be activated now.
	if podIP == "" || podIP == "host" || podLabels == nil {
		return notInfraContainerPolicy(), nil
	}

	// Check if the Pod's namespace is activated.
	if !k.cache.isNamespaceActive(kubernetesNamespace) {
		// TODO: Find a way to tell to TRIREME Allow All ??
		glog.V(2).Infof("Pod namespace (%s) is not NetworkPolicyActivated, AllowAll", kubernetesNamespace)
		allowAllPuPolicy := allowAllPolicy()
		// adding the namespace as an extra label.
		podLabels["@namespace"] = kubernetesNamespace
		allowAllPuPolicy.PolicyTags = podLabels
		allowAllPuPolicy.PolicyIPs = []string{podIP}
		return allowAllPuPolicy, nil
	}

	// Updating the cacheEntry with the PodLabels.
	k.cache.updatePodLabels(kubernetesPod, kubernetesNamespace, podLabels)
	// adding the namespace as an extra label.
	podLabels["@namespace"] = kubernetesNamespace

	// Generating all the rules and generate policy.
	allRules, err := k.KubernetesClient.PodRules(kubernetesPod, kubernetesNamespace)
	if err != nil {
		return nil, fmt.Errorf("Couldn't get the NetworkPolicies for Pod %s : %s", kubernetesPod, err)
	}
	allNamespaces, err := k.KubernetesClient.AllNamespaces()

	puPolicy, err := createPolicyRules(allRules, kubernetesNamespace, allNamespaces)
	if err != nil {
		return nil, err
	}
	puPolicy.PolicyTags = podLabels
	puPolicy.PolicyIPs = []string{podIP}

	return puPolicy, nil
}

// updatePodPolicy updates (and replace) the policy of the pod given in parameter.
func (k *KubernetesPolicy) updatePodPolicy(pod *api.Pod) error {
	podName := pod.GetName()
	podNamespace := pod.GetNamespace()
	glog.V(2).Infof("Update pod Policy for %s , namespace %s ", podName, podNamespace)

	if k.policyUpdater == nil {
		return fmt.Errorf("PolicyUpdate failed: No PolicyUpdater registered")
	}

	// Finding back the ContextID for that specificPod.
	contextID, err := k.cache.contextIDByPodName(podName, podNamespace)
	if err != nil {
		return fmt.Errorf("Error finding pod in cache for update: %s", err)
	}

	// Regenerating a Full Policy and Tags.
	containerPolicy, err := k.resolvePodPolicy(podName, podNamespace)
	if err != nil {
		return fmt.Errorf("Couldn't generate a Pod Policy for pod update %s", err)
	}
	returnChan := k.policyUpdater.UpdatePolicy(contextID, containerPolicy)
	if err := <-returnChan; err != nil {
		return fmt.Errorf("Error while updating the policy: %s", err)
	}
	return nil
}

// networkPolicyEventHandler handle the networkPolicy Events
func (k *KubernetesPolicy) networkPolicyEventHandler(networkPolicy *extensions.NetworkPolicy, eventType watch.EventType) error {
	switch eventType {
	case watch.Added, watch.Deleted, watch.Modified:

		glog.V(5).Infof("New K8S NetworkPolicy change detected: %s namespace: %s", networkPolicy.GetName(), networkPolicy.GetNamespace())

		// TODO: Filter on pods from localNode only.
		allLocalPods, err := k.KubernetesClient.LocalPods(networkPolicy.Namespace)
		if err != nil {
			return fmt.Errorf("Couldn't get all local pods: %s", err)
		}
		affectedPods, err := kubepox.ListPodsPerPolicy(networkPolicy, allLocalPods)
		if err != nil {
			return fmt.Errorf("Couldn't get all pods for policy: %s , %s ", networkPolicy.GetName(), err)
		}
		//Reresolve all affected pods
		for _, pod := range affectedPods.Items {
			glog.V(5).Infof("Updating pod: %s in namespace %s based on a K8S NetworkPolicy Change", pod.Name, pod.Namespace)
			err := k.updatePodPolicy(&pod)
			if err != nil {
				return fmt.Errorf("UpdatePolicy failed: %s", err)
			}
		}

	case watch.Error:
		return fmt.Errorf("Error on networkPolicy event channel ")
	}
	return nil
}

// podEventHandler handles the pod Events.
func (k *KubernetesPolicy) podEventHandler(pod *api.Pod, eventType watch.EventType) error {
	switch eventType {
	case watch.Added:
		glog.V(5).Infof("New K8S pod Added detected: %s namespace: %s", pod.GetName(), pod.GetNamespace())
	case watch.Deleted:
		glog.V(5).Infof("New K8S pod Deleted detected: %s namespace: %s", pod.GetName(), pod.GetNamespace())
		err := k.cache.deleteFromCacheByPodName(pod.GetName(), pod.GetNamespace())
		if err != nil {
			return fmt.Errorf("Error for PodDelete: %s ", err)
		}
	case watch.Modified:
		glog.V(5).Infof("New K8S pod Modified detected: %s namespace: %s", pod.GetName(), pod.GetNamespace())

		latest, err := k.cache.isLatestLabelSet(pod.GetName(), pod.GetNamespace(), pod.GetLabels())
		if err != nil {
			return fmt.Errorf("Failed to get pod in cache on ModifiedPodEvent: %s", err)
		}
		if latest {
			glog.V(5).Infof("No modified labels for Pod: %s namespace: %s", pod.GetName(), pod.GetNamespace())
			return nil
		}
		err = k.updatePodPolicy(pod)
		if err != nil {
			return fmt.Errorf("Failed UpdatePolicy on ModifiedPodEvent. Probably related to ongoing delete: %s", err)
		}
	case watch.Error:
		return fmt.Errorf("Error on pod event channel ")
	}
	return nil
}

// updateNamespace check if the policy for a specific namespace changed.
// If the policyactivation changed, it will resync all the pods on that namespace.
func (k *KubernetesPolicy) namespaceEventHandler(namespace *api.Namespace, eventType watch.EventType) error {
	switch eventType {
	case watch.Added:
		if k.cache.isNamespaceActive(namespace.GetName()) {
			// Namespace already activated
			glog.V(2).Infof("Namespace %s Added. already active", namespace.Name)
			return nil
		}
		if !isNamespaceNetworkPolicyActive(namespace) {
			// Namespace doesn't have NetworkPolicies activated
			glog.V(2).Infof("Namespace %s Added. doesn't have NetworkPolicies support. Not activating", namespace.Name)
			return nil
		}
		glog.V(2).Infof("Namespace %s Added. Activating", namespace.Name)
		return k.activateNamespace(namespace)

	case watch.Deleted:
		if k.cache.isNamespaceActive(namespace.GetName()) {
			glog.V(2).Infof("Namespace %s Deleted. Deactivating", namespace.Name)
			return k.deactivateNamespace(namespace)
		}

	case watch.Modified:
		if isNamespaceNetworkPolicyActive(namespace) {
			if k.cache.isNamespaceActive(namespace.GetName()) {
				glog.V(2).Infof("Namespace %s Modified. already active", namespace.Name)
				return nil
			}
			glog.V(2).Infof("Namespace %s Modified. Activating", namespace.Name)
			return k.activateNamespace(namespace)
		}

		if k.cache.isNamespaceActive(namespace.Name) {
			glog.V(2).Infof("Namespace %s Modified. Deactivating", namespace.Name)
			return k.deactivateNamespace(namespace)
		}
		glog.V(2).Infof("Namespace %s Modified. doesn't have NetworkPolicies support. Not activating", namespace.Name)
	}
	return nil
}

// activateNamespace starts to watch the pods and networkpolicies in the parameter namespace.
func (k *KubernetesPolicy) activateNamespace(namespace *api.Namespace) error {
	glog.V(2).Infof("Activating namespace %s ", namespace.Name)
	namespaceWatcher := NewNamespaceWatcher(k.KubernetesClient, namespace.Name)
	k.cache.activateNamespaceWatcher(namespace.Name, namespaceWatcher)
	// SyncExistingPods on Namespace
	namespaceWatcher.syncNamespace(k.KubernetesClient, k.updatePodPolicy)
	// Start watching new POD/Policy events.
	go namespaceWatcher.startWatchingNamespace(k.podEventHandler, k.networkPolicyEventHandler)
	return nil
}

// deactivateNamespace stops all the watching on the specified namespace.
func (k *KubernetesPolicy) deactivateNamespace(namespace *api.Namespace) error {
	glog.V(2).Infof("Deactivating namespace %s ", namespace.GetName())
	k.cache.deactivateNamespaceWatcher(namespace.GetName())
	return nil
}

// processNamespacesEvent watches all namespaces coming on the parameter chan.
// Based on the event, it will update the NamespaceWatcher
func (k *KubernetesPolicy) processNamespacesEvent(resultChan <-chan watch.Event, stopChan <-chan bool) {
	for {
		select {
		case <-stopChan:
			glog.V(2).Infof("Stopping namespace processor ")
			return
		case req := <-resultChan:
			namespace := req.Object.(*api.Namespace)
			glog.V(5).Infof("Processing namespace event for NS %s ", namespace.GetName())
			err := k.namespaceEventHandler(namespace, req.Type)
			if err != nil {
				glog.V(1).Infof("Error while processing NS event %s ", namespace.GetName())
			}
		}
	}
}

// syncAllNamespaces iterates over all the existing Kube namespaces and activates the needed ones.
func (k *KubernetesPolicy) syncAllNamespaces() error {
	namespaces, err := k.KubernetesClient.AllNamespaces()
	if err != nil {
		return fmt.Errorf("Couldn't get all namespaces %s ", err)
	}
	for _, namespace := range namespaces.Items {
		// For this sync, we fake receiving one new Added event per namespace.
		err := k.namespaceEventHandler(&namespace, watch.Added)
		if err != nil {
			glog.V(1).Infof("Error while processing NS sync %s ", namespace.GetName())
		}
	}
	return nil
}

// Start starts the KubernetesPolicer as a daemon.
// Effectively it registers watcher for:
// Namespace, Pod and networkPolicy changes
func (k *KubernetesPolicy) Start() {

	// Start by syncing all existing namespaces
	if err := k.syncAllNamespaces(); err != nil {
		glog.V(2).Infof("Error Syncing namespaces %s", err)
	}

	// Continue to watch for Namespaces changes.
	// resultChan holds all the Kubernetes namespaces events.
	resultNamespaceChan := make(chan watch.Event)
	k.stopNamespaceChan = make(chan bool)
	go k.KubernetesClient.NamespaceWatcher(resultNamespaceChan, k.stopNamespaceChan)

	// Process the new Namespace events coming ober the resultNamespaceChan.
	k.stopAll = make(chan bool)
	k.processNamespacesEvent(resultNamespaceChan, k.stopAll)
}

// Stop Stops all the channels
func (k *KubernetesPolicy) Stop() {
	k.stopAll <- true
	k.stopNamespaceChan <- true
	for _, namespaceWatcher := range k.cache.namespaceActivation {
		namespaceWatcher.stopWatchingNamespace()
	}
}
