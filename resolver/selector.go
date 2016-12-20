package resolver

import (
	"fmt"

	"github.com/aporeto-inc/trireme/policy"
	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	metav1 "k8s.io/kubernetes/pkg/apis/meta/v1"
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

func addPodRules(rule *extensions.NetworkPolicyIngressRule, namespace string) ([]policy.TagSelector, error) {

	receiverRules := []policy.TagSelector{}
	for _, peer := range rule.From {

		// Individual From. Each From is ORed.
		peerSelector, err := metav1.LabelSelectorAsSelector(peer.PodSelector)
		if err != nil {
			return nil, fmt.Errorf("Error while parsing Peer label selector %s", err)
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
		receiverRules = append(receiverRules, selector)
	}

	return receiverRules, nil
}

func addNamespaceRules(rule *extensions.NetworkPolicyIngressRule, podNamespace string, allNamespaces *api.NamespaceList) ([]policy.TagSelector, error) {
	receiverRules := []policy.TagSelector{}
	matchedNamespaces := map[string]bool{}
	for _, peer := range rule.From {
		// Individual From. Each From is ORed.
		namespaceSelector, err := metav1.LabelSelectorAsSelector(peer.NamespaceSelector)
		if err != nil {
			return nil, fmt.Errorf("Error while parsing Peer label selector %s", err)
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
		return nil, nil
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

	receiverRules = append(receiverRules, selector)
	return receiverRules, nil
}

func addACLRules() ([]policy.IPRule, error) {
	iPruleTCP := policy.IPRule{
		Address:  "0.0.0.0/0",
		Port:     "80",
		Protocol: "TCP",
	}
	iPruleUDP := policy.IPRule{
		Address:  "0.0.0.0/0",
		Port:     "80",
		Protocol: "UDP",
	}
	return []policy.IPRule{iPruleTCP, iPruleUDP}, nil
}

func logRules(containerPolicy *policy.PUPolicy) {
	for i, selector := range containerPolicy.ReceiverRules().TagSelectors {
		for _, clause := range selector.Clause {
			glog.V(5).Infof("Trireme policy for container X : Selector %d : %+v ", i, clause)
		}
	}
}

// createPolicyRules populate the RuleDB of a PU based on the list
// of IngressRules coming from Kubernetes.
func createPolicyRules(rules *[]extensions.NetworkPolicyIngressRule, podNamespace string, allNamespaces *api.NamespaceList, tags *policy.TagsMap, ips *policy.IPMap) (*policy.PUPolicy, error) {
	receiverRules := []policy.TagSelector{}
	ipRules := []policy.IPRule{}

	for _, rule := range *rules {

		// Phase1: populate the clauses related to each individual rules.
		podSelectorRules, err := addPodRules(&rule, podNamespace)
		if err != nil {
			return nil, fmt.Errorf("Error creating pod policyRule: %s", err)
		}
		receiverRules = append(receiverRules, podSelectorRules...)

		// Phase2: populate the clauses related to the namespace rules. (namepace selector...)
		namespaceSelectorRules, err := addNamespaceRules(&rule, podNamespace, allNamespaces)
		if err != nil {
			return nil, fmt.Errorf("Error creating pod namespaceRule: %s", err)
		}
		receiverRules = append(receiverRules, namespaceSelectorRules...)

		// Phase3: populate the network rules.
		aclSelectorRules, err := addACLRules()
		if err != nil {
			return nil, fmt.Errorf("Error creating pod ACLRules: %s", err)
		}
		ipRules = append(ipRules, aclSelectorRules...)

	}
	ingressACLs := policy.NewIPRuleList(ipRules)
	egressACLs := policy.NewIPRuleList(ipRules)
	receiverRulesList := policy.NewTagSelectorList(receiverRules)

	containerPolicy := policy.NewPUPolicy("", policy.Police, ingressACLs, egressACLs, nil, receiverRulesList, tags, tags, ips, nil)

	logRules(containerPolicy)
	return containerPolicy, nil
}

func allowAllPolicy(tags *policy.TagsMap, ipMap *policy.IPMap) *policy.PUPolicy {
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
	iPruleTCP := policy.IPRule{
		Address:  "0.0.0.0/0",
		Port:     "0:65535",
		Protocol: "TCP",
	}
	iPruleUDP := policy.IPRule{
		Address:  "0.0.0.0/0",
		Port:     "0:65535",
		Protocol: "UDP",
	}
	receivingRules := policy.NewTagSelectorList([]policy.TagSelector{selector})
	ingressACLs := policy.NewIPRuleList([]policy.IPRule{iPruleTCP, iPruleUDP})
	egressACLs := policy.NewIPRuleList([]policy.IPRule{iPruleTCP, iPruleUDP})

	return policy.NewPUPolicy("", policy.AllowAll, ingressACLs, egressACLs, nil, receivingRules, tags, tags, ipMap, nil)
}

func notInfraContainerPolicy() *policy.PUPolicy {
	return policy.NewPUPolicyWithDefaults()
}
