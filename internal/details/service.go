package details

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
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
		"Cluster IP", svc.Spec.ClusterIP,
		"Cluster IPs", joinStringSlice(svc.Spec.ClusterIPs),
		"External Name", svc.Spec.ExternalName,
		"External IPs", strings.Join(svc.Spec.ExternalIPs, ", "),
		"LoadBalancer IP", svc.Spec.LoadBalancerIP,
		"IP Families", joinStringSlice(ipFamilies(svc)),
		"IP Family Policy", ipFamilyPolicy(svc.Spec.IPFamilyPolicy),
		"Session Affinity", string(svc.Spec.SessionAffinity),
		"Publish Not Ready", boolStr(svc.Spec.PublishNotReadyAddresses),
	)))
	if rows := servicePortRows(svc.Spec.Ports); len(rows) > 0 {
		sections = append(sections, sectionTable("ports", "Ports", GroupSpec,
			[]string{"Name", "Port", "Target", "Protocol", "NodePort"}, rows))
	}
	if sel := selectorString(svc.Spec.Selector); sel != "" {
		sections = append(sections, sectionFields("selectors", "Selectors", GroupRelationships, fields("Selector", sel)))
	}
	if rows := serviceLoadBalancerIngressRows(svc); len(rows) > 0 {
		sections = append(sections, sectionTable("loadBalancer", "Load Balancer", GroupStatus,
			[]string{"IP", "Hostname", "IP Mode", "Ports"}, rows))
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

	endpointSections, endpointStatus := serviceEndpointSections(ctx, req, svc)
	sections = append(sections, endpointSections...)
	if endpointStatus != nil && string(svc.Spec.Type) != "ExternalName" {
		detail.Status = *endpointStatus
	}

	sections = append(sections, sectionFields("trafficPolicy", "Traffic Policy", GroupSpec, fields(
		"External Traffic Policy", string(svc.Spec.ExternalTrafficPolicy),
		"Internal Traffic Policy", internalTrafficPolicy(svc.Spec.InternalTrafficPolicy),
	)))
	sections = append(sections, metaSections(svc.Labels, svc.Annotations, ownerRefsFromMeta(svc.OwnerReferences, svc.Namespace))...)
	if mf := managedFieldsSection(svc.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}

func serviceEndpointSections(ctx context.Context, req *Request, svc *corev1.Service) ([]Section, *StatusBadge) {
	svcName := svc.Name
	var sections []Section
	var badge *StatusBadge

	slices, sliceErr := getEndpointSlicesForService(ctx, req, svcName)
	var legacyEP *corev1.Endpoints
	if sliceErr != nil || len(slices) == 0 {
		if ep, epErr := getEndpointsByName(ctx, req, svcName); epErr == nil {
			legacyEP = ep
		}
	}

	if parts := serviceEndpointSummaryParts(slices, legacyEP, svc); len(parts) > 0 {
		sections = append(sections, sectionFields("serviceEndpoint", "Endpoint", GroupSummary, fields(
			"Name", svcName,
			"Endpoints", strings.Join(parts, ", "),
		)))
	}

	if sliceErr == nil && len(slices) > 0 {
		ready, total := countEndpointSliceAddressTotals(slices)
		b := endpointStatusBadge(ready, total)
		badge = &b
		sections = append(sections, endpointSliceDetailSections(slices)...)
		return sections, badge
	}

	if legacyEP != nil && len(legacyEP.Subsets) > 0 {
		addrRows := legacyEndpointAddressRows(legacyEP.Subsets)
		ready, total := 0, len(addrRows)
		for _, row := range addrRows {
			if len(row) > 4 && row[4] == "True" {
				ready++
			}
		}
		b := endpointStatusBadge(ready, total)
		badge = &b
		sections = append(sections, legacyEndpointDetailSections(legacyEP)...)
		return sections, badge
	}
	return sections, badge
}

func getEndpointsByName(ctx context.Context, req *Request, name string) (*corev1.Endpoints, error) {
	clone := *req
	clone.Ref = req.Ref
	clone.Ref.Name = name
	return getEndpoints(ctx, &clone)
}

func endpointSliceDetailSections(slices []discoveryv1.EndpointSlice) []Section {
	var sections []Section
	var sliceRows [][]string
	var allEndpoints []discoveryv1.Endpoint
	var allPorts []discoveryv1.EndpointPort

	for _, es := range slices {
		ready, total := countEndpointSliceAddresses(es)
		sliceRows = append(sliceRows, []string{
			es.Name,
			string(es.AddressType),
			fmt.Sprintf("%d/%d", ready, total),
			es.GetNamespace(),
		})
		allEndpoints = append(allEndpoints, es.Endpoints...)
		allPorts = append(allPorts, es.Ports...)
	}

	if len(sliceRows) > 0 {
		sections = append(sections, sectionTable("endpointSlices", "EndpointSlices", GroupRelationships,
			[]string{"Name", "Address Type", "Ready", "Namespace"}, sliceRows))
	}
	if rows := endpointRows(allEndpoints); len(rows) > 0 {
		sections = append(sections, sectionTable("backendAddresses", "Backend Addresses", GroupRelationships,
			[]string{"Addresses", "Ready", "Node", "Zone", "Target"}, rows))
	}
	if rows := endpointPortRows(allPorts); len(rows) > 0 {
		sections = append(sections, sectionTable("backendPorts", "Backend Ports", GroupSpec,
			[]string{"Name", "Port", "Protocol", "App Protocol"}, rows))
	}
	return sections
}

func legacyEndpointDetailSections(ep *corev1.Endpoints) []Section {
	var sections []Section
	addrRows := legacyEndpointAddressRows(ep.Subsets)
	portRows := legacyEndpointPortRows(ep.Subsets)
	ready, total := 0, len(addrRows)
	for _, row := range addrRows {
		if len(row) > 4 && row[4] == "True" {
			ready++
		}
	}
	sections = append(sections, sectionFields("legacyEndpoints", "Legacy Endpoints", GroupStatus, fields(
		"Ready Addresses", fmtInt32(int32(ready)),
		"Total Addresses", fmtInt32(int32(total)),
		"Subsets", fmtInt32(int32(len(ep.Subsets))),
	)))
	if len(addrRows) > 0 {
		sections = append(sections, sectionTable("legacyAddresses", "Addresses", GroupRelationships,
			[]string{"IP", "Hostname", "Node", "Target", "Ready"}, addrRows))
	}
	if len(portRows) > 0 {
		sections = append(sections, sectionTable("legacyPorts", "Ports", GroupSpec,
			[]string{"Name", "Port", "Protocol", "App Protocol"}, portRows))
	}
	sections = append(sections, Section{
		ID:    "endpointDeprecation",
		Title: "Note",
		Group: GroupSpec,
		Notes: []string{
			"Showing v1 Endpoints subsets. On Kubernetes v1.33+ prefer EndpointSlice objects for full routing data.",
		},
	})
	return sections
}

func ipFamilies(svc *corev1.Service) []string {
	if svc == nil {
		return nil
	}
	out := make([]string, 0, len(svc.Spec.IPFamilies))
	for _, fam := range svc.Spec.IPFamilies {
		out = append(out, string(fam))
	}
	return out
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
