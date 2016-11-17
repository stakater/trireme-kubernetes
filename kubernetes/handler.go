package kubernetes

import (
	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/runtime"
)

// CreateResourceController creates a controller for a specific ressource and namespace.
// Input
func CreateResourceController(client cache.Getter, resource string, namespace string, apiStruct runtime.Object,
	addFunc func(addedApiStruct interface{}), deleteFunc func(deletedApiStruct interface{}), updateFunc func(oldApiStruct, updatedApiStruct interface{})) (cache.Store, *cache.Controller) {
	handlers := cache.ResourceEventHandlerFuncs{
		AddFunc:    addFunc,
		DeleteFunc: deleteFunc,
		UpdateFunc: updateFunc,
	}

	listWatch := cache.NewListWatchFromClient(client, resource, namespace, fields.Everything())
	store, controller := cache.NewInformer(listWatch, apiStruct, 0, handlers)
	return store, controller

}

// CreateNamespaceController creates a controller specifically for Namespaces.
func CreateNamespaceController(client cache.Getter,
	addFunc func(addedApiStruct *api.Namespace) error, deleteFunc func(deletedApiStruct *api.Namespace) error, updateFunc func(oldApiStruct, updatedApiStruct *api.Namespace) error) (cache.Store, *cache.Controller) {
	return CreateResourceController(client, "namespaces", "", &api.Namespace{},
		func(addedApiStruct interface{}) {
			if err := addFunc(addedApiStruct.(*api.Namespace)); err != nil {
				glog.V(2).Infof("Error while handling Add NameSpace: %s ", err)
			}
		},
		func(deletedApiStruct interface{}) {
			if err := deleteFunc(deletedApiStruct.(*api.Namespace)); err != nil {
				glog.V(2).Infof("Error while handling Delete NameSpace: %s ", err)

			}
		},
		func(oldApiStruct, updatedApiStruct interface{}) {
			if err := updateFunc(oldApiStruct.(*api.Namespace), updatedApiStruct.(*api.Namespace)); err != nil {
				glog.V(2).Infof("Error while handling Update NameSpace: %s ", err)

			}
		})
}

// CreatePodController creates a controller specifically for Pods.
func CreatePodController(client cache.Getter, namespace string,
	addFunc func(addedApiStruct *api.Pod) error, deleteFunc func(deletedApiStruct *api.Pod) error, updateFunc func(oldApiStruct, updatedApiStruct *api.Pod) error) (cache.Store, *cache.Controller) {
	return CreateResourceController(client, "pod", namespace, &api.Pod{},
		func(addedApiStruct interface{}) {
			if err := addFunc(addedApiStruct.(*api.Pod)); err != nil {
				glog.V(2).Infof("Error while handling Add Pod: %s ", err)
			}
		},
		func(deletedApiStruct interface{}) {
			if err := deleteFunc(deletedApiStruct.(*api.Pod)); err != nil {
				glog.V(2).Infof("Error while handling Delete Pod: %s ", err)
			}
		},
		func(oldApiStruct, updatedApiStruct interface{}) {
			if err := updateFunc(oldApiStruct.(*api.Pod), updatedApiStruct.(*api.Pod)); err != nil {
				glog.V(2).Infof("Error while handling Update Pod: %s ", err)
			}
		})
}

// CreateNetworkPoliciesController creates a controller specifically for NetworkPolicies.
func CreateNetworkPoliciesController(client cache.Getter, namespace string,
	addFunc func(addedApiStruct *extensions.NetworkPolicy) error, deleteFunc func(deletedApiStruct *extensions.NetworkPolicy) error, updateFunc func(oldApiStruct, updatedApiStruct *extensions.NetworkPolicy) error) (cache.Store, *cache.Controller) {
	return CreateResourceController(client, "networkpolicies", namespace, &extensions.NetworkPolicy{},
		func(addedApiStruct interface{}) {
			if err := addFunc(addedApiStruct.(*extensions.NetworkPolicy)); err != nil {
				glog.V(2).Infof("Error while handling Add NetworkPolicy: %s ", err)
			}
		},
		func(deletedApiStruct interface{}) {
			if err := deleteFunc(deletedApiStruct.(*extensions.NetworkPolicy)); err != nil {
				glog.V(2).Infof("Error while handling Delete NetworkPolicy: %s ", err)
			}
		},
		func(oldApiStruct, updatedApiStruct interface{}) {
			if err := updateFunc(oldApiStruct.(*extensions.NetworkPolicy), updatedApiStruct.(*extensions.NetworkPolicy)); err != nil {
				glog.V(2).Infof("Error while handling Update NetworkPolicy: %s ", err)
			}
		})
}
