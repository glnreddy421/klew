package details

import (
	"context"
	"fmt"
	"strings"

	discoveryv1 "k8s.io/api/discovery/v1"
)

type endpointSliceProvider struct{}

func (endpointSliceProvider) Kind() string { return "EndpointSlice" }

func (endpointSliceProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	es, err := getEndpointSlice(ctx, req)
	if err != nil {
		return nil, err
	}
	svcName := es.Labels["kubernetes.io/service-name"]
	ready, total := 0, len(es.Endpoints)
	for _, ep := range es.Endpoints {
		if ep.Conditions.Ready != nil && *ep.Conditions.Ready {
			ready++
		}
	}
	tone := "healthy"
	if total == 0 || ready < total {
		tone = "warning"
	}
	detail := &ObjectDetail{
		Title:    "EndpointSlice/" + es.Name,
		Category: "network",
		Status:   StatusBadge{Tone: tone, Label: fmt.Sprintf("%d/%d ready", ready, total)},
		Summary: fields(
			"Service", svcName,
			"Address Type", string(es.AddressType),
			"Ready", fmt.Sprintf("%d/%d", ready, total),
		),
	}
	var sections []Section
	sections = append(sections, sectionFields("status", "Status", GroupStatus, fields(
		"Address Type", string(es.AddressType),
		"Ready Endpoints", fmtInt32(int32(ready)),
		"Total Endpoints", fmtInt32(int32(total)),
		"Service", svcName,
	)))
	if rows := endpointRows(es.Endpoints); len(rows) > 0 {
		sections = append(sections, sectionTable("endpoints", "Endpoints", GroupRelationships,
			[]string{"Addresses", "Ready", "Node", "Zone", "Target"}, rows))
	}
	if rows := endpointPortRows(es.Ports); len(rows) > 0 {
		sections = append(sections, sectionTable("ports", "Ports", GroupSpec,
			[]string{"Name", "Port", "Protocol", "App Protocol"}, rows))
	}
	sections = append(sections, metaSections(es.Labels, es.Annotations, ownerRefsFromMeta(es.OwnerReferences, es.Namespace))...)
	detail.Sections = sections
	return detail, nil
}

func endpointRows(eps []discoveryv1.Endpoint) [][]string {
	var rows [][]string
	for _, ep := range eps {
		ready := ""
		if ep.Conditions.Ready != nil {
			ready = boolStr(*ep.Conditions.Ready)
		}
		node, zone := "", ""
		if ep.NodeName != nil {
			node = *ep.NodeName
		}
		if ep.Zone != nil {
			zone = *ep.Zone
		}
		target := ""
		if ep.TargetRef != nil {
			target = ep.TargetRef.Kind + "/" + ep.TargetRef.Name
		}
		rows = append(rows, []string{strings.Join(ep.Addresses, ","), ready, node, zone, target})
	}
	return rows
}

func endpointPortRows(ports []discoveryv1.EndpointPort) [][]string {
	var rows [][]string
	for _, p := range ports {
		name, port, proto, app := "", "", "", ""
		if p.Name != nil {
			name = *p.Name
		}
		if p.Port != nil {
			port = fmtInt32(*p.Port)
		}
		if p.Protocol != nil {
			proto = string(*p.Protocol)
		}
		if p.AppProtocol != nil {
			app = *p.AppProtocol
		}
		rows = append(rows, []string{name, port, proto, app})
	}
	return rows
}
