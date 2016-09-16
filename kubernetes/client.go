package kubernetes

import (
	"fmt"

	"github.com/aporeto-inc/kubepox"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
)

// KubernetesClient is the Trireme representation of the KubernetesClient.
type KubernetesClient struct {
	kubeClient *client.Client
	namespace  string
}

// NewKubernetesClient Generate and initialize a Trireme KubernetesClient object
func NewKubernetesClient(kubeconfig string, namespace string) (*KubernetesClient, error) {
	kubernetesClient := &KubernetesClient{}
	err := kubernetesClient.InitKubernetesClient(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("Coultn't initialize Kubernetes Client: %v", err)
	}
	return kubernetesClient, nil
}

// InitKubernetesClient Initialize the Kubernetes client
func (k *KubernetesClient) InitKubernetesClient(kubeconfig string) error {

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return fmt.Errorf("Error opening Kubeconfig: %v", err)
	}

	myClient, err := client.New(config)
	if err != nil {
		return fmt.Errorf("Error creating REST Kube Client: %v", err)
	}
	k.kubeClient = myClient
	return nil
}

// GetRulesPerPod return the list of all the IngressRules that apply to the pod.
func (k *KubernetesClient) GetRulesPerPod(podName string, namespace string) (*[]extensions.NetworkPolicyIngressRule, error) {
	// Step1: Get all the rules associated with this Pod.
	targetPod, err := k.kubeClient.Pods(namespace).Get(podName)
	if err != nil {
		return nil, fmt.Errorf("Couldn't get pod %v from Kubernetes API: %v", podName, err)
	}

	allPolicies, err := k.kubeClient.Extensions().NetworkPolicies(namespace).List(api.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("Couldn't list all the NetworkPolicies from Kubernetes API: %v ", err)
	}

	allRules, err := kubepox.ListIngressRulesPerPod(targetPod, allPolicies)
	if err != nil {
		return nil, fmt.Errorf("Couldn't process the list of rules for pod %v : %v", podName, err)
	}
	return allRules, nil
}

// GetPodLabels returns the list of all label associated with a pod.
func (k *KubernetesClient) GetPodLabels(podName string, namespace string) (map[string]string, error) {
	targetPod, err := k.kubeClient.Pods(namespace).Get(podName)
	if err != nil {
		return nil, fmt.Errorf("error getting Kubernetes labels for pod %v : %v ", podName, err)
	}
	return targetPod.GetLabels(), nil
}
