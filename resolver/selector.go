package resolver

import (
	"github.com/aporeto-inc/trireme/policy"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/selection"
)

func clauseEqual(requirement labels.Requirement) policy.clause {

}

func clauseIn(requirement labels.Requirement) policy.clause {

}

func clauseNotIn(requirement labels.Requirement) policy.clause {

}

func clauseExists(requirement labels.Requirement) policy.clause {

}

func clauseDoesNotExist(requirement labels.Requirement) policy.clause {

}

func indivualRule(req *policy.ContainerInfo, rule *extensions.NetworkPolicyIngressRule) error {

	for _, peer := range rule.From {

		// Individual peer. Each Peer is ORed
		peerSelector, err := apiu.LabelSelectorAsSelector(peer.PodSelector)
		if err != nil {
			return err
		}

		requirements, _ := peerSelector.Requirements()
		clause := []policy.KeyValueOperator{}

		for _, requirement := range requirements {
			// Each requirement is ANDed
			kv := policy.KeyValueOperator{}
			switch requirement.Operator() {
			case selection.Equals:
				kv.Key = requirement.Key()
				kv.Operator = policy.Equal
				kv.Value = requirement.Values().List()[0]
			case selection.In:
				return nil
			case selection.NotIn:
				return nil
			case selection.Exists:
				return nil
			case selection.DoesNotExist:
				return nil
			}

			clause = append(clause, kv)
		}
		selector := policy.TagSelectorInfo{
			Clause: clause,
			Action: policy.Accept,
		}
		req.Policy.Rules = append(req.Policy.Rules, selector)
	}

	return nil
}
