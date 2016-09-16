package kubernetes

import (
	"fmt"

	"github.com/aporeto-inc/kubepox"
	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
)

// StartPolicyWatcher initiates a go routine to keep updating the Trireme policies
// If the Kubernetes Policies get updated.
func (k *KubernetesClient) StartPolicyWatcher(UpdatedPodPolicy func(pod *api.Pod) error) {
	watcher, _ := k.kubeClient.Extensions().NetworkPolicies(k.namespace).Watch(api.ListOptions{})
	fmt.Printf("%+v", watcher)
	for {
		req := <-watcher.ResultChan()

		networkPolicy := req.Object.(*extensions.NetworkPolicy)
		glog.V(2).Infof("New K8S NetworkPolicy change detected: %s", networkPolicy.GetName())
		allPods, err := k.kubeClient.Pods("default").List(api.ListOptions{})
		if err != nil {
			glog.V(2).Infof("Couldn't get all pods for policy: %s", networkPolicy.GetName())
		}
		affectedPods, err := kubepox.ListPodsPerPolicy(networkPolicy, allPods)
		if err != nil {
			glog.V(2).Infof("Couldn't get all pods for policy: %s", networkPolicy.GetName())
		}

		//Reresolve all affected pods
		for _, pod := range affectedPods.Items {
			fmt.Println("affected pod: ", pod.Name)
			UpdatedPodPolicy(&pod)
		}
	}
}
