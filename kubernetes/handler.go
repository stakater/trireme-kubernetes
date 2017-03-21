package kubernetes

import (
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	api "k8s.io/client-go/pkg/api/v1"
	extensions "k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
)

const errorLogLevel = 2

// CreateResourceController creates a controller for a specific ressource and namespace.
// The parameter function will be called on Add/Delete/Update events
func CreateResourceController(client cache.Getter, resource string, namespace string, apiStruct runtime.Object, selector fields.Selector,
	addFunc func(addedApiStruct interface{}), deleteFunc func(deletedApiStruct interface{}), updateFunc func(oldApiStruct, updatedApiStruct interface{})) (cache.Store, cache.Controller) {

	handlers := cache.ResourceEventHandlerFuncs{
		AddFunc:    addFunc,
		DeleteFunc: deleteFunc,
		UpdateFunc: updateFunc,
	}

	listWatch := cache.NewListWatchFromClient(client, resource, namespace, selector)
	store, controller := cache.NewInformer(listWatch, apiStruct, 0, handlers)
	return store, controller
}

// CreateNamespaceController creates a controller specifically for Namespaces.
func (c *Client) CreateNamespaceController(
	addFunc func(addedApiStruct *api.Namespace) error, deleteFunc func(deletedApiStruct *api.Namespace) error, updateFunc func(oldApiStruct, updatedApiStruct *api.Namespace) error) (cache.Store, cache.Controller) {

	return CreateResourceController(c.KubeClient().Core().RESTClient(), "namespaces", "", &api.Namespace{}, fields.Everything(),
		func(addedApiStruct interface{}) {
			if err := addFunc(addedApiStruct.(*api.Namespace)); err != nil {
				glog.V(errorLogLevel).Infof("Error while handling Add NameSpace: %s ", err)
			}
		},
		func(deletedApiStruct interface{}) {
			if err := deleteFunc(deletedApiStruct.(*api.Namespace)); err != nil {
				glog.V(errorLogLevel).Infof("Error while handling Delete NameSpace: %s ", err)

			}
		},
		func(oldApiStruct, updatedApiStruct interface{}) {
			if err := updateFunc(oldApiStruct.(*api.Namespace), updatedApiStruct.(*api.Namespace)); err != nil {
				glog.V(errorLogLevel).Infof("Error while handling Update NameSpace: %s ", err)

			}
		})
}

// CreateLocalPodController creates a controller specifically for Pods.
func (c *Client) CreateLocalPodController(namespace string,
	addFunc func(addedApiStruct *api.Pod) error, deleteFunc func(deletedApiStruct *api.Pod) error, updateFunc func(oldApiStruct, updatedApiStruct *api.Pod) error) (cache.Store, cache.Controller) {

	return CreateResourceController(c.KubeClient().Core().RESTClient(), "pods", namespace, &api.Pod{}, c.localNodeSelector(),
		func(addedApiStruct interface{}) {
			if err := addFunc(addedApiStruct.(*api.Pod)); err != nil {
				glog.V(errorLogLevel).Infof("Error while handling Add Pod: %s ", err)
			}
		},
		func(deletedApiStruct interface{}) {
			if err := deleteFunc(deletedApiStruct.(*api.Pod)); err != nil {
				glog.V(errorLogLevel).Infof("Error while handling Delete Pod: %s ", err)
			}
		},
		func(oldApiStruct, updatedApiStruct interface{}) {
			if err := updateFunc(oldApiStruct.(*api.Pod), updatedApiStruct.(*api.Pod)); err != nil {
				glog.V(errorLogLevel).Infof("Error while handling Update Pod: %s ", err)
			}
		})
}

// CreateNetworkPoliciesController creates a controller specifically for NetworkPolicies.
func (c *Client) CreateNetworkPoliciesController(namespace string,
	addFunc func(addedApiStruct *extensions.NetworkPolicy) error, deleteFunc func(deletedApiStruct *extensions.NetworkPolicy) error, updateFunc func(oldApiStruct, updatedApiStruct *extensions.NetworkPolicy) error) (cache.Store, cache.Controller) {
	return CreateResourceController(c.KubeClient().Extensions().RESTClient(), "networkpolicies", namespace, &extensions.NetworkPolicy{}, fields.Everything(),
		func(addedApiStruct interface{}) {
			if err := addFunc(addedApiStruct.(*extensions.NetworkPolicy)); err != nil {
				glog.V(errorLogLevel).Infof("Error while handling Add NetworkPolicy: %s ", err)
			}
		},
		func(deletedApiStruct interface{}) {
			if err := deleteFunc(deletedApiStruct.(*extensions.NetworkPolicy)); err != nil {
				glog.V(errorLogLevel).Infof("Error while handling Delete NetworkPolicy: %s ", err)
			}
		},
		func(oldApiStruct, updatedApiStruct interface{}) {
			if err := updateFunc(oldApiStruct.(*extensions.NetworkPolicy), updatedApiStruct.(*extensions.NetworkPolicy)); err != nil {
				glog.V(errorLogLevel).Infof("Error while handling Update NetworkPolicy: %s ", err)
			}
		})
}

// CreateNodeController creates a controller specifically for Nodes.
func (c *Client) CreateNodeController(
	addFunc func(addedApiStruct *api.Node) error, deleteFunc func(deletedApiStruct *api.Node) error, updateFunc func(oldApiStruct, updatedApiStruct *api.Node) error) (cache.Store, cache.Controller) {
	return CreateResourceController(c.KubeClient().Core().RESTClient(), "nodes", "", &api.Node{}, fields.Everything(),
		func(addedApiStruct interface{}) {
			if err := addFunc(addedApiStruct.(*api.Node)); err != nil {
				glog.V(errorLogLevel).Infof("Error while handling Add Node: %s ", err)
			}
		},
		func(deletedApiStruct interface{}) {
			if err := deleteFunc(deletedApiStruct.(*api.Node)); err != nil {
				glog.V(errorLogLevel).Infof("Error while handling Delete Node: %s ", err)
			}
		},
		func(oldApiStruct, updatedApiStruct interface{}) {
			if err := updateFunc(oldApiStruct.(*api.Node), updatedApiStruct.(*api.Node)); err != nil {
				glog.V(errorLogLevel).Infof("Error while handling Update Node: %s ", err)
			}
		})
}

// CreateServiceController creates a controller specifically for Services.
func (c *Client) CreateServiceController(namespace string,
	addFunc func(addedApiStruct *api.Service) error, deleteFunc func(deletedApiStruct *api.Service) error, updateFunc func(oldApiStruct, updatedApiStruct *api.Service) error) (cache.Store, cache.Controller) {
	return CreateResourceController(c.KubeClient().Core().RESTClient(), "services", "", &api.Service{}, fields.Everything(),
		func(addedApiStruct interface{}) {
			if err := addFunc(addedApiStruct.(*api.Service)); err != nil {
				glog.V(errorLogLevel).Infof("Error while handling Add service: %s ", err)
			}
		},
		func(deletedApiStruct interface{}) {
			if err := deleteFunc(deletedApiStruct.(*api.Service)); err != nil {
				glog.V(errorLogLevel).Infof("Error while handling Delete service: %s ", err)
			}
		},
		func(oldApiStruct, updatedApiStruct interface{}) {
			if err := updateFunc(oldApiStruct.(*api.Service), updatedApiStruct.(*api.Service)); err != nil {
				glog.V(errorLogLevel).Infof("Error while handling Update service: %s ", err)
			}
		})
}
