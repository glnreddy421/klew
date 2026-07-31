package details

import (
	"context"
	"fmt"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
)

type networkPolicyProvider struct{}

func (networkPolicyProvider) Kind() string { return "NetworkPolicy" }

func (networkPolicyProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	np, err := getNetworkPolicy(ctx, req)
	if err != nil {
		return nil, err
	}
	sel := selectorString(np.Spec.PodSelector.MatchLabels)
	pods := podsMatchingLabels(req.Snapshot, np.Spec.PodSelector.MatchLabels)
	types := make([]string, 0, len(np.Spec.PolicyTypes))
	for _, t := range np.Spec.PolicyTypes {
		types = append(types, string(t))
	}
	detail := &ObjectDetail{
		Title:    "NetworkPolicy/" + np.Name,
		Category: "network",
		Status:   StatusBadge{Tone: "healthy", Label: strings.Join(types, ", ")},
		Summary: fields(
			"Pod Selector", sel,
			"Policy Types", strings.Join(types, ", "),
			"Affected Pods", fmtInt32(int32(len(pods))),
		),
	}
	var sections []Section
	sections = append(sections, sectionFields("podSelector", "Pod Selector", GroupRelationships, fields(
		"Match Labels", sel,
	)))
	if rows := netpolIngressRows(np.Spec.Ingress); len(rows) > 0 {
		sections = append(sections, sectionTable("ingressRules", "Ingress Rules", GroupSpec,
			[]string{"From", "Ports"}, rows))
	}
	if rows := netpolEgressRows(np.Spec.Egress); len(rows) > 0 {
		sections = append(sections, sectionTable("egressRules", "Egress Rules", GroupSpec,
			[]string{"To", "Ports"}, rows))
	}
	sections = append(sections, sectionFields("policyTypes", "Policy Types", GroupSpec, fields(
		"Types", strings.Join(types, ", "),
	)))
	if len(pods) > 0 {
		var rows [][]string
		for _, p := range pods {
			rows = append(rows, []string{p.Name, p.Phase, boolStr(p.Ready)})
		}
		sections = append(sections, sectionTable("affectedPods", "Affected Pods", GroupRelationships,
			[]string{"Name", "Phase", "Ready"}, rows))
	}
	sections = append(sections, metaSections(np.Labels, np.Annotations, ownerRefsFromMeta(np.OwnerReferences, np.Namespace))...)
	if mf := managedFieldsSection(np.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}

func netpolIngressRows(rules []networkingv1.NetworkPolicyIngressRule) [][]string {
	var rows [][]string
	for _, r := range rules {
		rows = append(rows, []string{peerList(r.From), portList(r.Ports)})
	}
	return rows
}

func netpolEgressRows(rules []networkingv1.NetworkPolicyEgressRule) [][]string {
	var rows [][]string
	for _, r := range rules {
		rows = append(rows, []string{peerList(r.To), portList(r.Ports)})
	}
	return rows
}

func peerList(peers []networkingv1.NetworkPolicyPeer) string {
	if len(peers) == 0 {
		return "*"
	}
	var parts []string
	for _, p := range peers {
		switch {
		case p.PodSelector != nil:
			parts = append(parts, "pod:"+selectorString(p.PodSelector.MatchLabels))
		case p.NamespaceSelector != nil:
			parts = append(parts, "ns:"+selectorString(p.NamespaceSelector.MatchLabels))
		case p.IPBlock != nil:
			parts = append(parts, "cidr:"+p.IPBlock.CIDR)
		}
	}
	return strings.Join(parts, "; ")
}

func portList(ports []networkingv1.NetworkPolicyPort) string {
	if len(ports) == 0 {
		return "*"
	}
	var parts []string
	for _, p := range ports {
		proto := "TCP"
		if p.Protocol != nil {
			proto = string(*p.Protocol)
		}
		port := "*"
		if p.Port != nil {
			port = p.Port.String()
		}
		parts = append(parts, fmt.Sprintf("%s/%s", port, proto))
	}
	return strings.Join(parts, ", ")
}
