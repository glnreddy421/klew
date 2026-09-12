package details

import (
	"context"
	"strings"
)

type namespaceProvider struct{}

func (namespaceProvider) Kind() string { return "Namespace" }

func (namespaceProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	ns, err := getNamespace(ctx, req)
	if err != nil {
		return nil, err
	}
	phase := string(ns.Status.Phase)
	tone := "healthy"
	if phase != "" && phase != "Active" {
		tone = "warning"
	}
	detail := &ObjectDetail{
		Title:    "Namespace/" + ns.Name,
		Category: "cluster",
		Status:   StatusBadge{Tone: tone, Label: phase},
		Summary:  fields("Phase", phase),
	}
	var sections []Section
	finalizers := make([]string, 0, len(ns.Spec.Finalizers))
	for _, f := range ns.Spec.Finalizers {
		finalizers = append(finalizers, string(f))
	}
	sections = append(sections, sectionFields("spec", "Spec", GroupSpec, fields(
		"Finalizers", strings.Join(finalizers, ", "),
	)))
	sections = append(sections, sectionFields("status", "Status", GroupSummary, fields("Phase", phase)))
	if len(ns.Status.Conditions) > 0 {
		var rows [][]string
		for _, c := range ns.Status.Conditions {
			rows = append(rows, []string{string(c.Type), string(c.Status), c.Reason, truncate(c.Message, 120)})
		}
		sections = append(sections, sectionTable("conditions", "Conditions", GroupSummary,
			[]string{"Type", "Status", "Reason", "Message"}, rows))
	}
	var podRows [][]string
	for _, p := range req.Snapshot.Pods {
		if p.Namespace == ns.Name {
			podRows = append(podRows, []string{p.Name, p.Phase, boolStr(p.Ready)})
		}
	}
	if len(podRows) > 0 {
		sections = append(sections, sectionTable("pods", "Pods", GroupRelationships,
			[]string{"Name", "Phase", "Ready"}, podRows))
	}
	sections = append(sections, metaSections(ns.Labels, ns.Annotations, ownerRefsFromMeta(ns.OwnerReferences, ""))...)
	if mf := managedFieldsSection(ns.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}
