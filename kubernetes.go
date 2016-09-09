package main

// This has to be refactored soon enough. For now, just using a simple
// Proof of concept of an example Kubernetes integration.
// This is based on the original example code from trireme/example/example.go

import (
	"flag"
	"fmt"
	"os"
	"sync"

	"github.com/aporeto-inc/kubepox"
	"github.com/aporeto-inc/trireme"
	"github.com/aporeto-inc/trireme/datapath"
	"github.com/aporeto-inc/trireme/policy"

	"github.com/docker/engine-api/types"
	"k8s.io/kubernetes/pkg/api"
	apiu "k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/extensions"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
)

// KubernetesPolicy holds the configuration of the policy engine
type KubernetesPolicy struct {
	cache      map[string]*policy.ContainerInfo
	kubeClient *client.Client
}

// Keep an open ready KubeClient
func (k *KubernetesPolicy) initKubernetesClient(kubeConfig string) error {

	config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		fmt.Printf("Error opening Kubeconfig: %v\n", err)
		os.Exit(1)
	}

	myClient, err := client.New(config)
	if err != nil {
		fmt.Printf("Error creating REST Kube Client: %v\n", err)
		os.Exit(1)
	}
	k.kubeClient = myClient
	return nil
}

// NewPolicyEngine creates a new policy engine for the Trireme package
func NewPolicyEngine() *KubernetesPolicy {
	return &KubernetesPolicy{cache: map[string]*policy.ContainerInfo{}}
}

// Righ now only focus on label base rules...
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

// GetContainerPolicy implements the Trireme interface. Here we just create a simple
// policy that accepts packets with the same labels as the target container.
func (k *KubernetesPolicy) GetContainerPolicy(context string, containerPolicy *policy.ContainerInfo) error {

	// Step1: Get all the rules associated with this Pod.
	targetPod, err := k.kubeClient.Pods("default").Get(containerPolicy.RunTime.Tags["io.kubernetes.pod.name"])
	if err != nil {
		return err
	}

	allPolicies, err := k.kubeClient.Extensions().NetworkPolicies("default").List(api.ListOptions{})
	if err != nil {
		return err
	}

	allRules, err := kubepox.ListIngressRulesPerPod(targetPod, allPolicies)
	if err != nil {
		return err
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
	fmt.Println("Metadata Extractor")

	contextID := info.ID[:12]

	fmt.Println(contextID)

	container := policy.NewContainerInfo(contextID)
	container.RunTime.Pid = info.State.Pid

	container.RunTime.IPAddresses["bridge"] = info.NetworkSettings.IPAddress
	container.RunTime.Name = info.Name

	for k, v := range info.Config.Labels {
		container.RunTime.Tags[k] = v
	}

	container.RunTime.Tags["image"] = info.Config.Image
	container.RunTime.Tags["name"] = info.Name
	container.RunTime.Tags[datapath.TransmitterLabel] = contextID

	// Adding all the specific Kubernetes K,V from the Pod.
	targetPod, err := k.kubeClient.Pods("default").Get(info.Config.Labels["io.kubernetes.pod.name"])
	if err != nil {
		fmt.Println("error getting KubeLabels: " + info.Config.Labels["io.kubernetes.pod.name"])
		return "", nil, err
	}
	// Iterate on PodLabels and add them as tags
	for key, value := range targetPod.GetLabels() {
		container.RunTime.Tags[key] = value
	}

	return contextID, container, nil
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: example -stderrthreshold=[INFO|WARN|FATAL] -log_dir=[string]\n")
	flag.PrintDefaults()
	os.Exit(2)
}

func init() {
	flag.Usage = usage
	// NOTE: This next line is key you have to call flag.Parse() for the command line
	// options or "flags" that are defined in the glog module to be picked up.
	flag.Parse()
}

func main() {
	var wg sync.WaitGroup

	networks := []string{"0.0.0.0/0"}

	policyEngine := NewPolicyEngine()
	// Get location of the Kubeconfig file. By default in your home.
	kubeconfig := os.Getenv("HOME") + "/.kube/config"

	policyEngine.initKubernetesClient(kubeconfig)

	isolator := trireme.NewIsolator(networks, policyEngine)

	wg.Add(1)
	isolator.Start()
	wg.Wait()
}
