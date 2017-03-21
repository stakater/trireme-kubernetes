package resolver

import (
	"encoding/json"
	"fmt"
	"net"

	"k8s.io/apimachinery/pkg/labels"
	api "k8s.io/client-go/pkg/api/v1"
	extensions "k8s.io/client-go/pkg/apis/extensions/v1beta1"

	"github.com/aporeto-inc/kubepox"
	"github.com/aporeto-inc/trireme"
	"github.com/aporeto-inc/trireme-kubernetes/kubernetes"

	"github.com/aporeto-inc/trireme/monitor"
	"github.com/aporeto-inc/trireme/policy"
	"github.com/aporeto-inc/trireme/supervisor"
	"github.com/golang/glog"
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
	triremeNetworks   []string
	policyUpdater     trireme.PolicyUpdater
	excluder          supervisor.Excluder
	localExcluded     bool
	KubernetesClient  *kubernetes.Client
	cache             *cache
	stopAll           chan struct{}
	stopNamespaceChan chan struct{}
}

// NewKubernetesPolicy creates a new policy engine for the Trireme package
func NewKubernetesPolicy(kubeconfig string, nodename string, triremeNetworks []string) (*KubernetesPolicy, error) {
	client, err := kubernetes.NewClient(kubeconfig, nodename)
	if err != nil {
		return nil, fmt.Errorf("Couldn't create KubernetesClient: %v ", err)
	}

	return &KubernetesPolicy{
		triremeNetworks:  triremeNetworks,
		cache:            newCache(),
		KubernetesClient: client,
		localExcluded:    false,
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

func isPolicyUpdateNeeded(oldPod, newPod *api.Pod) bool {
	if !(oldPod.Status.PodIP == newPod.Status.PodIP) {
		return true
	}
	if !labels.Equals(oldPod.GetLabels(), newPod.GetLabels()) {
		return true
	}
	return false
}

// SetPolicyUpdater registers the interface used for updating Policies explicitely.
func (k *KubernetesPolicy) SetPolicyUpdater(policyUpdater trireme.PolicyUpdater) error {
	k.policyUpdater = policyUpdater
	return nil
}

// SetExcluder registers the interface used for updating Policies explicitely.
func (k *KubernetesPolicy) SetExcluder(excluder supervisor.Excluder) error {
	k.excluder = excluder
	return nil
}

// excludeLocalIP registers the CNI Interface on the node for explicit allowance.
// This is needed for HealtCheck and LocalHost communication to the Pod.
func (k *KubernetesPolicy) excludeLocalIP(ip string) error {
	parsedIP := net.ParseIP(ip)
	ipb := parsedIP.To4()
	ipb[3] = 1
	k.localExcluded = true
	return nil
	//return k.excluder.AddExcludedIPs([]string{ipb.String() + "/32"})
}

// ResolvePolicy generates the Policy for the target PU.
// The policy for the PU will be based on the defined
// Kubernetes NetworkPolicies on the Pod to which the PU belongs.
func (k *KubernetesPolicy) ResolvePolicy(contextID string, runtimeGetter policy.RuntimeReader) (*policy.PUPolicy, error) {

	// Only the Infra Container should be policed. All the others should be AllowAll.
	// The Infra container can be found by checking env. variable.
	tagContent, ok := runtimeGetter.Tag(KubernetesContainerName)
	if !ok || tagContent != KubernetesInfraContainerName {
		// return AllowAll
		glog.V(2).Infof("Container is not Infra Container. AllowingAll. %s ", contextID)
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

	// Keep the mapping in cache: ContextID <--> PodNamespace/PodName
	k.cache.addPodToCache(contextID, podName, podNamespace)
	return k.resolvePodPolicy(podName, podNamespace)
}

// HandlePUEvent  is called by Trireme for notification that a specific PU got an event.
func (k *KubernetesPolicy) HandlePUEvent(contextID string, eventType monitor.Event) {
	glog.V(10).Infof("Container %s Event %s", contextID, eventType)
}

// resolvePodPolicy generates the Trireme Policy for a specific Kube Pod and Namespace.
func (k *KubernetesPolicy) resolvePodPolicy(kubernetesPod string, kubernetesNamespace string) (*policy.PUPolicy, error) {

	// Query Kube API to get the Pod's label and IP.
	pod, err := k.KubernetesClient.Pod(kubernetesPod, kubernetesNamespace)
	if err != nil {
		return nil, fmt.Errorf("Couldn't get labels for pod %s : %v", kubernetesPod, err)
	}

	// If IP is empty, wait for an UpdatePodEvent with the Actual PodIP. Not ready to be activated now.
	if pod.Status.PodIP == "" {
		fmt.Println("NotInfra")
		return notInfraContainerPolicy(), nil
	}
	// If Pod is running in the hostNS , no need for activation.
	if pod.Status.PodIP == pod.Status.HostIP {
		return notInfraContainerPolicy(), nil
	}

	// TODO: Cleanup excluder code
	if !k.localExcluded {
		k.excludeLocalIP(pod.Status.PodIP)
	}

	podLabels := pod.GetLabels()
	if podLabels == nil {
		return notInfraContainerPolicy(), nil
	}

	// Check if the Pod's namespace is activated.
	if !k.cache.isNamespaceActive(kubernetesNamespace) {

		glog.V(2).Infof("Pod namespace (%s) is not NetworkPolicyActivated, AllowAll.", kubernetesNamespace)
		// adding the namespace as an extra label.
		podLabels["@namespace"] = kubernetesNamespace
		ips := map[string]string{policy.DefaultNamespace: pod.Status.PodIP}
		allowAllPuPolicy := allowAllPolicy(policy.NewTagsMap(podLabels), policy.NewIPMap(ips), k.triremeNetworks)

		return allowAllPuPolicy, nil
	}

	// adding the namespace as an extra label.
	podLabels["@namespace"] = kubernetesNamespace

	// Generating all the rules and generate policy.

	// TODO: Quick hack to generate NetworkPolicy from the store instead than from the API.
	// Replace this with correct API Calls whenever client-go will support it.
	nsWatcher, exist := k.cache.getNamespaceWatcher(kubernetesNamespace)
	if !exist {
		return nil, fmt.Errorf("Couldn't find active Namespace %s ", kubernetesNamespace)
	}

	namespaceRules, err := nsWatcher.getPolicyList()
	if err != nil {
		return nil, fmt.Errorf("Couldn't generate current NetPolicies for the namespace %s ", kubernetesNamespace)
	}

	podRules, err := k.KubernetesClient.PodRules(kubernetesPod, kubernetesNamespace, namespaceRules)
	if err != nil {
		return nil, fmt.Errorf("Couldn't get the NetworkPolicies for Pod %s : %s", kubernetesPod, err)
	}
	allNamespaces, _ := k.KubernetesClient.AllNamespaces()

	ips := map[string]string{policy.DefaultNamespace: pod.Status.PodIP}

	puPolicy, err := generatePUPolicy(podRules, kubernetesNamespace, allNamespaces, policy.NewTagsMap(podLabels), policy.NewIPMap(ips), k.triremeNetworks)
	if err != nil {
		return nil, err
	}

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

// activateNamespace starts to watch the pods and networkpolicies in the parameter namespace.
func (k *KubernetesPolicy) activateNamespace(namespace *api.Namespace) error {
	glog.V(2).Infof("Activating namespace %s for NetworkPolicies", namespace.Name)

	podControllerStop := make(chan struct{})
	podStore, podController := k.KubernetesClient.CreateLocalPodController(namespace.Name,
		k.addPod,
		k.deletePod,
		k.updatePod)
	go podController.Run(podControllerStop)

	npControllerStop := make(chan struct{})
	npStore, npController := k.KubernetesClient.CreateNetworkPoliciesController(namespace.Name,
		k.addNetworkPolicy,
		k.deleteNetworkPolicy,
		k.updateNetworkPolicy)
	go npController.Run(npControllerStop)

	namespaceWatcher := NewNamespaceWatcher(namespace.Name, podStore, podController, podControllerStop, npStore, npController, npControllerStop)
	k.cache.activateNamespaceWatcher(namespace.Name, namespaceWatcher)

	return nil
}

// deactivateNamespace stops all the watching on the specified namespace.
func (k *KubernetesPolicy) deactivateNamespace(namespace *api.Namespace) error {
	glog.V(2).Infof("Deactivating namespace %s ", namespace.GetName())
	k.cache.deactivateNamespaceWatcher(namespace.GetName())
	return nil
}

// Run starts the KubernetesPolicer by watching for Namespace Changes.
// Run is blocking. Use go
func (k *KubernetesPolicy) Run() {
	k.stopAll = make(chan struct{})
	_, nsController := k.KubernetesClient.CreateNamespaceController(
		k.addNamespace,
		k.deleteNamespace,
		k.updateNamespace)
	nsController.Run(k.stopAll)
}

// Stop Stops all the channels
func (k *KubernetesPolicy) Stop() {
	k.stopAll <- struct{}{}
	k.stopNamespaceChan <- struct{}{}
	for _, namespaceWatcher := range k.cache.namespaceActivation {
		namespaceWatcher.stopWatchingNamespace()
	}
}

func (k *KubernetesPolicy) addNamespace(addedNS *api.Namespace) error {
	if k.cache.isNamespaceActive(addedNS.GetName()) {
		// Namespace already activated
		glog.V(2).Infof("Namespace %s Added. already active", addedNS.GetName())
		return nil
	}
	if !isNamespaceNetworkPolicyActive(addedNS) {
		// Namespace doesn't have NetworkPolicies activated
		glog.V(2).Infof("Namespace %s Added. doesn't have NetworkPolicies support. Not activating", addedNS.GetName())
		return nil
	}
	glog.V(2).Infof("Namespace %s Added. Activating", addedNS.GetName())
	return k.activateNamespace(addedNS)
}

func (k *KubernetesPolicy) deleteNamespace(deletedNS *api.Namespace) error {
	if k.cache.isNamespaceActive(deletedNS.GetName()) {
		glog.V(2).Infof("Namespace %s Deleted. Deactivating", deletedNS.GetName())
		return k.deactivateNamespace(deletedNS)
	}
	return nil
}

func (k *KubernetesPolicy) updateNamespace(oldNS, updatedNS *api.Namespace) error {
	if isNamespaceNetworkPolicyActive(updatedNS) {
		if k.cache.isNamespaceActive(updatedNS.GetName()) {
			glog.V(2).Infof("Namespace %s Modified. already active", updatedNS.GetName())
			return nil
		}
		glog.V(2).Infof("Namespace %s Modified. Activating", updatedNS.GetName())
		return k.activateNamespace(updatedNS)
	}

	if k.cache.isNamespaceActive(updatedNS.GetName()) {
		glog.V(2).Infof("Namespace %s Modified. Deactivating", updatedNS.GetName())
		return k.deactivateNamespace(updatedNS)
	}
	glog.V(2).Infof("Namespace %s Modified. doesn't have NetworkPolicies support. Not activating", updatedNS.GetName())
	return nil
}

func (k *KubernetesPolicy) addPod(addedPod *api.Pod) error {
	err := k.updatePodPolicy(addedPod)
	if err != nil {
		return fmt.Errorf("Failed UpdatePolicy on NewPodEvent: %s", err)
	}
	return nil
}

func (k *KubernetesPolicy) deletePod(deletedPod *api.Pod) error {
	glog.V(5).Infof("New K8S pod Deleted detected: %s namespace: %s", deletedPod.GetName(), deletedPod.GetNamespace())
	err := k.cache.deleteFromCacheByPodName(deletedPod.GetName(), deletedPod.GetNamespace())
	if err != nil {
		return fmt.Errorf("Error for PodDelete: %s ", err)
	}
	return nil
}

func (k *KubernetesPolicy) updatePod(oldPod, updatedPod *api.Pod) error {
	glog.V(5).Infof("New K8S pod Modified detected: %s namespace: %s", updatedPod.GetName(), updatedPod.GetNamespace())

	if !isPolicyUpdateNeeded(oldPod, updatedPod) {
		glog.V(5).Infof("No modified labels for Pod: %s namespace: %s", updatedPod.GetName(), updatedPod.GetNamespace())
		return nil
	}
	err := k.updatePodPolicy(updatedPod)
	if err != nil {
		return fmt.Errorf("Failed UpdatePolicy on ModifiedPodEvent. Probably related to ongoing delete: %s", err)
	}
	return nil
}

func (k *KubernetesPolicy) addNetworkPolicy(addedNP *extensions.NetworkPolicy) error {
	glog.V(5).Infof("New K8S NetworkPolicy change detected: %s namespace: %s", addedNP.GetName(), addedNP.GetNamespace())

	// TODO: Filter on pods from localNode only.
	allLocalPods, err := k.KubernetesClient.LocalPods(addedNP.Namespace)
	if err != nil {
		return fmt.Errorf("Couldn't get all local pods: %s", err)
	}
	affectedPods, err := kubepox.ListPodsPerPolicy(addedNP, allLocalPods)
	if err != nil {
		return fmt.Errorf("Couldn't get all pods for policy: %s , %s ", addedNP.GetName(), err)
	}
	//Reresolve all affected pods
	for _, pod := range affectedPods.Items {
		glog.V(5).Infof("Updating pod: %s in namespace %s based on a K8S NetworkPolicy Change", pod.Name, pod.Namespace)
		err := k.updatePodPolicy(&pod)
		if err != nil {
			return fmt.Errorf("UpdatePolicy failed: %s", err)
		}
	}
	return nil
}

func (k *KubernetesPolicy) deleteNetworkPolicy(deletedNP *extensions.NetworkPolicy) error {
	glog.V(5).Infof("New K8S NetworkPolicy change detected: %s namespace: %s", deletedNP.GetName(), deletedNP.GetNamespace())

	// TODO: Filter on pods from localNode only.
	allLocalPods, err := k.KubernetesClient.LocalPods(deletedNP.Namespace)
	if err != nil {
		return fmt.Errorf("Couldn't get all local pods: %s", err)
	}
	affectedPods, err := kubepox.ListPodsPerPolicy(deletedNP, allLocalPods)
	if err != nil {
		return fmt.Errorf("Couldn't get all pods for policy: %s , %s ", deletedNP.GetName(), err)
	}
	//Reresolve all affected pods
	for _, pod := range affectedPods.Items {
		glog.V(5).Infof("Updating pod: %s in namespace %s based on a K8S NetworkPolicy Change", pod.Name, pod.Namespace)
		err := k.updatePodPolicy(&pod)
		if err != nil {
			return fmt.Errorf("UpdatePolicy failed: %s", err)
		}
	}
	return nil
}

func (k *KubernetesPolicy) updateNetworkPolicy(oldNP, updatedNP *extensions.NetworkPolicy) error {
	glog.V(5).Infof("New K8S NetworkPolicy change detected: %s namespace: %s", updatedNP.GetName(), updatedNP.GetNamespace())

	// TODO: Filter on pods from localNode only.
	allLocalPods, err := k.KubernetesClient.LocalPods(updatedNP.Namespace)
	if err != nil {
		return fmt.Errorf("Couldn't get all local pods: %s", err)
	}
	affectedPods, err := kubepox.ListPodsPerPolicy(updatedNP, allLocalPods)
	if err != nil {
		return fmt.Errorf("Couldn't get all pods for policy: %s , %s ", updatedNP.GetName(), err)
	}
	//Reresolve all affected pods
	for _, pod := range affectedPods.Items {
		glog.V(5).Infof("Updating pod: %s in namespace %s based on a K8S NetworkPolicy Change", pod.Name, pod.Namespace)
		err := k.updatePodPolicy(&pod)
		if err != nil {
			return fmt.Errorf("UpdatePolicy failed: %s", err)
		}
	}
	return nil
}
