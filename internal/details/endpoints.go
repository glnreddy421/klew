package details

import (
	"context"
)

type endpointsProvider struct{}

func (endpointsProvider) Kind() string { return "Endpoints" }

func (endpointsProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	ep, err := getEndpoints(ctx, req)
	if err != nil {
		return nil, err
	}

	addrRows := legacyEndpointAddressRows(ep.Subsets)
	portRows := legacyEndpointPortRows(ep.Subsets)
	ready, total := 0, len(addrRows)
	for _, row := range addrRows {
		if len(row) > 4 && row[4] == "True" {
			ready++
		}
	}

	detail := &ObjectDetail{
		Title:    "Endpoints/" + ep.Name,
		Category: "network",
		Status:   endpointStatusBadge(ready, total),
		Summary: fields(
			"Addresses", fmtInt32(int32(total)),
			"Ready", fmtInt32(int32(ready)),
			"Subsets", fmtInt32(int32(len(ep.Subsets))),
		),
	}

	var sections []Section
	sections = append(sections, sectionFields("status", "Status", GroupStatus, fields(
		"Ready Addresses", fmtInt32(int32(ready)),
		"Total Addresses", fmtInt32(int32(total)),
		"Subsets", fmtInt32(int32(len(ep.Subsets))),
	)))
	if len(addrRows) > 0 {
		sections = append(sections, sectionTable("addresses", "Addresses", GroupRelationships,
			[]string{"IP", "Hostname", "Node", "Target", "Ready"}, addrRows))
	}
	if len(portRows) > 0 {
		sections = append(sections, sectionTable("ports", "Ports", GroupSpec,
			[]string{"Name", "Port", "Protocol", "App Protocol"}, portRows))
	}
	sections = append(sections, Section{
		ID:    "deprecation",
		Title: "Note",
		Group: GroupSpec,
		Notes: []string{
			"v1 Endpoints is deprecated in Kubernetes v1.33+. Prefer discovery.k8s.io/v1 EndpointSlice for new integrations.",
		},
	})
	sections = append(sections, metaSections(ep.Labels, ep.Annotations, ownerRefsFromMeta(ep.OwnerReferences, ep.Namespace))...)
	detail.Sections = sections
	return detail, nil
}
