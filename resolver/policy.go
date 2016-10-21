package resolver

import (
	"fmt"

	"github.com/aporeto-inc/kubernetes-integration/kubernetes"
	"github.com/aporeto-inc/trireme"

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

// KubernetesNetworkPolicyAnnotationID is the string used as an annotation key
// to define if a namespace should have the networkpolicy framework enabled.
const KubernetesNetworkPolicyAnnotationID = "net.beta.kubernetes.io/network-policy"

// KubernetesPolicy represents a Trireme Policer for Kubernetes.
// It implements the Trireme Resolver interface and implements the policies defined
// by Kubernetes NetworkPolicy API.
type KubernetesPolicy struct {
	policyUpdater     trireme.PolicyUpdater
	Kubernetes        *kubernetes.Client
	cache             *Cache
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
		cache:        newCache(),
		Kubernetes:   client,
		routineCount: 0,
	}, nil
}

// SetPolicyUpdater registers the interface used for updating Policies explicitely.
func (k *KubernetesPolicy) SetPolicyUpdater(p trireme.PolicyUpdater) error {
	k.policyUpdater = p
	return nil
}

// createPolicyRules populate the RuleDB of a PU based on the list
// of IngressRules coming from Kubernetes.
func createPolicyRules(rules *[]extensions.NetworkPolicyIngressRule) (*policy.PUPolicy, error) {
	containerPolicy := policy.NewPUPolicy()

	for _, rule := range *rules {
		// Populate the clauses related to each individual rules.
		individualRule(containerPolicy, &rule)
	}
	logRules(containerPolicy)
	return containerPolicy, nil
}

// GetPodPolicy get the Trireme Policy for a specific Pod and Namespace.
func (k *KubernetesPolicy) GetPodPolicy(kubernetesPod string, kubernetesNamespace string) (*policy.PUPolicy, error) {
	// Adding all the specific Kubernetes K,V from the Pod.
	// Iterate on PodLabels and add them as tags
	podLabels, err := k.Kubernetes.PodLabels(kubernetesPod, kubernetesNamespace)
	if err != nil {
		return nil, fmt.Errorf("Couldn't get Kubernetes labels for container %s : %v", kubernetesPod, err)
	}

	// Check if the Pod's namespace is activated.
	if !k.cache.namespaceStatus(kubernetesNamespace) {
		// TODO: Find a way to tell to TRIREME Allow All ??
		glog.V(2).Infof("Pod namespace (%s) is not NetworkPolicyActivated, AllowAll", kubernetesNamespace)
		pupolicy := policy.NewPUPolicy()
		pupolicy.TriremeAction = policy.AllowAll
		return pupolicy, nil
	}

	allRules, err := k.Kubernetes.PodRules(kubernetesPod, kubernetesNamespace)
	if err != nil {
		return nil, fmt.Errorf("Couldn't get the NetworkPolicies for Pod %s : %s", kubernetesPod, err)
	}

	// Step2: Translate all the metadata labels to Trireme Rules
	containerPolicy, err := createPolicyRules(allRules)
	if err != nil {
		return nil, err
	}
	containerPolicy.PolicyTags = podLabels

	// Step3: Done
	return containerPolicy, nil
}

// ResolvePolicy returns the Policy for the target PU.
// The policy for the PU will be based on the defined
// Kubernetes NetworkPolicies on the Pod to which the PU belongs.
func (k *KubernetesPolicy) ResolvePolicy(contextID string, runtimeGetter policy.RuntimeReader) (*policy.PUPolicy, error) {
	podName := runtimeGetter.Tags()[KubernetesPodName]
	podNamespace := runtimeGetter.Tags()[KubernetesPodNamespace]
	k.cache.addPodToCache(contextID, podName, podNamespace)
	return k.GetPodPolicy(podName, podNamespace)
}

// HandleDeletePU deletes a specific PU.
func (k *KubernetesPolicy) HandleDeletePU(contextID string) error {
	glog.V(2).Infof("Deleting Container Policy %s", contextID)
	return nil
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
	containerPolicy, err := k.GetPodPolicy(podName, podNamespace)
	if err != nil {
		return fmt.Errorf("Couldn't generate a Pod Policy for pod update %s", err)
	}
	returnChan := k.policyUpdater.UpdatePolicy(contextID, containerPolicy)
	if err := <-returnChan; err != nil {
		return fmt.Errorf("Error while updating the policy: %s", err)
	}
	return nil
}

