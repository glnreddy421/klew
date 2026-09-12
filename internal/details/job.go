package details

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
)

type jobProvider struct{}

func (jobProvider) Kind() string { return "Job" }

func (jobProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	job, err := getJob(ctx, req)
	if err != nil {
		return nil, err
	}
	tone, label := "healthy", "Active"
	if job.Status.Succeeded > 0 && job.Status.Active == 0 {
		tone, label = "healthy", "Succeeded"
	} else if job.Status.Failed > 0 {
		tone, label = "critical", "Failed"
	} else if job.Status.Active > 0 {
		tone, label = "warning", "Running"
	}
	podRows := podsForJob(ctx, req, job.Name)
	summaryPairs := []string{
		"Active", fmtInt32(job.Status.Active),
		"Succeeded", fmtInt32(job.Status.Succeeded),
		"Failed", fmtInt32(job.Status.Failed),
		"Completions", fmtInt32Ptr(job.Spec.Completions),
	}
	if extra := podSummaryFields(podRows); extra != nil {
		summaryPairs = append(summaryPairs, extra...)
	}
	summaryPairs = appendSchedulingSummaryPairs(summaryPairs, job.Spec.Template.Spec)
	detail := &ObjectDetail{
		Title:    "Job/" + job.Name,
		Category: "workload",
		Status:   StatusBadge{Tone: tone, Label: label},
		Summary:  fields(summaryPairs...),
	}
	statusPairs := []string{
		"Active", fmtInt32(job.Status.Active),
		"Succeeded", fmtInt32(job.Status.Succeeded),
		"Failed", fmtInt32(job.Status.Failed),
		"Start Time", fmtTime(job.Status.StartTime),
		"Completion Time", fmtTime(job.Status.CompletionTime),
	}
	if extra := podSummaryFields(podRows); extra != nil {
		statusPairs = append(statusPairs, extra...)
	}
	var sections []Section
	sections = append(sections, sectionFields("status", "Status", GroupSummary, fields(statusPairs...)))
	sections = append(sections, sectionFields("spec", "Spec", GroupSpec, fields(
		"Completions", fmtInt32Ptr(job.Spec.Completions),
		"Parallelism", fmtInt32Ptr(job.Spec.Parallelism),
		"Backoff Limit", fmtInt32Ptr(job.Spec.BackoffLimit),
		"TTL Seconds After Finished", fmtInt32Ptr(job.Spec.TTLSecondsAfterFinished),
	)))
	if rows := jobConditionRows(job.Status.Conditions); len(rows) > 0 {
		sections = append(sections, sectionTable("conditions", "Conditions", GroupSummary,
			[]string{"Type", "Status", "Reason", "Message"}, rows))
	}
	if ownerSec := ownerRefsRelationshipSection(job.OwnerReferences, job.Namespace); !ownerSec.Empty() {
		sections = append(sections, ownerSec)
	}
	if len(podRows) > 0 {
		sections = append(sections, sectionTable("pods", "Pods", GroupRelationships,
			[]string{"Name", "Phase", "Ready"}, podRows))
	} else if job.Status.Active > 0 || job.Status.Succeeded > 0 || job.Status.Failed > 0 {
		sections = append(sections, Section{
			ID:    "pods",
			Title: "Pods",
			Group: GroupRelationships,
			Notes: []string{"Pods for this Job could not be listed — check RBAC for pods/list in this namespace."},
		})
	} else {
		sections = append(sections, Section{
			ID:    "pods",
			Title: "Pods",
			Group: GroupRelationships,
			Notes: []string{"No Pods have been created for this Job yet."},
		})
	}
	sections = append(sections, podTemplateSections(&job.Spec.Template, GroupSpec)...)
	sections = append(sections, labelsAnnotationsSections(job.Labels, job.Annotations)...)
	if mf := managedFieldsSection(job.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}

func jobConditionRows(conds []batchv1.JobCondition) [][]string {
	var rows [][]string
	for _, c := range conds {
		rows = append(rows, []string{string(c.Type), string(c.Status), c.Reason, truncate(c.Message, 120)})
	}
	return rows
}
