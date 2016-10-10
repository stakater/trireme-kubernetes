package resolver

import (
	"fmt"

	"github.com/aporeto-inc/kubernetes-integration/kubernetes"

	"github.com/aporeto-inc/trireme"
	"github.com/aporeto-inc/trireme/datapath"
	"github.com/aporeto-inc/trireme/policy"
	"github.com/docker/docker/api/types"
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
	isolator          trireme.Isolator
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

// RegisterIsolator keeps a reference to the Isolator for Callbacks.
// If an isolator is already registered, this one will override the existing reference
// TODO: Refactor to not use registration mechanism
func (k *KubernetesPolicy) RegisterIsolator(isolator trireme.Isolator) {
	k.isolator = isolator
}

// createIndividualRules populate the RuleDB of a Container based on the list
// of IngressRules coming from Kubernetes
func createPolicyRules(req *policy.ContainerInfo, rules *[]extensions.NetworkPolicyIngressRule) error {
	//TODO: Hack to make sure that all Trireme Rules are wiped out before adding new ones.
	req.Policy.Rules = []policy.TagSelectorInfo{}

	for _, rule := range *rules {
		// Populate the clauses related to each individual rules.
		individualRule(req, &rule)
	}
	logRules(req)
	return nil
}

// TODO: ContextID should be meaningful: The Pod name for Kubernetes would make the most sense.
func generateContextID(containerID string) string {
	return containerID[:12]
}

// GetContainerPolicy returns the Policy for the targetContainers.
// The policy for the container will be based on the defined
// Kubernetes NetworkPolicies on the Pod to which the container belongs.
func (k *KubernetesPolicy) GetContainerPolicy(contextID string, containerPolicy *policy.ContainerInfo) error {
	cacheEntry, err := k.cache.getCachedPodByContextID(contextID)
	if err != nil {
		return fmt.Errorf("GetContainerPolicy failed. Pod not found in Cache: %s ", err)
	}

	// Check if the Pod's namespace is activated.
	if !k.cache.namespaceStatus(cacheEntry.podNamespace) {
		// TODO: Find a way to tell to TRIREME Allow All ??
		glog.V(2).Infof("Pod namespace (%s) is not NetworkPolicyActivated, AllowAll", cacheEntry.podNamespace)
		return nil
	}

	allRules, err := k.Kubernetes.PodRules(cacheEntry.podName, cacheEntry.podNamespace)
	if err != nil {
		return fmt.Errorf("Couldn't get the NetworkPolicies for Pod %s : %s", cacheEntry.podName, err)
	}

	// Step2: Translate all the metadata labels to Trireme Rules
	if err := createPolicyRules(containerPolicy, allRules); err != nil {
		return err
	}

	// Step3: Done
	return nil
}

// DeleteContainerPolicy deletes the container from Cache.
// TODO: Refactor so that it only returns an error. no ContainerInfo should be returned.
func (k *KubernetesPolicy) DeleteContainerPolicy(contextID string) *policy.ContainerInfo {
	glog.V(2).Infof("Deleting Container Policy %s", contextID)
	_, err := k.cache.getCachedPodByContextID(contextID)
	if err != nil {
		// TODO: Return error
		glog.V(2).Infof("Couldn't find Pod for ContextID %s", contextID)
	}
	k.cache.deletePodFromCacheByContextID(contextID)
	return nil
}

// MetadataExtractor implements the extraction of metadata from the Docker data
func (k *KubernetesPolicy) MetadataExtractor(info *types.ContainerJSON) (string, *policy.ContainerInfo, error) {
	containerName := info.Name
	containerID := info.ID
	podName, ok := info.Config.Labels[KubernetesPodName]
	if !ok {
		glog.V(2).Infof("No podName Found for container %s. Must not be K8S Pod Container. Not activating ", containerName)
		return "", nil, nil
	}

	podNamespace, ok := info.Config.Labels[KubernetesPodNamespace]
	if !ok {
		glog.V(2).Infof("No podNamespace Found for container %s. Must not be K8S Pod Container. Not activating ", containerName)
		return "", nil, nil
	}

	kubeContainerName, ok := info.Config.Labels[KubernetesContainerName]
	if !ok {
		glog.V(2).Infof("No Kubernetes container name Found for container %s. Must not be K8S Pod Container. Not activating ", containerName)
		return "", nil, nil
	}

	// Only activate the POD Kubernetes container.
	if kubeContainerName != "POD" {
		glog.V(2).Infof("Kubernetes Container (%s) is not Infra container %s. Not activating ", kubeContainerName, containerName)
		return "", nil, nil
	}

	glog.V(2).Infof("Processing Metadata for Kubernetes POD (%s) Container: %s", podName, containerName)

	contextID := generateContextID(containerID)
	containerInfo := policy.NewContainerInfo(contextID)
	containerInfo.RunTime.Pid = info.State.Pid

	//TODO: What behaviour if POD IP is found without an IP ? Erroring for now.
	if info.NetworkSettings.IPAddress == "" {
		return "", nil, fmt.Errorf("IP not present on Kubernetes POD (%s) container: %s", podName, containerName)
	}

	containerInfo.RunTime.IPAddresses["bridge"] = info.NetworkSettings.IPAddress
	containerInfo.RunTime.Name = containerName

	//TODO: Refactor to only include the ACTUAL labels. Everything else should be outside
	containerInfo.RunTime.Tags["name"] = info.Name
	containerInfo.RunTime.Tags[datapath.TransmitterLabel] = contextID

	// Adding all the specific Kubernetes K,V from the Pod.
	// Iterate on PodLabels and add them as tags
	podLabels, err := k.Kubernetes.PodLabels(info.Config.Labels[KubernetesPodName], info.Config.Labels[KubernetesPodNamespace])
	if err != nil {
		return "", nil, fmt.Errorf("Couldn't get Kubernetes labels for container %s : %v", containerName, err)
	}
	for key, value := range podLabels {
		containerInfo.RunTime.Tags[key] = value
	}

	k.cache.addPodToCache(contextID, containerID, podName, podNamespace, containerInfo)
	return contextID, containerInfo, nil
}

// updatePodPolicy updates (replace) the policy of the pod given in parameter.
// TODO: Handle cases where the Pod is not found in cache
func (k *KubernetesPolicy) updatePodPolicy(pod *api.Pod) error {
	glog.V(2).Infof("Update pod Policy for %s , namespace %s ", pod.Name, pod.Namespace)
	cachedEntry, err := k.cache.getCachedPodByName(pod.Name, pod.Namespace)
	if err != nil {
		return fmt.Errorf("Error finding pod in cache: %s", err)
	}
	contextID, err := k.cache.getContextIDByPodName(pod.Name, pod.Namespace)
	if err != nil {
		return fmt.Errorf("Error finding pod in cache: %s", err)
	}
	k.GetContainerPolicy(contextID, cachedEntry.containerInfo)
	k.isolator.UpdatePolicy(contextID, cachedEntry.containerInfo)
	return nil
}

func (k *KubernetesPolicy) namespaceSync() error {
	namespaces, err := k.Kubernetes.AllNamespaces()
	if err != nil {
		return fmt.Errorf("Couldn't get all namespaces %s ", err)
	}
	for _, namespace := range namespaces.Items {
		k.updateNamespace(&namespace, watch.Added)
	}
	return nil
}

func (k *KubernetesPolicy) processNamespacesEvent(resultChan <-chan watch.Event, stopChan <-chan bool) {
	for {
		select {
		case <-stopChan:
			glog.V(2).Infof("Stopping namespace processor ")
			return
		case req := <-resultChan:
			namespace := req.Object.(*api.Namespace)
			glog.V(2).Infof("Processing namespace event for NS %s ", namespace.GetName())
			k.updateNamespace(namespace, req.Type)
		}
	}
}

// NamespacePolicyActivated returns true if the namespace has NetworkPolicies
// activated on the annotation
func NamespacePolicyActivated(namespace *api.Namespace) bool {
	//TODO: Check on the correct annotation. For now activating all the existing namespaces
	return true
}

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

func (k *KubernetesPolicy) deactivateNamespace(namespace *api.Namespace) error {
	glog.V(2).Infof("Deactivating namespace %s ", namespace.GetName())
	k.cache.deactivateNamespaceWatcher(namespace.GetName())
	return nil
}

// updateNamespace check if the policy for the namespace changed.
// If the policy changed, it will resync all the pods on that namespace.
func (k *KubernetesPolicy) updateNamespace(namespace *api.Namespace, eventType watch.EventType) error {
	switch eventType {
	case watch.Added:
		if k.cache.namespaceStatus(namespace.GetName()) {
			// Namespace already activated
			glog.V(2).Infof("Namespace %s Added. already active", namespace.Name)
			return nil
		}
		if !NamespacePolicyActivated(namespace) {
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
		if NamespacePolicyActivated(namespace) {
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
