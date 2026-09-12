package details

import (
	"context"
	"fmt"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func podsForSelector(ctx context.Context, req *Request, matchLabels map[string]string) [][]string {
	if len(matchLabels) == 0 {
		return nil
	}
	if rows := podsForSelectorLive(ctx, req, matchLabels); len(rows) > 0 {
		return rows
	}
	return podsForSelectorSnapshot(req, matchLabels)
}

func podsForSelectorLive(ctx context.Context, req *Request, matchLabels map[string]string) [][]string {
	c := cs(req)
	if c == nil || len(matchLabels) == 0 {
		return nil
	}
	ns := nsOr(req, "")
	list, err := c.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(matchLabels).String(),
		Limit:         batchRelationshipListLimit,
	})
	if err != nil || len(list.Items) == 0 {
		return nil
	}
	rows := make([][]string, 0, len(list.Items))
	for _, pod := range list.Items {
		node := pod.Spec.NodeName
		if node == "" {
			node = "—"
		}
		rows = append(rows, []string{
			pod.Name,
			string(pod.Status.Phase),
			podReadyLabel(pod),
			node,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })
	return rows
}

func podsForSelectorSnapshot(req *Request, matchLabels map[string]string) [][]string {
	pods := podsMatchingLabels(req.Snapshot, matchLabels)
	if len(pods) == 0 {
		return nil
	}
	rows := make([][]string, 0, len(pods))
	for _, p := range pods {
		node := strings.TrimSpace(p.Node)
		if node == "" {
			node = "—"
		}
		rows = append(rows, []string{
			p.Name,
			p.Phase,
			boolStr(p.Ready),
			node,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })
	return rows
}

func podsRelationshipSection(ctx context.Context, req *Request, matchLabels map[string]string, rows [][]string) Section {
	if len(rows) == 0 {
		rows = podsForSelector(ctx, req, matchLabels)
	}
	if len(rows) > 0 {
		return sectionTable("pods", "Pods", GroupRelationships,
			[]string{"Name", "Phase", "Ready", "Node"}, rows)
	}
	if len(matchLabels) == 0 {
		return Section{}
	}
	if cs(req) != nil {
		return Section{
			ID:    "pods",
			Title: "Pods",
			Group: GroupRelationships,
			Notes: []string{"Pods for this workload could not be listed — check RBAC for pods/list in this namespace."},
		}
	}
	return Section{
		ID:    "pods",
		Title: "Pods",
		Group: GroupRelationships,
		Notes: []string{"No Pods match this workload selector yet."},
	}
}

func replicaSetsForSelector(ctx context.Context, req *Request, matchLabels map[string]string) [][]string {
	c := cs(req)
	if c == nil || len(matchLabels) == 0 {
		return nil
	}
	ns := nsOr(req, "")
	list, err := c.AppsV1().ReplicaSets(ns).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(matchLabels).String(),
		Limit:         batchRelationshipListLimit,
	})
	if err != nil || len(list.Items) == 0 {
		return nil
	}
	items := append([]appsv1.ReplicaSet(nil), list.Items...)
	sort.Slice(items, func(i, j int) bool {
		ti := items[i].CreationTimestamp.Time
		tj := items[j].CreationTimestamp.Time
		if ti.Equal(tj) {
			return items[i].Name > items[j].Name
		}
		return ti.After(tj)
	})
	rows := make([][]string, 0, len(items))
	for _, rs := range items {
		desired := int32Or(rs.Spec.Replicas, 1)
		rows = append(rows, []string{
			rs.Name,
			fmtInt32(rs.Status.ReadyReplicas),
			fmt.Sprintf("%d/%d", rs.Status.ReadyReplicas, desired),
			fmtTime(&rs.CreationTimestamp),
		})
	}
	return rows
}

func podsForCronJob(ctx context.Context, req *Request, cronJobName string) [][]string {
	return podsForCronJobFromJobs(ctx, req, jobsForCronJob(ctx, req, cronJobName))
}

func podsForCronJobFromJobs(ctx context.Context, req *Request, jobRows [][]string) [][]string {
	if len(jobRows) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	var rows [][]string
	for _, jobRow := range jobRows {
		if len(jobRow) == 0 {
			continue
		}
		for _, podRow := range podsForJob(ctx, req, jobRow[0]) {
			if len(podRow) == 0 {
				continue
			}
			if _, ok := seen[podRow[0]]; ok {
				continue
			}
			seen[podRow[0]] = struct{}{}
			rows = append(rows, podRow)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })
	return rows
}

func appendPodSummaryFields(summaryPairs []string, podRows [][]string) []string {
	if extra := podSummaryFields(podRows); extra != nil {
		return append(summaryPairs, extra...)
	}
	return summaryPairs
}

func workloadMatchLabels(sel *metav1.LabelSelector) map[string]string {
	if sel == nil || len(sel.MatchLabels) == 0 {
		return nil
	}
	return sel.MatchLabels
}

func podRowsFromItems(pods []corev1.Pod) [][]string {
	if len(pods) == 0 {
		return nil
	}
	rows := make([][]string, 0, len(pods))
	for _, pod := range pods {
		node := pod.Spec.NodeName
		if node == "" {
			node = "—"
		}
		rows = append(rows, []string{
			pod.Name,
			string(pod.Status.Phase),
			podReadyLabel(pod),
			node,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })
	return rows
}
