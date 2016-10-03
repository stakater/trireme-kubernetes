package resolver

import (
	"github.com/aporeto-inc/trireme/policy"
	apiu "k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/selection"
)

func clauseEqual(requirement labels.Requirement) []policy.KeyValueOperator {
	kvo := policy.KeyValueOperator{
		Key:      requirement.Key(),
		Operator: policy.Equal,
		Value:    requirement.Values().List()[0],
	}
	return []policy.KeyValueOperator{kvo}
}

func clauseIn(requirement labels.Requirement) []policy.KeyValueOperator {
	return nil
}

func clauseNotIn(requirement labels.Requirement) []policy.KeyValueOperator {
	kvos := []policy.KeyValueOperator{}
	for _, value := range requirement.Values().List() {
		kvo := policy.KeyValueOperator{
			Key:      requirement.Key(),
			Operator: policy.NotEqual,
			Value:    value,
		}
		kvos = append(kvos, kvo)
	}
	return kvos
}

func clauseExists(requirement labels.Requirement) []policy.KeyValueOperator {
	kvo := policy.KeyValueOperator{
		Key:      requirement.Key(),
		Operator: policy.Equal,
		Value:    "*",
	}
	return []policy.KeyValueOperator{kvo}
}

func clauseDoesNotExist(requirement labels.Requirement) []policy.KeyValueOperator {
	kvo := policy.KeyValueOperator{
		Key:      requirement.Key(),
		Operator: policy.NotEqual,
		Value:    "*",
	}
	return []policy.KeyValueOperator{kvo}
}

// generatePortTags generates all the
func generatePortTags(ports []extensions.NetworkPolicyPort) []policy.KeyValueOperator {
	// If no Ports are explicite
	if len(ports) == 0 {
		return []policy.KeyValueOperator{}
	}
	return nil

}

func individualRule(req *policy.ContainerInfo, rule *extensions.NetworkPolicyIngressRule) error {

	for _, peer := range rule.From {

		// Individual From. Each From is ORed.
		peerSelector, err := apiu.LabelSelectorAsSelector(peer.PodSelector)
		if err != nil {
			return err
		}

		requirements, _ := peerSelector.Requirements()
		completeClause := []policy.KeyValueOperator{}
		for _, requirement := range requirements {

			// Each requirement is ANDed
			switch requirement.Operator() {
			case selection.Equals:
				requirementClause := clauseEqual(requirement)
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
