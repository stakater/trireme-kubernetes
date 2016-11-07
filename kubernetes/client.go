package kubernetes

import (
	"fmt"

	"github.com/aporeto-inc/kubepox"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	client "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	"k8s.io/kubernetes/pkg/fields"
)

// Client is the Trireme representation of the Client.
type Client struct {
	kubeClient *client.Clientset
	localNode  string
}

// NewClient Generate and initialize a Trireme Client object
func NewClient(kubeconfig string, namespace string, nodename string) (*Client, error) {
	Client := &Client{}
	Client.localNode = nodename

	if err := Client.InitKubernetesClient(kubeconfig); err != nil {
		return nil, fmt.Errorf("Couldn't initialize Kubernetes Client: %v", err)
	}
	return Client, nil
}

// InitKubernetesClient Initialize the Kubernetes client
func (c *Client) InitKubernetesClient(kubeconfig string) error {

	var config *restclient.Config
	var err error

	if kubeconfig == "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return fmt.Errorf("Error Building InCluster config: %v", err)
		}
	} else {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return fmt.Errorf("Error Building config from Kubeconfig: %v", err)
		}
	}

	myClient, err := client.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("Error creating REST Kube Client: %v", err)
	}
	c.kubeClient = myClient
	return nil
}

func (c *Client) localNodeOption() api.ListOptions {
	fs := fields.Set(map[string]string{
		"spec.nodeName": c.localNode,
	})
	option := api.ListOptions{
		FieldSelector: fs.AsSelector(),
	}
	return option
}

// PodRules return the list of all the IngressRules that apply to the pod.
func (c *Client) PodRules(podName string, namespace string) (*[]extensions.NetworkPolicyIngressRule, error) {
	// Step1: Get all the rules associated with this Pod.
	targetPod, err := c.kubeClient.Pods(namespace).Get(podName)
	if err != nil {
		return nil, fmt.Errorf("Couldn't get pod %v from Kubernetes API: %v", podName, err)
	}

	allPolicies, err := c.kubeClient.Extensions().NetworkPolicies(namespace).List(api.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("Couldn't list all the NetworkPolicies from Kubernetes API: %v ", err)
	}

	allRules, err := kubepox.ListIngressRulesPerPod(targetPod, allPolicies)
	if err != nil {
		return nil, fmt.Errorf("Couldn't process the list of rules for pod %v : %v", podName, err)
	}
	return allRules, nil
}

// PodLabels returns the list of all labels associated with a pod.
func (c *Client) PodLabels(podName string, namespace string) (map[string]string, error) {
	targetPod, err := c.kubeClient.Pods(namespace).Get(podName)
	if err != nil {
		return nil, fmt.Errorf("error getting Kubernetes labels for pod %v : %v ", podName, err)
	}
	return targetPod.GetLabels(), nil
}

// PodIP returns the pod's IP.
func (c *Client) PodIP(podName string, namespace string) (string, error) {
	targetPod, err := c.kubeClient.Pods(namespace).Get(podName)
	if err != nil {
		return "", fmt.Errorf("error getting Kubernetes IP for pod %v : %v ", podName, err)
	}
	return targetPod.Status.PodIP, nil
}

// PodLabelsAndIP returns the list of all labels associated with a pod as well as the Pod's IP.
func (c *Client) PodLabelsAndIP(podName string, namespace string) (map[string]string, string, error) {
	targetPod, err := c.kubeClient.Pods(namespace).Get(podName)
	if err != nil {
		return nil, "", fmt.Errorf("error getting Kubernetes labels & IP for pod %v : %v ", podName, err)
	}
	ip := targetPod.Status.PodIP
	if targetPod.Status.PodIP == targetPod.Status.HostIP {
		ip = "host"
	}
	return targetPod.GetLabels(), ip, nil
}

// LocalPods return a PodList with all the pods scheduled on the local node
func (c *Client) LocalPods(namespace string) (*api.PodList, error) {
	return c.kubeClient.Pods(namespace).List(c.localNodeOption())
}

// AllNamespaces return a list of all existing namespaces
func (c *Client) AllNamespaces() (*api.NamespaceList, error) {
	return c.kubeClient.Namespaces().List(api.ListOptions{})
}

// AddLocalNodeAnnotation adds the annotationKey:annotationValue
func (c *Client) AddLocalNodeAnnotation(annotationKey, annotationValue string) error {
	nodeName := c.localNode
	node, err := c.kubeClient.Nodes().Get(nodeName)
	if err != nil {
		return fmt.Errorf("Couldn't get node %s: %s", nodeName, err)
	}

	annotations := node.GetAnnotations()
	annotations[annotationKey] = annotationValue
	node.SetAnnotations(annotations)
	_, err = c.kubeClient.Nodes().Update(node)
	if err != nil {
		return fmt.Errorf("Error updating Annotations for node %s: %s", nodeName, err)
	}
	return nil
}

// AllNodes return a list of all the nodes on the KubeCluster.
func (c *Client) AllNodes() (*api.NodeList, error) {
	nodes, err := c.kubeClient.Nodes().List(api.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("Couldn't get nodes list : %s", err)
	}
	return nodes, nil
}
