package policy

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

	apiu "k8s.io/kubernetes/pkg/api/unversioned"
)

// KubernetesPodName is the label used by Docker for the K8S pod name
const KubernetesPodName = "io.kubernetes.pod.name"

// KubernetesPodNamespace is the label used by Docker for the K8s namespace
const KubernetesPodNamespace = "io.kubernetes.pod.namespace"

// KubernetesPolicy represents a Trireme Policer for Kubernetes.
// It implements the Trireme Resolver interface and implements the policies defined
// by Kubernetes NetworkPolicy API.
type KubernetesPolicy struct {
	cache      map[string]*policy.ContainerInfo
	isolator   trireme.Isolator
	kubernetes *kubernetes.KubernetesClient
}

// NewKubernetesPolicy creates a new policy engine for the Trireme package
func NewKubernetesPolicy(kubeconfig string, namespace string) (*KubernetesPolicy, error) {
	client, err := kubernetes.NewKubernetesClient(kubeconfig, namespace)
	if err != nil {
		return nil, fmt.Errorf("Couldn't create KubernetesClient: %v ", err)
	}

	return &KubernetesPolicy{
		cache:      map[string]*policy.ContainerInfo{},
		kubernetes: client,
	}, nil
}

// RegisterIsolator keeps a reference to the Isolator for Callbacks.
// If an isolator is already registered, this one will override the existing reference
func (k *KubernetesPolicy) RegisterIsolator(isolator trireme.Isolator) {
	k.isolator = isolator
}

// Right now only focus on label base rules...
func createIndividualRules(req *policy.ContainerInfo, allRules *[]extensions.NetworkPolicyIngressRule) error {
	for _, rule := range *allRules {
		for _, from := range rule.From {
			labelsKeyValue, err := apiu.LabelSelectorAsMap(from.PodSelector)
			if err != nil {
				return err
			}
			for key, value := range labelsKeyValue {
				req.Policy.Rules.AddElements([]string{key + "=" + value}, "accept")
				fmt.Println(key, "   ", value)
			}
		}
	}
	return nil
}

// GetContainerPolicy returns the Policy for the targetContainers.
// The policy for the container will be based on the defined
// Kubernetes NetworkPolicies on the Pod to which the container belongs.
func (k *KubernetesPolicy) GetContainerPolicy(context string, containerPolicy *policy.ContainerInfo) error {

	fmt.Println("GetContainerPolicy")
	podName, ok := containerPolicy.RunTime.Tags[KubernetesPodName]
	// The container doesn't belong to Kubernetes
	if !ok {
		return fmt.Errorf("Container is not a KubernetesPODContainer")
	}
	namespace, _ := containerPolicy.RunTime.Tags[KubernetesPodNamespace]

	allRules, err := k.kubernetes.GetRulesPerPod(podName, namespace)
	if err != nil {
		return fmt.Errorf("Couldn't process the pod %v through the KubernetesPolicies: %v", podName, err)
	}

	// Step2: Translate all the metadata labels to Trireme Rules
	if err := createIndividualRules(containerPolicy, allRules); err != nil {
		return err
	}

	// Step3: Done
	k.cache[context] = containerPolicy
	return nil
}

// DeleteContainerPolicy implements the corresponding interface. We have no
// state in this example
func (k *KubernetesPolicy) DeleteContainerPolicy(context string) *policy.ContainerInfo {
	return k.cache[context]
}

// MetadataExtractor implements the extraction of metadata from the Docker data
func (k *KubernetesPolicy) MetadataExtractor(info *types.ContainerJSON) (string, *policy.ContainerInfo, error) {
	containerName := info.Name
	containerID := info.ID
	_, ok := info.Config.Labels[KubernetesPodName]
	if !ok {
		glog.V(2).Infof("No podName Found for container [%s]%s. Must not be K8S Pod Container. Not activating ", containerName, containerID)
		return "", nil, nil
	}
	_, ok = info.Config.Labels[KubernetesPodNamespace]
	if !ok {
		glog.V(2).Infof("No podNamespace Found for container [%s]%s. Must not be K8S Pod Container. Not activating ", containerName, containerID)
		return "", nil, nil
	}
	contextID := containerID[:12]

	glog.V(2).Infof("Processing Metadata for Docker Container: [%s]%s", containerName, containerID)

	container := policy.NewContainerInfo(contextID)
	container.RunTime.Pid = info.State.Pid

	if info.NetworkSettings.IPAddress == "" {
		glog.V(2).Infof("No IP Found for container [%s]%s. Must not be K8S Pod Container. Not activating ", containerName, containerID)
		return "", nil, nil
	}

	container.RunTime.IPAddresses["bridge"] = info.NetworkSettings.IPAddress
	container.RunTime.Name = info.Name

	//TODO: Refactor to only include the ACTUAL labels. Everything else should be outside
	container.RunTime.Tags[KubernetesPodName] = info.Config.Labels[KubernetesPodName]
	container.RunTime.Tags[KubernetesPodNamespace] = info.Config.Labels[KubernetesPodNamespace]
	container.RunTime.Tags["name"] = info.Name
	container.RunTime.Tags[datapath.TransmitterLabel] = contextID

	// Adding all the specific Kubernetes K,V from the Pod.
	// Iterate on PodLabels and add them as tags
	podLabels, err := k.kubernetes.GetPodLabels(info.Config.Labels[KubernetesPodName], info.Config.Labels[KubernetesPodNamespace])
	if err != nil {
		return "", nil, fmt.Errorf("Couldn't get Kubernetes labels for container [%s]%s : %v", containerName, containerID, err)
	}
	for key, value := range podLabels {
		container.RunTime.Tags[key] = value
	}
	return contextID, container, nil
}

// UpdatePodPolicy updates (replace) the policy of the pod given in parameter.
func (k *KubernetesPolicy) UpdatePodPolicy(pod *api.Pod) error {
	fmt.Println("TODO: Update Policy for ", pod.Name)
	return nil
}

// Start starts the KubernetesPolicer as a daemon.
// Effectively it registers as a Watcher for policy changes.
func (k *KubernetesPolicy) Start() {
	go k.kubernetes.StartPolicyWatcher(k.UpdatePodPolicy)
}
