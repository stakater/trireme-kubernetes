package resolver

import (
	"fmt"

	"github.com/aporeto-inc/trireme/policy"
	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	apiu "k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/selection"
)

func clauseEquals(requirement labels.Requirement) []policy.KeyValueOperator {
	return []policy.KeyValueOperator{
		policy.KeyValueOperator{
			Key:      requirement.Key(),
			Operator: policy.Equal,
			Value:    requirement.Values().List(),
		},
	}
}

func clauseNotEquals(requirement labels.Requirement) []policy.KeyValueOperator {
	return []policy.KeyValueOperator{
		policy.KeyValueOperator{
			Key:      requirement.Key(),
			Operator: policy.NotEqual,
			Value:    requirement.Values().List(),
		},
	}
}

func clauseIn(requirement labels.Requirement) []policy.KeyValueOperator {
	return []policy.KeyValueOperator{
		policy.KeyValueOperator{
			Key:      requirement.Key(),
			Operator: policy.Equal,
			Value:    requirement.Values().List(),
		},
	}
}

func clauseNotIn(requirement labels.Requirement) []policy.KeyValueOperator {
	return []policy.KeyValueOperator{
		policy.KeyValueOperator{
			Key:      requirement.Key(),
			Operator: policy.NotEqual,
			Value:    requirement.Values().List(),
		},
	}
}

func clauseExists(requirement labels.Requirement) []policy.KeyValueOperator {
	return []policy.KeyValueOperator{
		policy.KeyValueOperator{
			Key:      requirement.Key(),
			Operator: policy.KeyExists,
			Value:    []string{"*"},
		},
	}
}

func clauseDoesNotExist(requirement labels.Requirement) []policy.KeyValueOperator {
	return []policy.KeyValueOperator{
		policy.KeyValueOperator{
			Key:      requirement.Key(),
			Operator: policy.KeyNotExists,
			Value:    []string{"*"},
		},
	}
}

// generatePortTags generates all the clauses for the ports
func generatePortTags(ports []extensions.NetworkPolicyPort) []policy.KeyValueOperator {
	// If Port is not defined, then no need for specific traffic matching.
	if ports == nil {
		return []policy.KeyValueOperator{}
	}
	// If Port is defined but no ports are defined into it, then No traffic is matched at all
	if len(ports) == 0 {
		return nil
	}

	portList := []string{}
	for _, port := range ports {
		portList = append(portList, port.Port.String())
	}
	kvo := policy.KeyValueOperator{
		Key:      "@port",
		Operator: policy.Equal,
		Value:    portList,
	}
	return []policy.KeyValueOperator{kvo}

}

func generateNamespacekvo(namespace string) []policy.KeyValueOperator {
	kvo := policy.KeyValueOperator{
		Key:      "@namespace",
		Operator: policy.Equal,
		Value:    []string{namespace},
	}
	return []policy.KeyValueOperator{kvo}
}

func addPodRules(containerPolicy *policy.PUPolicy, rule *extensions.NetworkPolicyIngressRule, namespace string) error {

	for _, peer := range rule.From {

		// Individual From. Each From is ORed.
		peerSelector, err := apiu.LabelSelectorAsSelector(peer.PodSelector)
		if err != nil {
			return fmt.Errorf("Error while parsing Peer label selector %s", err)
		}
		peerRequirements, _ := peerSelector.Requirements()

		// Initialize the completeClause with the port matching
		completeClause := []policy.KeyValueOperator{}
		completeClause = append(completeClause, generatePortTags(rule.Ports)...)

		// Also add the Pod Namespace as a requirement.
		completeClause = append(completeClause, generateNamespacekvo(namespace)...)

		// Go over each specific requirement and add it as a clause.
		for _, requirement := range peerRequirements {
			// Each requirement is ANDed
			switch requirement.Operator() {
			case selection.Equals:
				requirementClause := clauseEquals(requirement)
				completeClause = append(completeClause, requirementClause...)
			case selection.NotEquals:
				requirementClause := clauseNotEquals(requirement)
				completeClause = append(completeClause, requirementClause...)
			case selection.In:
				requirementClause := clauseIn(requirement)
				completeClause = append(completeClause, requirementClause...)
			case selection.NotIn:
				requirementClause := clauseNotIn(requirement)
				completeClause = append(completeClause, requirementClause...)
			case selection.Exists:
				requirementClause := clauseExists(requirement)
				completeClause = append(completeClause, requirementClause...)
			case selection.DoesNotExist:
				requirementClause := clauseDoesNotExist(requirement)
				completeClause = append(completeClause, requirementClause...)
			}

		}
		selector := policy.TagSelector{
			Clause: completeClause,
			Action: policy.Accept,
		}
		containerPolicy.Rules = append(containerPolicy.Rules, selector)
	}

	return nil
}

