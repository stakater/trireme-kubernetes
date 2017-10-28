package resolver

import (
	"testing"

	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAllowAllIngress(*testing.T) {

	allowAllIngressDefinition := &networking.NetworkPolicy{
		Spec: networking.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			Ingress: []networking.NetworkPolicyIngressRule{
				networking.NetworkPolicyIngressRule{},
			},
		},
	}

}
func TestAllowAllEgress(*testing.T) {

	allowAllEgressDefinition := &networking.NetworkPolicy{
		Spec: networking.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			Egress: []networking.NetworkPolicyEgressRule{
				networking.NetworkPolicyEgressRule{},
			},
		},
	}

}
