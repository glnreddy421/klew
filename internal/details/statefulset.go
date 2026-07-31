package details

import (
	"context"
	"fmt"
)

type statefulSetProvider struct{}

func (statefulSetProvider) Kind() string { return "StatefulSet" }

func (statefulSetProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	sts, err := getStatefulSet(ctx, req)
	if err != nil {
		return nil, err
	}
	desired := int32Or(sts.Spec.Replicas, 1)
	ready := sts.Status.ReadyReplicas >= desired
	detail := &ObjectDetail{
		Title:    "StatefulSet/" + sts.Name,
		Category: "workload",
		Status:   replicaStatus(sts.Status.ReadyReplicas, desired, ready),
		Summary: fields(
			"Replicas", fmt.Sprintf("%d/%d", sts.Status.ReadyReplicas, desired),
			"Service Name", sts.Spec.ServiceName,
			"Update Strategy", string(sts.Spec.UpdateStrategy.Type),
		),
	}
	var sections []Section
	sections = append(sections, sectionFields("status", "Status", GroupStatus, fields(
		"Replicas", fmtInt32(sts.Status.Replicas),
		"Ready", fmtInt32(sts.Status.ReadyReplicas),
		"Current", fmtInt32(sts.Status.CurrentReplicas),
		"Updated", fmtInt32(sts.Status.UpdatedReplicas),
		"Available", fmtInt32(sts.Status.AvailableReplicas),
		"Current Revision", sts.Status.CurrentRevision,
		"Update Revision", sts.Status.UpdateRevision,
	)))
	sections = append(sections, sectionFields("strategy", "Strategy", GroupSpec, fields(
		"Type", string(sts.Spec.UpdateStrategy.Type),
		"Pod Management", string(sts.Spec.PodManagementPolicy),
		"Service Name", sts.Spec.ServiceName,
	)))
	if sts.Spec.Selector != nil {
		if sel := selectorString(sts.Spec.Selector.MatchLabels); sel != "" {
			sections = append(sections, sectionFields("selector", "Selector", GroupRelationships, fields("Match Labels", sel)))
		}
	}
	sections = append(sections, podTemplateSections(&sts.Spec.Template, GroupSpec)...)
	if len(sts.Spec.VolumeClaimTemplates) > 0 {
		var rows [][]string
		for _, pvc := range sts.Spec.VolumeClaimTemplates {
			sc := ""
			if pvc.Spec.StorageClassName != nil {
				sc = *pvc.Spec.StorageClassName
			}
			cap := ""
			if q, ok := pvc.Spec.Resources.Requests["storage"]; ok {
				cap = q.String()
			}
			rows = append(rows, []string{pvc.Name, sc, cap})
		}
		sections = append(sections, sectionTable("volumeClaimTemplates", "Volume Claim Templates", GroupSpec,
			[]string{"Name", "StorageClass", "Capacity"}, rows))
	}
	sections = append(sections, metaSections(sts.Labels, sts.Annotations, ownerRefsFromMeta(sts.OwnerReferences, sts.Namespace))...)
	if mf := managedFieldsSection(sts.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}
