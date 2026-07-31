package details

import (
	"context"
)

type clusterRoleProvider struct{}

func (clusterRoleProvider) Kind() string { return "ClusterRole" }

func (clusterRoleProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	cr, err := getClusterRole(ctx, req)
	if err != nil {
		return nil, err
	}
	detail := &ObjectDetail{
		Title:    "ClusterRole/" + cr.Name,
		Category: "access",
		Status:   StatusBadge{Tone: "healthy", Label: "Active"},
		Summary:  fields("Rules", fmtInt32(int32(len(cr.Rules)))),
	}
	var sections []Section
	sections = append(sections, sectionFields("status", "Status", GroupStatus, fields(
		"Rules", fmtInt32(int32(len(cr.Rules))),
		"Aggregation", boolStr(cr.AggregationRule != nil),
	)))
	if rows := policyRuleRows(cr.Rules); len(rows) > 0 {
		sections = append(sections, sectionTable("rules", "Rules", GroupSpec,
			[]string{"API Groups", "Resources", "Verbs", "Resource Names"}, rows))
	}
	sections = append(sections, metaSections(cr.Labels, cr.Annotations, ownerRefsFromMeta(cr.OwnerReferences, ""))...)
	if mf := managedFieldsSection(cr.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}
