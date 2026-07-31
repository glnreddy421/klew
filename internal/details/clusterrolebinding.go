package details

import (
	"context"
)

type clusterRoleBindingProvider struct{}

func (clusterRoleBindingProvider) Kind() string { return "ClusterRoleBinding" }

func (clusterRoleBindingProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	crb, err := getClusterRoleBinding(ctx, req)
	if err != nil {
		return nil, err
	}
	detail := &ObjectDetail{
		Title:    "ClusterRoleBinding/" + crb.Name,
		Category: "access",
		Status:   StatusBadge{Tone: "healthy", Label: "Bound"},
		Summary: fields(
			"Role", crb.RoleRef.Kind+"/"+crb.RoleRef.Name,
			"Subjects", fmtInt32(int32(len(crb.Subjects))),
		),
	}
	var sections []Section
	sections = append(sections, sectionFields("status", "Status", GroupStatus, fields(
		"Subjects", fmtInt32(int32(len(crb.Subjects))),
	)))
	sections = append(sections, sectionFields("roleRef", "RoleRef", GroupRelationships, fields(
		"Kind", crb.RoleRef.Kind,
		"Name", crb.RoleRef.Name,
		"API Group", crb.RoleRef.APIGroup,
	)))
	if rows := subjectRows(crb.Subjects); len(rows) > 0 {
		sections = append(sections, sectionTable("subjects", "Subjects", GroupRelationships,
			[]string{"Kind", "Name", "Namespace", "API Group"}, rows))
	}
	sections = append(sections, metaSections(crb.Labels, crb.Annotations, ownerRefsFromMeta(crb.OwnerReferences, ""))...)
	if mf := managedFieldsSection(crb.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}
