package resolver

import (
	"fmt"

	extensions "k8s.io/client-go/pkg/apis/extensions/v1beta1"
	kubecache "k8s.io/client-go/tools/cache"
)

// NamespaceWatcher implements the policy for a specific Namespace
type NamespaceWatcher struct {
	namespace            string
	podStore             kubecache.Store
	podController        kubecache.Controller
	podControllerStop    chan struct{}
	policyStore          kubecache.Store
	policyController     kubecache.Controller
	policyControllerStop chan struct{}
}

// NewNamespaceWatcher initialize a new NamespaceWatcher that watches the Pod and
// Networkpolicy events on the specific namespace passed in parameter.
func NewNamespaceWatcher(namespace string, podStore kubecache.Store, podController kubecache.Controller, podControllerStop chan struct{},
	policyStore kubecache.Store, policyController kubecache.Controller, policyControllerStop chan struct{}) *NamespaceWatcher {

	namespaceWatcher := &NamespaceWatcher{
		namespace:            namespace,
		podStore:             podStore,
		podController:        podController,
		podControllerStop:    podControllerStop,
		policyStore:          policyStore,
		policyController:     policyController,
		policyControllerStop: policyControllerStop,
	}

	return namespaceWatcher
}

func (n *NamespaceWatcher) stopWatchingNamespace() {
	n.podControllerStop <- struct{}{}
	n.policyControllerStop <- struct{}{}
}

// getPolicyList returns the list of Policy based on what is in the store
// at that time for this namespace.
//
// TODO: Eventually go back to an API call to Kubernetes API.
//
func (n *NamespaceWatcher) getPolicyList() (*extensions.NetworkPolicyList, error) {
	if n.policyStore == nil {
		return nil, fmt.Errorf("PolicyStore not initialized correctly")
	}

	storeList := n.policyStore.List()

	// Copy and cast all the store objects to a NetworkPolicyList object
	networkPolicyList := extensions.NetworkPolicyList{}
	networkPolicyList.Items = make([]extensions.NetworkPolicy, len(storeList))

	for _, policy := range storeList {
		networkPolicyList.Items = append(networkPolicyList.Items, *(policy.(*extensions.NetworkPolicy)))
	}

	return &networkPolicyList, nil
}
