package details

import (
	"context"
	"fmt"
)

type daemonSetProvider struct{}

func (daemonSetProvider) Kind() string { return "DaemonSet" }

func (daemonSetProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	ds, err := getDaemonSet(ctx, req)
	if err != nil {
		return nil, err
	}
	desired := ds.Status.DesiredNumberScheduled
	ready := ds.Status.NumberReady >= desired && desired > 0
	detail := &ObjectDetail{
		Title:    "DaemonSet/" + ds.Name,
		Category: "workload",
		Status:   replicaStatus(ds.Status.NumberReady, desired, ready),
		Summary: fields(
			"Ready", fmt.Sprintf("%d/%d", ds.Status.NumberReady, desired),
			"Updated", fmtInt32(ds.Status.UpdatedNumberScheduled),
			"Available", fmtInt32(ds.Status.NumberAvailable),
		),
	}
	var sections []Section
	sections = append(sections, sectionFields("status", "Status", GroupStatus, fields(
		"Desired", fmtInt32(ds.Status.DesiredNumberScheduled),
		"Current", fmtInt32(ds.Status.CurrentNumberScheduled),
		"Ready", fmtInt32(ds.Status.NumberReady),
		"Updated", fmtInt32(ds.Status.UpdatedNumberScheduled),
		"Available", fmtInt32(ds.Status.NumberAvailable),
		"Misscheduled", fmtInt32(ds.Status.NumberMisscheduled),
	)))
	sections = append(sections, sectionFields("strategy", "Strategy", GroupSpec, fields(
		"Type", string(ds.Spec.UpdateStrategy.Type),
	)))
	if ds.Spec.Selector != nil {
		if sel := selectorString(ds.Spec.Selector.MatchLabels); sel != "" {
			sections = append(sections, sectionFields("selector", "Selector", GroupRelationships, fields("Match Labels", sel)))
		}
	}
	sections = append(sections, podTemplateSections(&ds.Spec.Template, GroupSpec)...)
	sections = append(sections, metaSections(ds.Labels, ds.Annotations, ownerRefsFromMeta(ds.OwnerReferences, ds.Namespace))...)
	if mf := managedFieldsSection(ds.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}
