package details

import (
	"context"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
)

type roleProvider struct{}

func (roleProvider) Kind() string { return "Role" }

func (roleProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	role, err := getRole(ctx, req)
	if err != nil {
		return nil, err
	}
	detail := &ObjectDetail{
		Title:    "Role/" + role.Name,
		Category: "access",
		Status:   StatusBadge{Tone: "healthy", Label: "Active"},
		Summary:  fields("Rules", fmtInt32(int32(len(role.Rules)))),
	}
	var sections []Section
	sections = append(sections, sectionFields("status", "Status", GroupStatus, fields(
		"Namespace", role.Namespace,
		"Rules", fmtInt32(int32(len(role.Rules))),
	)))
	if rows := policyRuleRows(role.Rules); len(rows) > 0 {
		sections = append(sections, sectionTable("rules", "Rules", GroupSpec,
			[]string{"API Groups", "Resources", "Verbs", "Resource Names"}, rows))
	}
	sections = append(sections, metaSections(role.Labels, role.Annotations, ownerRefsFromMeta(role.OwnerReferences, role.Namespace))...)
	if mf := managedFieldsSection(role.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}

func policyRuleRows(rules []rbacv1.PolicyRule) [][]string {
	var rows [][]string
	for _, r := range rules {
		rows = append(rows, []string{
			strings.Join(r.APIGroups, ", "),
			strings.Join(r.Resources, ", "),
			strings.Join(r.Verbs, ", "),
			strings.Join(r.ResourceNames, ", "),
		})
	}
	return rows
}
