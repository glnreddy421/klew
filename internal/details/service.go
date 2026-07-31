package details

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

type serviceProvider struct{}

func (serviceProvider) Kind() string { return "Service" }

func (serviceProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	svc, err := getService(ctx, req)
	if err != nil {
		return nil, err
	}
	detail := &ObjectDetail{
		Title:    "Service/" + svc.Name,
		Category: "network",
		Status:   StatusBadge{Tone: "healthy", Label: string(svc.Spec.Type)},
		Summary: fields(
			"Type", string(svc.Spec.Type),
			"ClusterIP", svc.Spec.ClusterIP,
			"Selector", selectorString(svc.Spec.Selector),
		),
	}
	var sections []Section
	sections = append(sections, sectionFields("type", "Type", GroupSummary, fields(
		"Type", string(svc.Spec.Type),
		"ClusterIP", svc.Spec.ClusterIP,
		"External Name", svc.Spec.ExternalName,
		"External IPs", strings.Join(svc.Spec.ExternalIPs, ", "),
		"LoadBalancer IP", svc.Spec.LoadBalancerIP,
	)))
	if rows := servicePortRows(svc.Spec.Ports); len(rows) > 0 {
		sections = append(sections, sectionTable("ports", "Ports", GroupSpec,
			[]string{"Name", "Port", "Target", "Protocol", "NodePort"}, rows))
	}
	if sel := selectorString(svc.Spec.Selector); sel != "" {
		sections = append(sections, sectionFields("selectors", "Selectors", GroupRelationships, fields("Selector", sel)))
	}
	pods := podsMatchingLabels(req.Snapshot, svc.Spec.Selector)
	if len(pods) > 0 {
		var rows [][]string
		for _, p := range pods {
			rows = append(rows, []string{p.Name, p.Phase, boolStr(p.Ready), p.Node})
		}
		sections = append(sections, sectionTable("targetPods", "Target Pods", GroupRelationships,
			[]string{"Name", "Phase", "Ready", "Node"}, rows))
	}
	if slices, err := getEndpointSlicesForService(ctx, req, svc.Name); err == nil && len(slices) > 0 {
		var rows [][]string
		degraded := false
		for _, es := range slices {
			ready, total := 0, 0
			for _, ep := range es.Endpoints {
				total++
				if ep.Conditions.Ready != nil && *ep.Conditions.Ready {
					ready++
				} else {
					degraded = true
				}
			}
			rows = append(rows, []string{es.Name, fmt.Sprintf("%d/%d", ready, total), string(es.AddressType)})
		}
		sections = append(sections, sectionTable("endpointSlices", "EndpointSlices", GroupRelationships,
			[]string{"Name", "Ready", "Address Type"}, rows))
		if degraded {
			detail.Status = StatusBadge{Tone: "warning", Label: "Endpoints degraded"}
		} else {
			detail.Status = StatusBadge{Tone: "healthy", Label: "Endpoints ready"}
		}
	}
	sections = append(sections, sectionFields("sessionAffinity", "Session Affinity", GroupSpec, fields(
		"Affinity", string(svc.Spec.SessionAffinity),
	)))
	sections = append(sections, sectionFields("trafficPolicy", "Traffic Policy", GroupSpec, fields(
		"External Traffic Policy", string(svc.Spec.ExternalTrafficPolicy),
		"Internal Traffic Policy", internalTrafficPolicy(svc.Spec.InternalTrafficPolicy),
		"IP Family Policy", ipFamilyPolicy(svc.Spec.IPFamilyPolicy),
	)))
	sections = append(sections, metaSections(svc.Labels, svc.Annotations, ownerRefsFromMeta(svc.OwnerReferences, svc.Namespace))...)
	if mf := managedFieldsSection(svc.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}

func servicePortRows(ports []corev1.ServicePort) [][]string {
	var rows [][]string
	for _, p := range ports {
		np := ""
		if p.NodePort != 0 {
			np = fmtInt32(p.NodePort)
		}
		rows = append(rows, []string{p.Name, fmtInt32(p.Port), p.TargetPort.String(), string(p.Protocol), np})
	}
	return rows
}

func internalTrafficPolicy(p *corev1.ServiceInternalTrafficPolicy) string {
	if p == nil {
		return ""
	}
	return string(*p)
}

func ipFamilyPolicy(p *corev1.IPFamilyPolicy) string {
	if p == nil {
		return ""
	}
	return string(*p)
}
