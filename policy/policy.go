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
			}
		}
	}
	return nil
}

// GetContainerPolicy returns the Policy for the targetContainers.
// The policy for the container will be based on the defined
// Kubernetes NetworkPolicies on the Pod to which the container belongs.
func (k *KubernetesPolicy) GetContainerPolicy(context string, containerPolicy *policy.ContainerInfo) error {

	podName, ok := containerPolicy.RunTime.Tags["io.kubernetes.pod.name"]
	// The container doesn't belong to Kubernetes
	if !ok {
		return fmt.Errorf("Container is not a KubernetesPODContainer")
	}

	allRules, err := k.kubernetes.GetRulesPerPod(podName)
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
	contextID := info.ID[:12]

	glog.V(2).Infof("Metadata Processor for Container: %v ", contextID)
	fmt.Println(contextID)

	container := policy.NewContainerInfo(contextID)
	container.RunTime.Pid = info.State.Pid

	if info.NetworkSettings.IPAddress == "" {
		glog.V(2).Infof("No IP Found for container. Not activating ", contextID)
	}

	container.RunTime.IPAddresses["bridge"] = info.NetworkSettings.IPAddress
	fmt.Println("IP of the container: " + container.RunTime.IPAddresses["bridge"])
	container.RunTime.Name = info.Name

	for k, v := range info.Config.Labels {
		container.RunTime.Tags[k] = v
	}

	container.RunTime.Tags["image"] = info.Config.Image
	container.RunTime.Tags["name"] = info.Name
	container.RunTime.Tags[datapath.TransmitterLabel] = contextID

	// Adding all the specific Kubernetes K,V from the Pod.
	// Iterate on PodLabels and add them as tags
	podLabels, err := k.kubernetes.GetPodLabels(info.Config.Labels["io.kubernetes.pod.name"])
	if err != nil {
		return "", nil, fmt.Errorf("Couldn't get Kubernetes labels for container %v : %v", info.Name, err)
	}
	for key, value := range podLabels {
		container.RunTime.Tags[key] = value
	}

	return contextID, container, nil
}

func (k *KubernetesPolicy) updatePodPolicy(pod *api.Pod) error {
	fmt.Println("TODO: Update Policy for ", pod.Name)
	return nil
}

// Start starts the KubernetesPolicer as a daemon.
// Effectively it registers as a Watcher for policy changes.
func (k *KubernetesPolicy) Start() {
	go k.kubernetes.StartPolicyWatcher()
}
