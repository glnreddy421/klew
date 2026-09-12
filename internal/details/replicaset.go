package details

import (
	"context"
	"fmt"
)

type replicaSetProvider struct{}

func (replicaSetProvider) Kind() string { return "ReplicaSet" }

func (replicaSetProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	rs, err := getReplicaSet(ctx, req)
	if err != nil {
		return nil, err
	}
	desired := int32Or(rs.Spec.Replicas, 1)
	ready := rs.Status.ReadyReplicas >= desired
	matchLabels := workloadMatchLabels(rs.Spec.Selector)
	podRows := podsForSelector(ctx, req, matchLabels)
	summaryPairs := appendSchedulingSummaryPairs(appendPodSummaryFields([]string{
		"Replicas", fmt.Sprintf("%d/%d", rs.Status.ReadyReplicas, desired),
		"Fully Labeled", fmtInt32(rs.Status.FullyLabeledReplicas),
	}, podRows), rs.Spec.Template.Spec)

	detail := &ObjectDetail{
		Title:    "ReplicaSet/" + rs.Name,
		Category: "workload",
		Status:   replicaStatus(rs.Status.ReadyReplicas, desired, ready),
		Summary:  fields(summaryPairs...),
	}
	var sections []Section
	sections = append(sections, sectionFields("status", "Status", GroupSummary, fields(
		"Replicas", fmtInt32(rs.Status.Replicas),
		"Ready", fmtInt32(rs.Status.ReadyReplicas),
		"Available", fmtInt32(rs.Status.AvailableReplicas),
		"Fully Labeled", fmtInt32(rs.Status.FullyLabeledReplicas),
		"Observed Generation", fmtInt64(rs.Status.ObservedGeneration),
	)))
	if rs.Spec.Selector != nil {
		if sel := selectorString(rs.Spec.Selector.MatchLabels); sel != "" {
			sections = append(sections, sectionFields("selector", "Selector", GroupRelationships, fields("Match Labels", sel)))
		}
	}
	if sec := podsRelationshipSection(ctx, req, matchLabels, podRows); !sec.Empty() {
		sections = append(sections, sec)
	}
	sections = append(sections, podTemplateSections(&rs.Spec.Template, GroupSpec)...)
	sections = append(sections, metaSections(rs.Labels, rs.Annotations, ownerRefsFromMeta(rs.OwnerReferences, rs.Namespace))...)
	if mf := managedFieldsSection(rs.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}
