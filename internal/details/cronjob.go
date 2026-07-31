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
	detail := &ObjectDetail{
		Title:    "CronJob/" + cj.Name,
		Category: "workload",
		Status:   StatusBadge{Tone: tone, Label: label},
		Summary: fields(
			"Schedule", cj.Spec.Schedule,
			"Suspend", boolStr(suspended),
			"Last Schedule", last,
			"Active Jobs", fmtInt32(int32(len(cj.Status.Active))),
		),
	}
	var sections []Section
	sections = append(sections, sectionFields("status", "Status", GroupStatus, fields(
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
	if len(cj.Status.Active) > 0 {
		var rows [][]string
		for _, a := range cj.Status.Active {
			rows = append(rows, []string{a.Kind, a.Name, a.Namespace})
		}
		sections = append(sections, sectionTable("activeJobs", "Active Jobs", GroupRelationships,
			[]string{"Kind", "Name", "Namespace"}, rows))
	}
	sections = append(sections, podTemplateSections(&cj.Spec.JobTemplate.Spec.Template, GroupSpec)...)
	sections = append(sections, metaSections(cj.Labels, cj.Annotations, ownerRefsFromMeta(cj.OwnerReferences, cj.Namespace))...)
	if mf := managedFieldsSection(cj.ManagedFields); !mf.Empty() {
		sections = append(sections, mf)
	}
	detail.Sections = sections
	return detail, nil
}
