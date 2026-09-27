package kube

import (
	"fmt"
	"strings"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// NamespaceSelectorMatches reports whether a namespace is in scope for a webhook's namespaceSelector.
// A nil selector matches all namespaces.
func NamespaceSelectorMatches(ns string, nsLabels map[string]string, sel *metav1.LabelSelector) (bool, error) {
	if sel == nil {
		return true, nil
	}
	ls, err := metav1.LabelSelectorAsSelector(sel)
	if err != nil {
		return false, err
	}
	if ls.Empty() {
		return true, nil
	}
	return ls.Matches(labels.Set(nsLabels)), nil
}

// FormatNamespaceScopeLabel summarizes webhook namespace targeting for browse and investigation UI.
func FormatNamespaceScopeLabel(sel *metav1.LabelSelector) string {
	if sel == nil {
		return "All namespaces"
	}
	if len(sel.MatchLabels) == 0 && len(sel.MatchExpressions) == 0 {
		return "All namespaces"
	}
	var parts []string
	for k, v := range sel.MatchLabels {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	for _, ex := range sel.MatchExpressions {
		vals := strings.Join(ex.Values, ",")
		parts = append(parts, fmt.Sprintf("%s %s %s", ex.Key, ex.Operator, vals))
	}
	if len(parts) == 0 {
		return "All namespaces"
	}
	return strings.Join(parts, "; ")
}

// WebhookRulesMatchWorkloads returns true when a rule can intercept workload or pod admission.
func WebhookRulesMatchWorkloads(rules []admissionregistrationv1.RuleWithOperations) bool {
	for _, r := range rules {
		if !ruleOperationsIncludeCreateOrUpdate(r.Operations) {
			continue
		}
		for _, res := range r.Resources {
			switch res {
			case "pods", "deployments", "replicasets", "statefulsets", "daemonsets", "jobs", "*":
				return true
			}
		}
	}
	return false
}

func ruleOperationsIncludeCreateOrUpdate(ops []admissionregistrationv1.OperationType) bool {
	for _, op := range ops {
		switch op {
		case admissionregistrationv1.Create, admissionregistrationv1.Update, "*":
			return true
		}
	}
	return false
}
