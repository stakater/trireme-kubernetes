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
func (k *KubernetesPolicy) networkPolicyEventHandler(networkPolicy *extensions.NetworkPolicy, eventType watch.EventType) error {
	switch eventType {
	case watch.Added, watch.Deleted, watch.Modified:

		glog.V(5).Infof("New K8S NetworkPolicy change detected: %s namespace: %s", networkPolicy.GetName(), networkPolicy.GetNamespace())

		// TODO: Filter on pods from localNode only.
		allPods, err := k.Kubernetes.LocalPods(networkPolicy.Namespace)
		if err != nil {
			return fmt.Errorf("Couldn't get all local pods: %s", err)
		}
		affectedPods, err := kubepox.ListPodsPerPolicy(networkPolicy, allPods)
		if err != nil {
			return fmt.Errorf("Couldn't get all pods for policy: %s , %s ", networkPolicy.GetName(), err)
		}
		//Reresolve all affected pods
		for _, pod := range affectedPods.Items {
			glog.V(5).Infof("affected pod: %s", pod.Name)
			err := k.updatePodPolicy(&pod)
			if err != nil {
				return fmt.Errorf("UpdatePolicy failed: %s", err)
			}
		}

	case watch.Error:
		return fmt.Errorf("Error on networkPolicy event channel ")
	}
	return nil
}

// podEventHandler handles the pod Events.
func (k *KubernetesPolicy) podEventHandler(pod *api.Pod, eventType watch.EventType) error {
	switch eventType {
	case watch.Added:
		glog.V(5).Infof("New K8S pod Added detected: %s namespace: %s", pod.GetName(), pod.GetNamespace())
	case watch.Deleted:
		glog.V(5).Infof("New K8S pod Deleted detected: %s namespace: %s", pod.GetName(), pod.GetNamespace())
		err := k.cache.deleteFromCacheByPodName(pod.GetName(), pod.GetNamespace())
		if err != nil {
			return fmt.Errorf("Error for PodDelete: %s ", err)
		}
		/*	case watch.Modified:
			glog.V(5).Infof("New K8S pod Modified detected: %s namespace: %s", pod.GetName(), pod.GetNamespace())
			err := k.updatePodPolicy(pod)
			if err != nil {
				return fmt.Errorf("Failed UpdatePolicy on ModifiedPodEvent: %s", err)
			}
		*/
	case watch.Error:
		return fmt.Errorf("Error on pod event channel ")
	}
	return nil
}
