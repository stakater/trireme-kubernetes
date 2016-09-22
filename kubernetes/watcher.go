package kubernetes

import (
	"fmt"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/watch"
)

// PolicyWatcher iterates over the networkPolicyEvents. Each event generates a call to the parameter function.
func (c *Client) PolicyWatcher(namespace string, networkPolicyHandler func(event *watch.Event) error) error {
	watcher, err := c.kubeClient.Extensions().NetworkPolicies(namespace).Watch(api.ListOptions{})
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
func (c *Client) LocalPodWatcher(namespace string, podHandler func(event *watch.Event) error) error {
	option := c.localNodeOption()
	watcher, err := c.kubeClient.Pods(namespace).Watch(option)
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
func (c *Client) NamespaceWatcher(namespaceHandler func(event *watch.Event) error) error {
	watcher, err := c.kubeClient.Namespaces().Watch(api.ListOptions{})
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