// namespaceSync iterates over all the existing Kube namespaces and sync all the needed
func (k *KubernetesPolicy) namespaceSync() error {
	namespaces, err := k.Kubernetes.AllNamespaces()
	if err != nil {
		return fmt.Errorf("Couldn't get all namespaces %s ", err)
	}
	for _, namespace := range namespaces.Items {
		err := k.updateNamespace(&namespace, watch.Added)
		if err != nil {
			glog.V(1).Infof("Error while processing NS sync %s ", namespace.GetName())
		}
	}
	return nil
}

// NamespacePolicyActivated returns true if the namespace has NetworkPolicies
// activated on the annotation
func isNamespacePolicyActive(namespace *api.Namespace) bool {
	//TODO: Check on the correct annotation. For now activating all the existing namespaces
	if namespace.GetName() == "kube-system" {
		return false
	}
	return true
}

// updateNamespace check if the policy for a specific namespace changed.
// If the policyactivation changed, it will resync all the pods on that namespace.
func (k *KubernetesPolicy) updateNamespace(namespace *api.Namespace, eventType watch.EventType) error {
	switch eventType {
	case watch.Added:
		if k.cache.namespaceStatus(namespace.GetName()) {
			// Namespace already activated
			glog.V(2).Infof("Namespace %s Added. already active", namespace.Name)
			return nil
		}
		if !isNamespacePolicyActive(namespace) {
			// Namespace doesn't have NetworkPolicies activated
			glog.V(2).Infof("Namespace %s Added. doesn't have NetworkPolicies support. Not activating", namespace.Name)
			return nil
		}
		glog.V(2).Infof("Namespace %s Added. Activating", namespace.Name)
		return k.activateNamespace(namespace)

	case watch.Deleted:
		if k.cache.namespaceStatus(namespace.GetName()) {
			glog.V(2).Infof("Namespace %s Deleted. Deactivating", namespace.Name)
			return k.deactivateNamespace(namespace)
		}

	case watch.Modified:
		if isNamespacePolicyActive(namespace) {
			if k.cache.namespaceStatus(namespace.GetName()) {
				glog.V(2).Infof("Namespace %s Modified. already active", namespace.Name)
				return nil
			}
			glog.V(2).Infof("Namespace %s Modified. Activating", namespace.Name)
			return k.activateNamespace(namespace)
		}

		if k.cache.namespaceStatus(namespace.Name) {
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
	namespaceWatcher := NewNamespaceWatcher(k.Kubernetes, namespace.Name)
	// SyncExistingPods on Namespace
	namespaceWatcher.syncNamespace(k.Kubernetes, k.updatePodPolicy)
	// Start watching new POD/Policy events.
	go namespaceWatcher.startWatchingNamespace(k.podEventHandler, k.networkPolicyEventHandler)
	k.cache.activateNamespaceWatcher(namespace.Name, namespaceWatcher)
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
			glog.V(2).Infof("Processing namespace event for NS %s ", namespace.GetName())
			err := k.updateNamespace(namespace, req.Type)
			if err != nil {
				glog.V(1).Infof("Error while processing NS event %s ", namespace.GetName())
			}
		}
	}
}

// Start starts the KubernetesPolicer as a daemon.
// Effectively it registers watcher for:
// Namespace, Pod and networkPolicy changes
func (k *KubernetesPolicy) Start() {

	// Start by checking all existing namespaces to see which one got activated.
	if err := k.namespaceSync(); err != nil {
		glog.V(2).Infof("Error Syncing namespaces %s", err)
	}

	// Continue to watch for Namespaces changes.
	// resultChan holds all the Kubernetes namespaces events.
	resultNamespaceChan := make(chan watch.Event)
	k.stopNamespaceChan = make(chan bool)
	go k.Kubernetes.NamespaceWatcher(resultNamespaceChan, k.stopNamespaceChan)

	// Process the new Namespace events.
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
