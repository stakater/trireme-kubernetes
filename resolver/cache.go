package resolver

import (
	"fmt"
	"sync"
)

// Cache keeps all the state needed for the integration.
type Cache struct {
	// namespaceActivation
	namespaceActivation map[string]*NamespaceWatcher
	// contextIDCache keeps a mapping between a POD name and the corresponding contextID
	contextIDCache map[string]string
	sync.RWMutex
}

func newCache() *Cache {
	return &Cache{
		namespaceActivation: map[string]*NamespaceWatcher{},
		contextIDCache:      map[string]string{},
	}
}

func kubePodIdentifier(podName string, podNamespace string) string {
	return podNamespace + "/" + podName
}

func (c *Cache) addPodToCache(contextID string, podName string, podNamespace string) {
	c.Lock()
	defer c.Unlock()
	kubeIdentifier := kubePodIdentifier(podName, podNamespace)
	c.contextIDCache[kubeIdentifier] = contextID
}

func (c *Cache) contextIDByPodName(podName string, podNamespace string) (string, error) {
	c.Lock()
	defer c.Unlock()
	kubeIdentifier := kubePodIdentifier(podName, podNamespace)
	contextID, ok := c.contextIDCache[kubeIdentifier]
	if !ok {
		return "", fmt.Errorf("Pod %v not found in Cache", kubeIdentifier)
	}
	return contextID, nil
}

func (c *Cache) deleteFromCacheByPodName(podName string, podNamespace string) error {
	c.Lock()
	defer c.Unlock()
	kubeIdentifier := kubePodIdentifier(podName, podNamespace)
	_, ok := c.contextIDCache[kubeIdentifier]
	if !ok {
		return fmt.Errorf("Pod %v not found in Cache", kubeIdentifier)
	}
	delete(c.contextIDCache, kubeIdentifier)
	return nil
}

func (c *Cache) getNamespaceWatcher(namespace string) (*NamespaceWatcher, bool) {
	namespaceWatcher, ok := c.namespaceActivation[namespace]
	return namespaceWatcher, ok
}

func (c *Cache) activateNamespaceWatcher(namespace string, namespaceWatcher *NamespaceWatcher) {
	c.namespaceActivation[namespace] = namespaceWatcher
}

func (c *Cache) deactivateNamespaceWatcher(namespace string) {
	namespaceWatcher, ok := c.namespaceActivation[namespace]
	if !ok {
		return
	}
	namespaceWatcher.podStopChan <- true
	namespaceWatcher.policyStopChan <- true
	delete(c.namespaceActivation, namespace)
}

func (c *Cache) namespaceStatus(namespace string) bool {
	_, ok := c.namespaceActivation[namespace]
	if !ok {
		return false
	}
	return true
}
