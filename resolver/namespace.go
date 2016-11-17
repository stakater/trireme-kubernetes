package resolver

import kubecache "k8s.io/kubernetes/pkg/client/cache"

// NamespaceWatcher implements the policy for a specific Namespace
type NamespaceWatcher struct {
	namespace            string
	podController        *kubecache.Controller
	podControllerStop    chan struct{}
	policyController     *kubecache.Controller
	policyControllerStop chan struct{}
}

// NewNamespaceWatcher initialize a new NamespaceWatcher that watches the Pod and
// Networkpolicy events on the specific namespace passed in parameter.
func NewNamespaceWatcher(namespace string, podController *kubecache.Controller, podControllerStop chan struct{},
	policyController *kubecache.Controller, policyControllerStop chan struct{}) *NamespaceWatcher {
	// Creating all the channels for the Subwatchers.
	namespaceWatcher := &NamespaceWatcher{
		namespace:            namespace,
		podController:        podController,
		podControllerStop:    podControllerStop,
		policyController:     policyController,
		policyControllerStop: policyControllerStop,
	}
	return namespaceWatcher
}

func (n *NamespaceWatcher) stopWatchingNamespace() {
	n.podControllerStop <- struct{}{}
	n.policyControllerStop <- struct{}{}
}
