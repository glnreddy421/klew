package details

import (
	"context"
	"sort"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const batchRelationshipListLimit = 100

func ownerRefsRelationshipSection(owners []metav1.OwnerReference, ns string) Section {
	rows := ownerRows(ownerRefsFromMeta(owners, ns))
	if len(rows) == 0 {
		return Section{}
	}
	return sectionTable("ownerRefs", "Owner References", GroupRelationships,
		[]string{"Kind", "Name", "Namespace", "UID"}, rows)
}

func podsForJob(ctx context.Context, req *Request, jobName string) [][]string {
	c := cs(req)
	if c == nil || strings.TrimSpace(jobName) == "" {
		return nil
	}
	ns := nsOr(req, "")
	selectors := []string{
		"batch.kubernetes.io/job-name=" + jobName,
		"job-name=" + jobName,
	}
	seen := map[string]struct{}{}
	var rows [][]string
	for _, sel := range selectors {
		list, err := c.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
			LabelSelector: sel,
			Limit:         batchRelationshipListLimit,
		})
		if err != nil {
			continue
		}
		for _, pod := range list.Items {
			if _, ok := seen[pod.Name]; ok {
				continue
			}
			seen[pod.Name] = struct{}{}
			rows = append(rows, []string{
				pod.Name,
				string(pod.Status.Phase),
				podReadyLabel(pod),
			})
		}
		if len(rows) > 0 {
			break
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })
	return rows
}

func jobsForCronJob(ctx context.Context, req *Request, cronJobName string) [][]string {
	c := cs(req)
	if c == nil || strings.TrimSpace(cronJobName) == "" {
		return nil
	}
	ns := nsOr(req, "")
	list, err := c.BatchV1().Jobs(ns).List(ctx, metav1.ListOptions{
		LabelSelector: "batch.kubernetes.io/cronjob-name=" + cronJobName,
		Limit:         batchRelationshipListLimit,
	})
	if err != nil || len(list.Items) == 0 {
		return nil
	}
	items := append([]batchv1.Job(nil), list.Items...)
	sort.Slice(items, func(i, j int) bool {
		ti := items[i].CreationTimestamp.Time
		tj := items[j].CreationTimestamp.Time
		if ti.Equal(tj) {
			return items[i].Name > items[j].Name
		}
		return ti.After(tj)
	})
	rows := make([][]string, 0, len(items))
	for _, job := range items {
		rows = append(rows, []string{
			job.Name,
			fmtInt32(job.Status.Active),
			fmtInt32(job.Status.Succeeded),
			fmtInt32(job.Status.Failed),
			fmtTime(job.Status.StartTime),
		})
	}
	return rows
}

func podSummaryFields(rows [][]string) []string {
	if len(rows) == 0 {
		return nil
	}
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		if len(row) == 0 || strings.TrimSpace(row[0]) == "" {
			continue
		}
		names = append(names, row[0])
	}
	if len(names) == 0 {
		return nil
	}
	if len(names) == 1 {
		return []string{"Pod", names[0]}
	}
	return []string{"Pods", strings.Join(names, ", ")}
}

func podReadyLabel(pod corev1.Pod) string {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady {
			return string(cond.Status)
		}
	}
	switch pod.Status.Phase {
	case corev1.PodSucceeded:
		return "True"
	case corev1.PodFailed:
		return "False"
	default:
		return "Unknown"
	}
}