func addNamespaceRules(containerPolicy *policy.PUPolicy, rule *extensions.NetworkPolicyIngressRule, podNamespace string, allNamespaces *api.NamespaceList) error {

	matchedNamespaces := map[string]bool{}
	for _, peer := range rule.From {
		// Individual From. Each From is ORed.
		namespaceSelector, err := apiu.LabelSelectorAsSelector(peer.NamespaceSelector)
		if err != nil {
			return fmt.Errorf("Error while parsing Peer label selector %s", err)
		}
		for _, namespace := range allNamespaces.Items {
			if namespaceSelector.Matches(labels.Set(namespace.GetLabels())) {
				matchedNamespaces[namespace.GetName()] = true
			}
		}
	}

	allowedNamespaces := []string{}
	for namespace := range matchedNamespaces {
		// We don't want to match all of the current namespace.
		if namespace == podNamespace {
			continue
		}
		allowedNamespaces = append(allowedNamespaces, namespace)
	}
	// No need to add the Namespace clause if no namespaces were matched.
	if len(allowedNamespaces) == 0 {
		return nil
	}
	clause := policy.KeyValueOperator{
		Key:      "@namespace",
		Operator: policy.Equal,
		Value:    allowedNamespaces,
	}

	selector := policy.TagSelector{
		Clause: []policy.KeyValueOperator{clause},
		Action: policy.Accept,
	}

	containerPolicy.Rules = append(containerPolicy.Rules, selector)
	return nil
}

func logRules(containerPolicy *policy.PUPolicy) {
	for i, selector := range containerPolicy.Rules {
		for _, clause := range selector.Clause {
			glog.V(5).Infof("Trireme policy for container X : Selector %d : %+v ", i, clause)
		}
	}
}

// createPolicyRules populate the RuleDB of a PU based on the list
// of IngressRules coming from Kubernetes.
func createPolicyRules(rules *[]extensions.NetworkPolicyIngressRule, podNamespace string, allNamespaces *api.NamespaceList) (*policy.PUPolicy, error) {
	containerPolicy := policy.NewPUPolicy()

	for _, rule := range *rules {
		// Populate the clauses related to each individual rules.
		if err := addPodRules(containerPolicy, &rule, podNamespace); err != nil {
			return nil, fmt.Errorf("Error creating pod policyRule: %s", err)
		}
		if err := addNamespaceRules(containerPolicy, &rule, podNamespace, allNamespaces); err != nil {
			return nil, fmt.Errorf("Error creating pod policyRule: %s", err)
		}
	}
	logRules(containerPolicy)
	return containerPolicy, nil
}

func allowAllPolicy() *policy.PUPolicy {
	containerPolicy := policy.NewPUPolicy()
	completeClause := []policy.KeyValueOperator{
		policy.KeyValueOperator{
			Key:      "@namespace",
			Operator: policy.Equal,
			Value:    []string{"*"},
		},
	}
	selector := policy.TagSelector{
		Clause: completeClause,
		Action: policy.Accept,
	}
	containerPolicy.Rules = append(containerPolicy.Rules, selector)
	containerPolicy.TriremeAction = policy.AllowAll

	return containerPolicy
}

func notInfraContainerPolicy() *policy.PUPolicy {
	containerPolicy := policy.NewPUPolicy()
	containerPolicy.PolicyIPs = []string{""}
	containerPolicy.TriremeAction = policy.AllowAll

	return containerPolicy
}
