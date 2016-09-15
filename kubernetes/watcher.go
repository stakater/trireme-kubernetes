package kubernetes

import (
	"fmt"

	"github.com/aporeto-inc/kubepox"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
)

// StartPolicyWatcher initiates a go routine to keep updating the Trireme policies
// If the Kubernetes Policies get updated.
func (k *KubernetesClient) StartPolicyWatcher() {
	watcher, _ := k.kubeClient.Extensions().NetworkPolicies(k.namespace).Watch(api.ListOptions{})
	fmt.Printf("%+v", watcher)
	for {
		req := <-watcher.ResultChan()

		networkPolicy := req.Object.(*extensions.NetworkPolicy)
		fmt.Println("New Policy Detected ", networkPolicy.GetName())
		allPods, err := k.kubeClient.Pods("default").List(api.ListOptions{})
		if err != nil {
			fmt.Println("panic")
		}
		affectedPods, err := kubepox.ListPodsPerPolicy(networkPolicy, allPods)
		if err != nil {
			fmt.Println("panic")
		}
		for _, pod := range affectedPods.Items {
			fmt.Println("affected pod: ", pod.Name)
		}
	}
}
