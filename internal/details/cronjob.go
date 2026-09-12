package details

import (
	"context"
)

type cronJobProvider struct{}

func (cronJobProvider) Kind() string { return "CronJob" }

func (cronJobProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	cj, err := getCronJob(ctx, req)
	if err != nil {
		return nil, err
	}
	suspended := cj.Spec.Suspend != nil && *cj.Spec.Suspend
	tone, label := "healthy", "Scheduled"
	if suspended {
		tone, label = "warning", "Suspended"
	}
	last := ""
	if cj.Status.LastScheduleTime != nil {
		last = fmtTime(cj.Status.LastScheduleTime)
	}
	jobRows := jobsForCronJob(ctx, req, cj.Name)
	podRows := podsForCronJobFromJobs(ctx, req, jobRows)
	summaryPairs := appendSchedulingSummaryPairs(appendPodSummaryFields([]string{
		"Schedule", cj.Spec.Schedule,
		"Suspend", boolStr(suspended),
		"Last Schedule", last,
		"Active Jobs", fmtInt32(int32(len(cj.Status.Active))),
	}, podRows), cj.Spec.JobTemplate.Spec.Template.Spec)

	detail := &ObjectDetail{
		Title:    "CronJob/" + cj.Name,
		Category: "workload",
		Status:   StatusBadge{Tone: tone, Label: label},
		Summary:  fields(summaryPairs...),
	}
	var sections []Section
	sections = append(sections, sectionFields("status", "Status", GroupSummary, fields(
		"Last Schedule", last,
		"Last Successful", fmtTime(cj.Status.LastSuccessfulTime),
		"Active Jobs", fmtInt32(int32(len(cj.Status.Active))),
	)))
	tz := ""
	if cj.Spec.TimeZone != nil {
		tz = *cj.Spec.TimeZone
	}
	sections = append(sections, sectionFields("spec", "Spec", GroupSpec, fields(
		"Schedule", cj.Spec.Schedule,
		"Time Zone", tz,
		"Suspend", boolStr(suspended),
		"Concurrency Policy", string(cj.Spec.ConcurrencyPolicy),
		"Successful Jobs History", fmtInt32Ptr(cj.Spec.SuccessfulJobsHistoryLimit),
		"Failed Jobs History", fmtInt32Ptr(cj.Spec.FailedJobsHistoryLimit),
	)))
	if ownerSec := ownerRefsRelationshipSection(cj.OwnerReferences, cj.Namespace); !ownerSec.Empty() {
		sections = append(sections, ownerSec)
	}
	if len(jobRows) > 0 {
		sections = append(sections, sectionTable("jobs", "Jobs", GroupRelationships,
			[]string{"Name", "Active", "Succeeded", "Failed", "Start Time"}, jobRows))
	} else if len(cj.Status.Active) > 0 {
		var rows [][]string
		for _, a := range cj.Status.Active {
			rows = append(rows, []string{a.Kind, a.Name, a.Namespace})
		}
		sections = append(sections, sectionTable("activeJobs", "Active Jobs", GroupRelationships,
			[]string{"Kind", "Name", "Namespace"}, rows))
	} else {
		note := "No Jobs found yet — they appear here after the next scheduled run."
		if last != "" {
			note = "No active Jobs. Last schedule: " + last + ". Completed Jobs appear here after each run."
		}
		sections = append(sections, Section{
			ID:    "jobs",
			Title: "Jobs",
			Group: GroupRelationships,
			Notes: []string{note},
		})
	}
	if len(podRows) > 0 {
		sections = append(sections, sectionTable("pods", "Pods", GroupRelationships,
			[]string{"Name", "Phase", "Ready"}, podRows))
	} else if len(jobRows) > 0 {
		sections = append(sections, Section{
			ID:    "pods",
			Title: "Pods",
			Group: GroupRelationships,
			Notes: []string{"Pods for CronJob Jobs could not be listed — check RBAC for pods/list in this namespace."},
		})
	}
	sections = append(sections, podTemplateSections(&cj.Spec.JobTemplate.Spec.Template, GroupSpec)...)
	sections = append(sections, labelsAnnotationsSections(cj.Labels, cj.Annotations)...)
	if mf := managedFieldsSection(cj.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}
