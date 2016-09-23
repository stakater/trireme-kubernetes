package resolver

import (
	"fmt"

	"github.com/aporeto-inc/trireme/policy"
)

// Cache keeps all the state needed for the integration.
type Cache struct {
	// namespaceActivation
	namespaceActivation map[string]*NamespaceWatcher
	// podCache keeps a mapping between a POD name and the corresponding contextID
	contextIDCache map[string]string
	// cache keeps a cache of the contextID to the podCacheEntry object
	podEntryCache map[string]*podCacheEntry
	//
}

type podCacheEntry struct {
	podName       string
	podNamespace  string
	dockerID      string
	containerInfo *policy.ContainerInfo
}

func newCache() *Cache {
	return &Cache{
		namespaceActivation: map[string]*NamespaceWatcher{},
		contextIDCache:      map[string]string{},
		podEntryCache:       map[string]*podCacheEntry{},
	}
}

func kubePodIdentifier(podName, podNamespace string) string {
	return podNamespace + "/" + podName
}

func (c *Cache) addPodToCache(contextID string, dockerID string, podName string, podNamespace string, containerInfo *policy.ContainerInfo) {
	kubeIdentifier := kubePodIdentifier(podName, podNamespace)
	cacheEntry := &podCacheEntry{
		podName:       podName,
		podNamespace:  podNamespace,
		dockerID:      dockerID,
		containerInfo: containerInfo,
	}

	c.contextIDCache[kubeIdentifier] = contextID
	c.podEntryCache[contextID] = cacheEntry
}

func (c *Cache) getCachedPodByName(podName string, podNamespace string) (*podCacheEntry, error) {
	contextID, err := c.getContextIDByPodName(podName, podNamespace)
	if err != nil {
		return nil, err
	}
	cacheEntry, err := c.getCachedPodByContextID(contextID)
	if err != nil {
		return nil, fmt.Errorf("Cache Inconsistency: %s ", err)
	}
	return cacheEntry, nil
}

func (c *Cache) getContextIDByPodName(podName string, podNamespace string) (string, error) {
	kubeIdentifier := kubePodIdentifier(podName, podNamespace)
	contextID, ok := c.contextIDCache[kubeIdentifier]
	if !ok {
		return "", fmt.Errorf("Pod %v not found in Cache", kubeIdentifier)
	}
	return contextID, nil
}

func (c *Cache) getCachedPodByContextID(contextID string) (*podCacheEntry, error) {
	cacheEntry, ok := c.podEntryCache[contextID]
	if !ok {
		return nil, fmt.Errorf("ContextID %s not found in Cache", contextID)
	}
	return cacheEntry, nil
}

func (c *Cache) deletePodFromCacheByName(podName string, podNamespace string) error {
	kubeIdentifier := kubePodIdentifier(podName, podNamespace)
	contextID, ok := c.contextIDCache[kubeIdentifier]
	if !ok {
		return fmt.Errorf("Pod %v not found in Cache", kubeIdentifier)
	}
	delete(c.contextIDCache, kubeIdentifier)
	delete(c.podEntryCache, contextID)
	return nil
}

func (c *Cache) deletePodFromCacheByContextID(contextID string) error {
	cacheEntry, err := c.getCachedPodByContextID(contextID)
	if err != nil {
		return err
	}
	kubeIdentifier := kubePodIdentifier(cacheEntry.podNamespace, cacheEntry.podName)
	delete(c.contextIDCache, kubeIdentifier)
	delete(c.podEntryCache, contextID)
	return nil
}

func (c *Cache) activateNamespace(namespace string, namespaceWatcher *NamespaceWatcher) {
	c.namespaceActivation[namespace] = namespaceWatcher
}

func (c *Cache) deactivateNamespace(namespace string) {
	delete(c.namespaceActivation, namespace)
}

func (c *Cache) namespaceStatus(namespace string) bool {
	_, ok := c.namespaceActivation[namespace]
	if !ok {
		return false
	}
	return true
}
