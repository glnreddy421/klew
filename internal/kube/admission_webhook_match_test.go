package kube

import (
	"testing"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNamespaceSelectorMatches(t *testing.T) {
	labels := map[string]string{"env": "prod", "team": "payments"}
	sel := &metav1.LabelSelector{
		MatchLabels: map[string]string{"env": "prod"},
	}
	ok, err := NamespaceSelectorMatches("payments", labels, sel)
	if err != nil || !ok {
		t.Fatalf("expected match, ok=%v err=%v", ok, err)
	}
	ok, err = NamespaceSelectorMatches("staging", map[string]string{"env": "dev"}, sel)
	if err != nil || ok {
		t.Fatalf("expected no match, ok=%v err=%v", ok, err)
	}
	ok, err = NamespaceSelectorMatches("any", labels, nil)
	if err != nil || !ok {
		t.Fatalf("nil selector should match all")
	}
}

func TestWebhookRulesMatchWorkloads(t *testing.T) {
	rules := []admissionregistrationv1.RuleWithOperations{{
		Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
		Rule: admissionregistrationv1.Rule{
			APIGroups: []string{""},
			Resources: []string{"pods"},
		},
	}}
	if !WebhookRulesMatchWorkloads(rules) {
		t.Fatal("expected pods create to match")
	}
	rules[0].Rule.Resources = []string{"configmaps"}
	if WebhookRulesMatchWorkloads(rules) {
		t.Fatal("configmaps only should not match workload perimeter")
	}
}
