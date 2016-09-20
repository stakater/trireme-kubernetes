package policy

import (
	"fmt"

	"github.com/aporeto-inc/kubernetes-integration/kubernetes"

	"github.com/aporeto-inc/trireme"
	"github.com/aporeto-inc/trireme/datapath"
	"github.com/aporeto-inc/trireme/datapath/lookup"
	"github.com/aporeto-inc/trireme/policy"
	"github.com/docker/docker/api/types"
	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	apiu "k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/extensions"
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
	isolator   trireme.Isolator
	kubernetes *kubernetes.KubernetesClient
	cache      *Cache
}

// NewKubernetesPolicy creates a new policy engine for the Trireme package
func NewKubernetesPolicy(kubeconfig string, namespace string) (*KubernetesPolicy, error) {
	client, err := kubernetes.NewKubernetesClient(kubeconfig, namespace)
	if err != nil {
		return nil, fmt.Errorf("Couldn't create KubernetesClient: %v ", err)
	}

	return &KubernetesPolicy{
		cache:      newCache(),
		kubernetes: client,
	}, nil
}

// RegisterIsolator keeps a reference to the Isolator for Callbacks.
// If an isolator is already registered, this one will override the existing reference
// TODO: Refactor to not use registration mechanism
func (k *KubernetesPolicy) RegisterIsolator(isolator trireme.Isolator) {
	k.isolator = isolator
}

// createIndividualRules populate the RuleDB of a Container based on the list of
// of IngressRules coming from Kubernetes
func createIndividualRules(req *policy.ContainerInfo, allRules *[]extensions.NetworkPolicyIngressRule) error {
	//TODO: Temp hack to temporary create new rules:
	req.Policy.Rules = lookup.NewRuleDB()

	for _, rule := range *allRules {
		for _, from := range rule.From {
			labelsKeyValue, err := apiu.LabelSelectorAsMap(from.PodSelector)
			if err != nil {
				return err
			}
			for key, value := range labelsKeyValue {
				req.Policy.Rules.AddElements([]string{key + "=" + value}, "accept")
			}
		}
	}
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

	allRules, err := k.kubernetes.GetRulesPerPod(cacheEntry.podName, cacheEntry.podNamespace)
	if err != nil {
		return fmt.Errorf("Couldn't get the NetworkPolicies for Pod %s : %s", cacheEntry.podName, err)
	}

	// Step2: Translate all the metadata labels to Trireme Rules
	if err := createIndividualRules(containerPolicy, allRules); err != nil {
		return err
	}

	// Step3: Done
	return nil
}

// DeleteContainerPolicy deletes the container from Cache.
// TODO: Refactor so that it only returns an error. no ContainerInfo should be returned.
func (k *KubernetesPolicy) DeleteContainerPolicy(contextID string) *policy.ContainerInfo {
	_, err := k.cache.getCachedPodByContextID(contextID)
	if err != nil {
		// TODO: Return error
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
	podLabels, err := k.kubernetes.GetPodLabels(info.Config.Labels[KubernetesPodName], info.Config.Labels[KubernetesPodNamespace])
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
	glog.V(2).Infof("Update pod Policy for %s ", pod.Name)
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

// updateNamespacePolicy check if the policy for the namespace changed.
// If the policy changed, it will resync all the pods on that namespace.
func (k *KubernetesPolicy) updateNamespacePolicy(namespace *api.Namespace) error {
	annotation := namespace.GetAnnotations()
	fmt.Println(annotation[KubernetesNetworkPolicyAnnotationID])
	return nil
}

func (k *KubernetesPolicy) namespaceSync() error {
	namespaces, err := k.kubernetes.GetAllNamespaces()
	if err != nil {
		return fmt.Errorf("Couldn't get all namespaces %s ", err)
	}
	for _, namespace := range namespaces.Items {
		annotation := namespace.GetAnnotations()
		fmt.Println(annotation[KubernetesNetworkPolicyAnnotationID])
	}
	return nil
}

// Start starts the KubernetesPolicer as a daemon.
// Effectively it registers as a Watcher for policy changes.
func (k *KubernetesPolicy) Start() {
	go k.kubernetes.PolicyWatcher("", k.networkPolicyEventHandler)
	//go k.kubernetes.PodWatcher("", k.podEventHandler)
	//go k.kubernetes.NamespaceWatcher(k.namespaceHandler)
}
