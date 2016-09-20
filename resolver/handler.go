package resolver

import (
	"fmt"

	"github.com/aporeto-inc/kubepox"
	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/watch"
)

// networkPolicyEventHandler handle the networkPolicy Events
func (k *KubernetesPolicy) networkPolicyEventHandler(event *watch.Event) error {
	switch event.Type {
	case watch.Added, watch.Deleted, watch.Modified:

		networkPolicy, success := event.Object.(*extensions.NetworkPolicy)
		if !success {
			return fmt.Errorf("Couldn't decode the networkPolicyObject for event: %s ", event.Type)
		}
		glog.V(2).Infof("New K8S NetworkPolicy change detected: %s namespace: %s", networkPolicy.GetName(), networkPolicy.GetNamespace())

		// TODO: Filter on pods from localNode only.
		allPods, err := k.kubernetes.GetLocalPods(networkPolicy.Namespace)
		if err != nil {
			glog.V(2).Infof("Couldn't get all pods for policy: %s", networkPolicy.GetName())
		}
		affectedPods, err := kubepox.ListPodsPerPolicy(networkPolicy, allPods)
		if err != nil {
			glog.V(2).Infof("Couldn't get all pods for policy: %s", networkPolicy.GetName())
		}
		//Reresolve all affected pods
		for _, pod := range affectedPods.Items {
			glog.V(2).Infof("affected pod: %s", pod.Name)
			k.updatePodPolicy(&pod)
		}

	case watch.Error:
		return fmt.Errorf("Error on networkPolicy event channel ")
	}
	return nil
}

// podEventHandler handles the pod Events.
func (k *KubernetesPolicy) podEventHandler(event *watch.Event) error {
	switch event.Type {
	case watch.Added, watch.Deleted, watch.Modified:

		pod, success := event.Object.(*api.Pod)
		if !success {
			return fmt.Errorf("Couldn't decode the pod Object for event: %s ", event.Type)
		}
		glog.V(2).Infof("New K8S pod change detected: %s namespace: %s", pod.GetName(), pod.GetNamespace())

	case watch.Error:
		return fmt.Errorf("Error on pod event channel ")
	}
	return nil
}

// namespaceHandler handles the namespace events
func (k *KubernetesPolicy) namespaceHandler(event *watch.Event) error {
	switch event.Type {
	case watch.Added, watch.Modified, watch.Deleted:
		namespace, success := event.Object.(*api.Namespace)
		if !success {
			return fmt.Errorf("Couldn't decode the namespace Object for event: %s ", event.Type)
		}
		glog.V(2).Infof("New K8S namespace change detected: %s ", namespace.GetName())

	case watch.Error:
		return fmt.Errorf("Error on namespace event channel ")
	}
	return nil
}
