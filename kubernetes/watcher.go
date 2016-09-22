package kubernetes

import (
	"fmt"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/watch"
)

// PolicyWatcher iterates over the networkPolicyEvents. Each event generates a call to the parameter function.
func (k *Client) PolicyWatcher(namespace string, networkPolicyHandler func(event *watch.Event) error) error {
	watcher, err := k.kubeClient.Extensions().NetworkPolicies(namespace).Watch(api.ListOptions{})
	if err != nil {
		return fmt.Errorf("Couldn't open the Policy watch channel: %s", err)
	}
	for {
		req, open := <-watcher.ResultChan()
		if !open {
			glog.V(2).Infof("Error processing networkPolicyEvent : %s", err)
		}
		if err := networkPolicyHandler(&req); err != nil {
			glog.V(2).Infof("Error processing networkPolicyEvent : %s", err)
		}
	}
}

// LocalPodWatcher iterates over the podEvents. Each event generates a call to the parameter function.
func (k *Client) LocalPodWatcher(namespace string, podHandler func(event *watch.Event) error) error {

	// Watching Pods on the localnode only
	fs := fields.Set(map[string]string{
		"spec.nodeName": k.localNode,
	})
	option := api.ListOptions{
		FieldSelector: fs.AsSelector(),
	}

	watcher, err := k.kubeClient.Pods(namespace).Watch(option)
	if err != nil {
		return fmt.Errorf("Couldn't open the Pod watch channel: %s", err)
	}
	for {
		req, open := <-watcher.ResultChan()
		if !open {
			glog.V(2).Infof("Error processing podEvents : %s", err)
		}
		if err := podHandler(&req); err != nil {
			glog.V(2).Infof("Error processing podEvents : %s", err)
		}
	}
}

// NamespaceWatcher iterates over the namespaceEvents. Each event generates a call to the parameter function.
func (k *Client) NamespaceWatcher(namespaceHandler func(event *watch.Event) error) error {
	watcher, err := k.kubeClient.Namespaces().Watch(api.ListOptions{})
	if err != nil {
		return fmt.Errorf("Couldn't open the Namespace watch channel: %s", err)
	}
	for {
		req, open := <-watcher.ResultChan()
		if !open {
			glog.V(2).Infof("Error processing namespaceEvents : %s", err)
		}
		if err := namespaceHandler(&req); err != nil {
			glog.V(2).Infof("Error processing namespaceEvents : %s", err)
		}
	}
}
