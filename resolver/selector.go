package resolver

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	api "k8s.io/client-go/pkg/api/v1"
	extensions "k8s.io/client-go/pkg/apis/extensions/v1beta1"

	"github.com/aporeto-inc/trireme/policy"
	"github.com/golang/glog"
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

// portSelector generates all the clauses for the ports
func portSelector(ports []extensions.NetworkPolicyPort) []policy.KeyValueOperator {
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

func namespaceSelector(namespace string) []policy.KeyValueOperator {
	kvo := policy.KeyValueOperator{
		Key:      "@namespace",
		Operator: policy.Equal,
		Value:    []string{namespace},
	}
	return []policy.KeyValueOperator{kvo}
}

// podRules generates all the rules for the whole pod.
func podRules(rule *extensions.NetworkPolicyIngressRule, namespace string) ([]policy.TagSelector, error) {

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
		completeClause = append(completeClause, portSelector(rule.Ports)...)

		// Also add the Pod Namespace as a requirement.
		completeClause = append(completeClause, namespaceSelector(namespace)...)

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

// namespaceRules generates all the rules associated with the matching of other namespaces
func namespaceRules(rule *extensions.NetworkPolicyIngressRule, podNamespace string, allNamespaces *api.NamespaceList) ([]policy.TagSelector, error) {
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

// aclRules generate the IPRules used as ACLs outside of Trireme cluster.
func aclRules(rule extensions.NetworkPolicyIngressRule) ([]policy.IPRule, error) {
	aclPolicy := []policy.IPRule{}
	if rule.Ports == nil {
		return nil, fmt.Errorf("Ports entry is nil")
	}

	for _, portEntry := range rule.Ports {
		proto := "TCP"
		if *portEntry.Protocol == api.ProtocolUDP {
			proto = "UDP"
		}

		iPruleTCP := policy.IPRule{
			Address:  "0.0.0.0/0",
			Port:     portEntry.Port.String(),
			Protocol: proto,
			Action:   policy.Accept,
		}

		iPruleUDP := policy.IPRule{
			Address:  "0.0.0.0/0",
			Port:     portEntry.Port.String(),
			Protocol: proto,
			Action:   policy.Accept,
		}
		aclPolicy = append(aclPolicy, iPruleTCP, iPruleUDP)
	}

	return aclPolicy, nil
}

// aclRules generate the IPRules used as ACLs outside of Trireme cluster.
func aclAllowAllRules() []policy.IPRule {
	iPruleTCP := policy.IPRule{
		Address:  "0.0.0.0/0",
		Port:     "0:65535",
		Protocol: "TCP",
		Action:   policy.Accept,
	}
	iPruleUDP := policy.IPRule{
		Address:  "0.0.0.0/0",
		Port:     "0:65535",
		Protocol: "UDP",
		Action:   policy.Accept,
	}
	return []policy.IPRule{iPruleTCP, iPruleUDP}
}

// logRules logs all the rules currently used. Useful for debugging.
func logRules(containerPolicy *policy.PUPolicy) {
	for i, selector := range containerPolicy.ReceiverRules().TagSelectors {
		for _, clause := range selector.Clause {
			glog.V(5).Infof("Trireme policy for container X : Selector %d : %+v ", i, clause)
		}
	}
}

// generatePUPolicy creates a PUPolicy representation
func generatePUPolicy(rules *[]extensions.NetworkPolicyIngressRule, podNamespace string, allNamespaces *api.NamespaceList, tags *policy.TagsMap, ips *policy.IPMap, triremeNets []string) (*policy.PUPolicy, error) {
	receiverRules := []policy.TagSelector{}
	ipRules := []policy.IPRule{}

	for _, rule := range *rules {

		// Not matching any traffic. Go to next rule
		if len(rule.From) == 0 || len(rule.Ports) == 0 {
			continue
		}

		// From is not set, Only using the Port information.
		if rule.From == nil {
			// Ports also not set: Allow All!
			if rule.Ports == nil {
				return allowAllPolicy(tags, ips, triremeNets), nil
			}

			aclSelectorRules, err := aclRules(rule)
			if err != nil {
				return nil, fmt.Errorf("Error creating pod ACLRules: %s", err)
			}
			ipRules = append(ipRules, aclSelectorRules...)
			continue
		}

		// Phase1: populate the clauses related to each individual rules.
		podSelectorRules, err := podRules(&rule, podNamespace)
		if err != nil {
			return nil, fmt.Errorf("Error creating pod policyRule: %s", err)
		}
		receiverRules = append(receiverRules, podSelectorRules...)

		// Phase2: populate the clauses related to the namespace rules. (namepace selector...)
		namespaceSelectorRules, err := namespaceRules(&rule, podNamespace, allNamespaces)
		if err != nil {
			return nil, fmt.Errorf("Error creating pod namespaceRule: %s", err)
		}
		receiverRules = append(receiverRules, namespaceSelectorRules...)

	}
	ingressACLs := policy.NewIPRuleList(ipRules)

	// Egress Allow All as per Network Policy definition.
	egressACLs := policy.NewIPRuleList(aclAllowAllRules())
	receiverRulesList := policy.NewTagSelectorList(receiverRules)

	containerPolicy := policy.NewPUPolicy("", policy.Police, egressACLs, ingressACLs, nil, receiverRulesList, tags, tags, ips, triremeNets, nil)

	logRules(containerPolicy)
	return containerPolicy, nil
}

// allowAllPolicy returns a simple generic policy used in order to not police the PU.
// example: The NS is not networkPolicy activated.
func allowAllPolicy(tags *policy.TagsMap, ipMap *policy.IPMap, triremeNets []string) *policy.PUPolicy {
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
	allowAllRules := aclAllowAllRules()
	receivingRules := policy.NewTagSelectorList([]policy.TagSelector{selector})
	ingressACLs := policy.NewIPRuleList([]policy.IPRule{allowAllRules[0], allowAllRules[1]})
	egressACLs := policy.NewIPRuleList([]policy.IPRule{allowAllRules[0], allowAllRules[1]})

	return policy.NewPUPolicy("", policy.AllowAll, ingressACLs, egressACLs, nil, receivingRules, tags, tags, ipMap, triremeNets, nil)
}

// notInfraContainerPolicy is a policy that should apply to the other containers in a PoD that are not the infra container.
func notInfraContainerPolicy() *policy.PUPolicy {
	return policy.NewPUPolicyWithDefaults()
}
