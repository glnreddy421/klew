package details

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type serviceAccountProvider struct{}

func (serviceAccountProvider) Kind() string { return "ServiceAccount" }

func (serviceAccountProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	sa, err := getServiceAccount(ctx, req)
	if err != nil {
		return nil, err
	}
	detail := &ObjectDetail{
		Title:    "ServiceAccount/" + sa.Name,
		Category: "access",
		Status:   StatusBadge{Tone: "healthy", Label: "Active"},
		Summary: fields(
			"Secrets", fmtInt32(int32(len(sa.Secrets))),
			"Image Pull Secrets", fmtInt32(int32(len(sa.ImagePullSecrets))),
		),
	}
	var sections []Section
	if len(sa.Secrets) > 0 {
		var rows [][]string
		for _, s := range sa.Secrets {
			rows = append(rows, []string{s.Name})
		}
		sections = append(sections, sectionTable("secrets", "Secrets", GroupRelationships,
			[]string{"Name"}, rows))
	}
	if len(sa.ImagePullSecrets) > 0 {
		var rows [][]string
		for _, s := range sa.ImagePullSecrets {
			rows = append(rows, []string{s.Name})
		}
		sections = append(sections, sectionTable("imagePullSecrets", "Image Pull Secrets", GroupRelationships,
			[]string{"Name"}, rows))
	}
	if rows := podsUsingServiceAccount(ctx, req, sa.Name); len(rows) > 0 {
		sections = append(sections, sectionTable("referencedByPods", "Referenced By Pods", GroupRelationships,
			[]string{"Pod", "Phase"}, rows))
	}
	sections = append(sections, metaSections(sa.Labels, sa.Annotations, ownerRefsFromMeta(sa.OwnerReferences, sa.Namespace))...)
	if mf := managedFieldsSection(sa.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}

func podsUsingServiceAccount(ctx context.Context, req *Request, saName string) [][]string {
	c := cs(req)
	if c == nil {
		return nil
	}
	list, err := c.CoreV1().Pods(nsOr(req, "")).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}
	var rows [][]string
	for _, p := range list.Items {
		if p.Spec.ServiceAccountName == saName || (saName == "default" && p.Spec.ServiceAccountName == "") {
			rows = append(rows, []string{p.Name, string(p.Status.Phase)})
		}
	}
	return rows
}
