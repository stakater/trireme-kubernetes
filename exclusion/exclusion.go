package exclusion

import (
	"fmt"
	"net"
	"sync"

	"github.com/aporeto-inc/trireme-kubernetes/kubernetes"
	"github.com/aporeto-inc/trireme/supervisor"
	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
)

// Watcher is maintaining the state of the ExclusionList.
type Watcher struct {
	kubeClient            kubernetes.Client
	triremeNets           []*net.IPNet
	excluder              supervisor.Excluder
	excludedIPs           map[string]struct{}
	serviceController     *cache.Controller
	serviceControllerStop chan struct{}
	mutex                 sync.Mutex
}

// NewWatcher generates a new Watcher
func NewWatcher(triremeNets []string, kubeClient kubernetes.Client, excluder supervisor.Excluder) (*Watcher, error) {
	ipNets := []*net.IPNet{}
	for _, triremeNet := range triremeNets {
		_, parsedNet, err := net.ParseCIDR(triremeNet)
		if err != nil {
			return nil, fmt.Errorf("Error parsing Trireme Subnet: %s", err)
		}
		ipNets = append(ipNets, parsedNet)
	}
	watcher := &Watcher{
		kubeClient:  kubeClient,
		triremeNets: ipNets,
		excluder:    excluder,
		excludedIPs: map[string]struct{}{},
		mutex:       sync.Mutex{},
	}

	watcher.serviceControllerStop = make(chan struct{})
	_, watcher.serviceController = kubeClient.CreateServiceController(
		watcher.addService,
		watcher.deleteService,
		watcher.updateService)

	return watcher, nil
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
	fmt.Printf("Processing Added Cluster IP: %s", addedAPIStruct.Spec.ClusterIP)
	endpoints, _ := w.kubeClient.Endpoints(addedAPIStruct.GetName(), addedAPIStruct.GetNamespace())
	for _, set := range endpoints.Subsets {
		for _, ip := range set.Addresses {
			glog.V(2).Infof("Checking if endpoint IP %s (ClusterIP %s ) is part of TriremeNets ", ip, addedAPIStruct.Spec.ClusterIP)
			if !w.isInTriremeNets(ip.IP) {
				return w.excludeServiceIP(addedAPIStruct.Spec.ClusterIP)
			}
		}
	}
	return nil
}

func (w *Watcher) deleteService(deletedAPIStruct *api.Service) error {
	if w.isIPExcluded(deletedAPIStruct.Spec.ClusterIP) {
		return w.restoreServiceIP(deletedAPIStruct.Spec.ClusterIP)
	}
	return nil
}

func (w *Watcher) updateService(oldAPIStruct, updatedAPIStruct *api.Service) error {
	return nil
}

func (w *Watcher) excludeServiceIP(ip string) error {
	glog.V(2).Infof("Excluding IP %s", ip)
	if err := w.excluder.AddExcludedIP(ip + "/32"); err != nil {
		return fmt.Errorf("Error excluding IP: %s", err)
	}
	w.mutex.Lock()
	w.excludedIPs[ip] = struct{}{}
	w.mutex.Unlock()
	return nil
}

func (w *Watcher) restoreServiceIP(ip string) error {
	glog.V(2).Infof("Restoring IP %s", ip)

	if err := w.excluder.RemoveExcludedIP(ip + "/32"); err != nil {
		return fmt.Errorf("Error restoring IP: %s", err)
	}
	w.mutex.Lock()
	delete(w.excludedIPs, ip)
	w.mutex.Unlock()
	return nil
}

func (w *Watcher) isIPExcluded(ip string) bool {
	w.mutex.Lock()
	_, ok := w.excludedIPs[ip]
	w.mutex.Unlock()
	return ok
}

func (w *Watcher) isInTriremeNets(ip string) bool {
	glog.V(2).Infof("Testing IP %s", ip)
	testip := net.ParseIP(ip)
	if testip == nil {
		return false
	}
	for _, subnet := range w.triremeNets {
		if subnet.Contains(testip) {
			return true
		}
	}
	return false
}
