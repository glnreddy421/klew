package details

import (
	"context"

	rbacv1 "k8s.io/api/rbac/v1"
)

type roleBindingProvider struct{}

func (roleBindingProvider) Kind() string { return "RoleBinding" }

func (roleBindingProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	rb, err := getRoleBinding(ctx, req)
	if err != nil {
		return nil, err
	}
	detail := &ObjectDetail{
		Title:    "RoleBinding/" + rb.Name,
		Category: "access",
		Status:   StatusBadge{Tone: "healthy", Label: "Bound"},
		Summary: fields(
			"Role", rb.RoleRef.Kind+"/"+rb.RoleRef.Name,
			"Subjects", fmtInt32(int32(len(rb.Subjects))),
		),
	}
	var sections []Section
	sections = append(sections, sectionFields("status", "Status", GroupStatus, fields(
		"Namespace", rb.Namespace,
		"Subjects", fmtInt32(int32(len(rb.Subjects))),
	)))
	sections = append(sections, sectionFields("roleRef", "RoleRef", GroupRelationships, fields(
		"Kind", rb.RoleRef.Kind,
		"Name", rb.RoleRef.Name,
		"API Group", rb.RoleRef.APIGroup,
	)))
	if rows := subjectRows(rb.Subjects); len(rows) > 0 {
		sections = append(sections, sectionTable("subjects", "Subjects", GroupRelationships,
			[]string{"Kind", "Name", "Namespace", "API Group"}, rows))
	}
	sections = append(sections, metaSections(rb.Labels, rb.Annotations, ownerRefsFromMeta(rb.OwnerReferences, rb.Namespace))...)
	if mf := managedFieldsSection(rb.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}

func subjectRows(subjects []rbacv1.Subject) [][]string {
	var rows [][]string
	for _, s := range subjects {
		rows = append(rows, []string{s.Kind, s.Name, s.Namespace, s.APIGroup})
	}
	return rows
}
