package resolver

import (
	"github.com/aporeto-inc/trireme/policy"
	"github.com/golang/glog"
	apiu "k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/selection"
)

func clauseEquals(requirement labels.Requirement) []policy.KeyValueOperator {
	kvo := policy.KeyValueOperator{
		Key:      requirement.Key(),
		Operator: policy.Equal,
		Value:    requirement.Values().List(),
	}
	return []policy.KeyValueOperator{kvo}
}

func clauseNotEquals(requirement labels.Requirement) []policy.KeyValueOperator {
	kvo := policy.KeyValueOperator{
		Key:      requirement.Key(),
		Operator: policy.NotEqual,
		Value:    requirement.Values().List(),
	}
	return []policy.KeyValueOperator{kvo}
}

func clauseIn(requirement labels.Requirement) []policy.KeyValueOperator {
	kvo := policy.KeyValueOperator{
		Key:      requirement.Key(),
		Operator: policy.Equal,
		Value:    requirement.Values().List(),
	}
	return []policy.KeyValueOperator{kvo}
}

func clauseNotIn(requirement labels.Requirement) []policy.KeyValueOperator {
	kvo := policy.KeyValueOperator{
		Key:      requirement.Key(),
		Operator: policy.NotEqual,
		Value:    requirement.Values().List(),
	}
	return []policy.KeyValueOperator{kvo}
}

func clauseExists(requirement labels.Requirement) []policy.KeyValueOperator {
	kvo := policy.KeyValueOperator{
		Key:      requirement.Key(),
		Operator: policy.ANY,
		Value:    []string{"*"},
	}
	return []policy.KeyValueOperator{kvo}
}

func clauseDoesNotExist(requirement labels.Requirement) []policy.KeyValueOperator {
	kvo := policy.KeyValueOperator{
		Key:      requirement.Key(),
		Operator: policy.ANY,
		Value:    []string{"*"},
	}
	return []policy.KeyValueOperator{kvo}
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

func individualRule(req *policy.ContainerInfo, rule *extensions.NetworkPolicyIngressRule) error {

	for _, peer := range rule.From {

		// Individual From. Each From is ORed.
		peerSelector, err := apiu.LabelSelectorAsSelector(peer.PodSelector)
		if err != nil {
			return err
		}

		requirements, _ := peerSelector.Requirements()

		// Initialize the completeClause with the port matching
		completeClause := []policy.KeyValueOperator{}
		completeClause = append(completeClause, generatePortTags(rule.Ports)...)
		for _, requirement := range requirements {

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
		selector := policy.TagSelectorInfo{
			Clause: completeClause,
			Action: policy.Accept,
		}
		req.Policy.Rules = append(req.Policy.Rules, selector)
	}

	return nil
}

func printRules(req *policy.ContainerInfo) {
	for i, selector := range req.Policy.Rules {
		for _, clause := range selector.Clause {
			glog.V(2).Infof("Trireme policy for container %s : Selector %s : %+v ", req.RunTime.Name, i, clause)
		}
	}
}
