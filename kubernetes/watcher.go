package kubernetes

import (
	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/watch"
)

// PolicyWatcher iterates over the networkPolicyEvents. Each event generates a call to the parameter function.
func (k *Client) PolicyWatcher(namespace string, networkPolicyHandler func(event *watch.Event) error) {
	watcher, _ := k.kubeClient.Extensions().NetworkPolicies(namespace).Watch(api.ListOptions{})
	for {
		req := <-watcher.ResultChan()
		if err := networkPolicyHandler(&req); err != nil {
			glog.V(2).Infof("Error processing networkPolicyEvent : %s", err)
		}
	}
}

// PodWatcher iterates over the podEvents. Each event generates a call to the parameter function.
func (k *Client) PodWatcher(namespace string, podHandler func(event *watch.Event) error) {
	watcher, _ := k.kubeClient.Pods(namespace).Watch(api.ListOptions{})
	for {
		req := <-watcher.ResultChan()
		if err := podHandler(&req); err != nil {
			glog.V(2).Infof("Error processing podEvents : %s", err)
		}
	}
}

// NamespaceWatcher iterates over the namespaceEvents. Each event generates a call to the parameter function.
func (k *Client) NamespaceWatcher(namespaceHandler func(event *watch.Event) error) {
	watcher, _ := k.kubeClient.Namespaces().Watch(api.ListOptions{})
	for {
		req := <-watcher.ResultChan()
		if err := namespaceHandler(&req); err != nil {
			glog.V(2).Infof("Error processing namespaceEvents : %s", err)
		}
	}
}
