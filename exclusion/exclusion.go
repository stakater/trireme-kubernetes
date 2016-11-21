package exclusion

import (
	"fmt"

	"github.com/aporeto-inc/trireme-kubernetes/kubernetes"
	"github.com/aporeto-inc/trireme/supervisor"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
)

// Watcher is maintaining the state of the ExclusionList.
type Watcher struct {
	kubeClient            kubernetes.Client
	triremeNets           []string
	excluder              supervisor.Excluder
	serviceController     *cache.Controller
	serviceControllerStop chan struct{}
}

func NewWatcher(triremeNets []string, kubeClient kubernetes.Client, excluder supervisor.Excluder) Watcher {

	watcher := Watcher{
		kubeClient:  kubeClient,
		triremeNets: triremeNets,
		excluder:    excluder,
	}

	watcher.serviceControllerStop = make(chan struct{})
	_, watcher.serviceController = kubeClient.CreateServiceController(
		watcher.addService,
		watcher.deleteService,
		watcher.updateService)

	return watcher
}

// Start launches the Exclusion Watcher
// Blocking. Use go...
func (w *Watcher) Start() {
	w.serviceController.Run(w.serviceControllerStop)
}

// Stop stops the Excluder updater.
func (w *Watcher) Stop() {
	w.serviceControllerStop <- struct{}{}
}

func (w *Watcher) addService(addedAPIStruct *api.Service) error {
	if addedAPIStruct.Spec.ClusterIP == "" {
		return nil
	}
	fmt.Printf("Processing Cluster IP: %s", addedAPIStruct.Spec.ClusterIP)
	endpoints, _ := w.kubeClient.Endpoints(addedAPIStruct.GetName(), addedAPIStruct.GetNamespace())
	for _, set := range endpoints.Subsets {
		fmt.Printf("Addresses: %+v ", set.Addresses)
	}
	return nil
}

func (w *Watcher) deleteService(deletedAPIStruct *api.Service) error {
	return nil
}

func (w *Watcher) updateService(oldAPIStruct, updatedAPIStruct *api.Service) error {
	return nil
}
