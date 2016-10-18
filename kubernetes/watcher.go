package kubernetes

import (
	"fmt"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/watch"
)

// PolicyWatcher iterates over the networkPolicyEvents. Each event generates a call to the parameter function.
func (c *Client) PolicyWatcher(namespace string, resultChan chan<- watch.Event, stopChan <-chan bool) error {
	for {
		watcher, err := c.kubeClient.Extensions().NetworkPolicies(namespace).Watch(api.ListOptions{})
		if err != nil {
			glog.V(4).Infof("Couldn't open the policy (ns %s)watch channel: %s", namespace, err)
			return fmt.Errorf("Couldn't open the policy (ns: %s)watch channel: %s", namespace, err)
		}
	Watch:
		for {
			select {
			case <-stopChan:
				return nil
			case req, open := <-watcher.ResultChan():
				if !open {
					glog.V(2).Infof("NetworkPolicy Watcher channel closed.")
					break Watch
				}
				glog.V(4).Infof("Adding NetworkPolicyEvent")
				resultChan <- req
			}
		}
	}
}

// LocalPodWatcher iterates over the podEvents. Each event generates a call to the parameter function.
func (c *Client) LocalPodWatcher(namespace string, resultChan chan<- watch.Event, stopChan <-chan bool) error {
	option := c.localNodeOption()
	for {
		watcher, err := c.kubeClient.Pods(namespace).Watch(option)
		if err != nil {
			glog.V(4).Infof("Couldn't open the Pod (ns:%s) watch channel: %s", namespace, err)
			return fmt.Errorf("Couldn't open the Pod (ns:%s) watch channel: %s", namespace, err)
		}
	Watch:
		for {
			select {
			case <-stopChan:
				return nil
			case req, open := <-watcher.ResultChan():
				if !open {
					glog.V(2).Infof("LocalPod Watcher channel closed.")
					break Watch
				}
				glog.V(4).Infof("Adding PodEvent")
				resultChan <- req
			}
		}
	}
}

// NamespaceWatcher iterates over the namespaceEvents. Each event generates a call to the parameter function.
func (c *Client) NamespaceWatcher(resultChan chan<- watch.Event, stopChan <-chan bool) error {
	for {
		watcher, err := c.kubeClient.Namespaces().Watch(api.ListOptions{})
		if err != nil {
			glog.V(4).Infof("Couldn't open the Namespace watch channel: %s", err)
			return fmt.Errorf("Couldn't open the Namespace watch channel: %s", err)
		}
	Watch:
		for {
			select {
			case <-stopChan:
				return nil
			case req, open := <-watcher.ResultChan():
				if !open {
					glog.V(2).Infof("NamespaceWatcher channel closed.")
					break Watch
				}
				glog.V(4).Infof("Adding NamespaceEvent")
				resultChan <- req
			}
		}
	}
}

// NodeWatcher watches new nodes and send node events on the resultChan
func (c *Client) NodeWatcher(resultChan chan<- watch.Event, stopChan <-chan bool) error {
	for {
		watcher, err := c.kubeClient.Nodes().Watch(api.ListOptions{})
		if err != nil {
			glog.V(4).Infof("Couldn't open the Node watch channel: %s", err)
			return fmt.Errorf("Couldn't open the Node watch channel: %s", err)
		}
	Watch:
		for {
			select {
			case <-stopChan:
				return nil
			case req, open := <-watcher.ResultChan():
				if !open {
					glog.V(2).Infof("NodeWatcher channel closed.")
					break Watch
				}
				glog.V(6).Infof("Adding NodeEvent")
				resultChan <- req
			}
		}
	}
}
