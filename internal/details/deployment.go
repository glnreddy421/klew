package details

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
)

type deploymentProvider struct{}

func (deploymentProvider) Kind() string { return "Deployment" }

func (deploymentProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	d, err := getDeployment(ctx, req)
	if err != nil {
		return nil, err
	}
	ready := d.Status.ReadyReplicas >= int32Or(d.Spec.Replicas, 1) && d.Status.UnavailableReplicas == 0
	detail := &ObjectDetail{
		Title:    "Deployment/" + d.Name,
		Category: "workload",
		Status:   replicaStatus(d.Status.ReadyReplicas, int32Or(d.Spec.Replicas, 1), ready),
		Summary: fields(
			"Replicas", fmt.Sprintf("%d/%d", d.Status.ReadyReplicas, int32Or(d.Spec.Replicas, 1)),
			"Updated", fmtInt32(d.Status.UpdatedReplicas),
			"Available", fmtInt32(d.Status.AvailableReplicas),
			"Strategy", string(d.Spec.Strategy.Type),
		),
	}

	var sections []Section
	sections = append(sections, sectionFields("status", "Status", GroupStatus, fields(
		"Observed Generation", fmtInt64(d.Status.ObservedGeneration),
		"Replicas", fmtInt32(d.Status.Replicas),
		"Ready", fmtInt32(d.Status.ReadyReplicas),
		"Updated", fmtInt32(d.Status.UpdatedReplicas),
		"Available", fmtInt32(d.Status.AvailableReplicas),
		"Unavailable", fmtInt32(d.Status.UnavailableReplicas),
	)))
	sections = append(sections, sectionFields("replicaSummary", "Replica Summary", GroupStatus, fields(
		"Desired", fmtInt32(int32Or(d.Spec.Replicas, 1)),
		"Current", fmtInt32(d.Status.Replicas),
		"Ready", fmtInt32(d.Status.ReadyReplicas),
		"Updated", fmtInt32(d.Status.UpdatedReplicas),
		"Available", fmtInt32(d.Status.AvailableReplicas),
	)))
	sections = append(sections, sectionFields("strategy", "Strategy", GroupSpec, fields(
		"Type", string(d.Spec.Strategy.Type),
		"Max Unavailable", surgeUnavailable(d.Spec.Strategy.RollingUpdate, true),
		"Max Surge", surgeUnavailable(d.Spec.Strategy.RollingUpdate, false),
		"Revision History Limit", fmtInt32Ptr(d.Spec.RevisionHistoryLimit),
		"Progress Deadline", fmtInt32Ptr(d.Spec.ProgressDeadlineSeconds),
	)))
	if rows := deployConditionRows(d.Status.Conditions); len(rows) > 0 {
		sections = append(sections, sectionTable("conditions", "Conditions", GroupStatus,
			[]string{"Type", "Status", "Reason", "Message"}, rows))
	}
	if d.Spec.Selector != nil {
		if sel := selectorString(d.Spec.Selector.MatchLabels); sel != "" {
			sections = append(sections, sectionFields("selector", "Selector", GroupRelationships, fields("Match Labels", sel)))
		}
	}
	sections = append(sections, podTemplateSections(&d.Spec.Template, GroupSpec)...)
	sections = append(sections, metaSections(d.Labels, d.Annotations, ownerRefsFromMeta(d.OwnerReferences, d.Namespace))...)
	if mf := managedFieldsSection(d.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}

func replicaStatus(ready, desired int32, ok bool) StatusBadge {
	if desired == 0 {
		return StatusBadge{Tone: "unknown", Label: "Scaled to 0"}
	}
	if ready >= desired && ok {
		return StatusBadge{Tone: "healthy", Label: "Ready"}
	}
	if ready == 0 {
		return StatusBadge{Tone: "critical", Label: "No ready replicas"}
	}
	return StatusBadge{Tone: "warning", Label: fmt.Sprintf("%d/%d ready", ready, desired)}
}

func surgeUnavailable(ru *appsv1.RollingUpdateDeployment, unavailable bool) string {
	if ru == nil {
		return ""
	}
	if unavailable {
		if ru.MaxUnavailable != nil {
			return ru.MaxUnavailable.String()
		}
		return ""
	}
	if ru.MaxSurge != nil {
		return ru.MaxSurge.String()
	}
	return ""
}

func fmtInt32Ptr(p *int32) string {
	if p == nil {
		return ""
	}
	return fmtInt32(*p)
}

func deployConditionRows(conds []appsv1.DeploymentCondition) [][]string {
	var rows [][]string
	for _, c := range conds {
		rows = append(rows, []string{string(c.Type), string(c.Status), c.Reason, truncate(c.Message, 120)})
	}
	return rows
}
