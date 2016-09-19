package policy

import (
	"fmt"

	"github.com/aporeto-inc/trireme/policy"
)

type podCache struct {
	// podCache keeps a mapping between a POD name and the corresponding contextID
	contextIDCache map[string]string
	// cache keeps a cache of the contextID to the podCacheEntry object
	podEntryCache map[string]*podCacheEntry
}

type podCacheEntry struct {
	podName       string
	podNamespace  string
	dockerID      string
	containerInfo *policy.ContainerInfo
}

func kubePodIdentifier(podName, podNamespace string) string {
	return podNamespace + "/" + podName
}

func (k *KubernetesPolicy) addPodToCache(contextID string, dockerID string, podName string, podNamespace string, containerInfo *policy.ContainerInfo) {
	kubeIdentifier := kubePodIdentifier(podName, podNamespace)
	cacheEntry := &podCacheEntry{
		podName:       podName,
		podNamespace:  podNamespace,
		dockerID:      dockerID,
		containerInfo: containerInfo,
	}

	k.cache.contextIDCache[kubeIdentifier] = contextID
	k.cache.podEntryCache[contextID] = cacheEntry
}

func (k *KubernetesPolicy) getCachedPodByName(podName string, podNamespace string) (*podCacheEntry, error) {
	contextID, err := k.getContextIDByPodName(podName, podNamespace)
	if err != nil {
		return nil, err
	}
	cacheEntry, err := k.getCachedPodByContextID(contextID)
	if err != nil {
		return nil, fmt.Errorf("Cache Inconsistency: %s ", err)
	}
	return cacheEntry, nil
}

func (k *KubernetesPolicy) getContextIDByPodName(podName string, podNamespace string) (string, error) {
	kubeIdentifier := kubePodIdentifier(podName, podNamespace)
	contextID, ok := k.cache.contextIDCache[kubeIdentifier]
	if !ok {
		return "", fmt.Errorf("Pod %v not found in Cache", kubeIdentifier)
	}
	return contextID, nil
}

func (k *KubernetesPolicy) getCachedPodByContextID(contextID string) (*podCacheEntry, error) {
	cacheEntry, ok := k.cache.podEntryCache[contextID]
	if !ok {
		return nil, fmt.Errorf("ContextID %s not found in Cache", contextID)
	}
	return cacheEntry, nil
}

func (k *KubernetesPolicy) deletePodFromCacheByName(podName string, podNamespace string) error {
	kubeIdentifier := kubePodIdentifier(podName, podNamespace)
	contextID, ok := k.cache.contextIDCache[kubeIdentifier]
	if !ok {
		return fmt.Errorf("Pod %v not found in Cache", kubeIdentifier)
	}
	delete(k.cache.contextIDCache, kubeIdentifier)
	delete(k.cache.podEntryCache, contextID)
	return nil
}

func (k *KubernetesPolicy) deletePodFromCacheByContextID(contextID string) error {
	cacheEntry, err := k.getCachedPodByContextID(contextID)
	if err != nil {
		return err
	}
	kubeIdentifier := kubePodIdentifier(cacheEntry.podNamespace, cacheEntry.podName)
	delete(k.cache.contextIDCache, kubeIdentifier)
	delete(k.cache.podEntryCache, contextID)
	return nil
}
