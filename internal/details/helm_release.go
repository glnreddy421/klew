package details

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/glnreddy421/klew/internal/kube"
)

type helmReleaseProvider struct{}

func (helmReleaseProvider) Kind() string { return "HelmRelease" }

func (helmReleaseProvider) Build(ctx context.Context, req *Request) (*ObjectDetail, error) {
	rec, err := kube.GetHelmRelease(ctx, req.Client, req.Ref.Namespace, req.Ref.Name)
	if err != nil {
		return nil, err
	}

	statusTone := helmStatusTone(rec.Status)
	chartLabel := helmChartLabel(rec)
	detail := &ObjectDetail{
		Title:    rec.Name,
		Category: "helm",
		Status: StatusBadge{
			Tone:  statusTone,
			Label: stringsTitle(rec.Status),
		},
		Summary: fields(
			"Chart", chartLabel,
			"Updated", formatHelmRelative(rec.Updated),
			"Namespace", rec.Namespace,
			"Version", rec.ChartVersion,
			"Revision", strconv.Itoa(rec.Revision),
		),
	}

	var sections []Section
	if rec.ConfigYAML != "" {
		sections = append(sections, Section{
			ID:             "helm-values",
			Title:          "Values",
			Group:          GroupSpec,
			Code:           rec.ConfigYAML,
			AltCode:        rec.UserValuesYAML,
			AltToggleLabel: "User-supplied values only",
		})
	}
	if rec.Notes != "" {
		sections = append(sections, Section{
			ID:    "helm-notes",
			Title: "Notes",
			Group: GroupSummary,
			Notes: splitHelmNotes(rec.Notes),
		})
	}

	if managed, managedErr := listHelmManagedResources(ctx, req, rec); managedErr == nil && len(managed) > 0 {
		sort.Slice(managed, func(i, j int) bool {
			a, b := managed[i], managed[j]
			if a[0] != b[0] {
				return a[0] < b[0]
			}
			return a[1] < b[1]
		})
		sections = append(sections, sectionTable("managed", "Managed Resources", GroupRelationships,
			[]string{"Kind", "Name"}, managed))
	}

	detail.Sections = sections
	return detail, nil
}

func helmChartLabel(rec *kube.HelmReleaseRecord) string {
	if rec == nil {
		return "—"
	}
	if rec.Chart == "" {
		return "—"
	}
	if rec.ChartVersion != "" {
		return rec.Chart + "-" + rec.ChartVersion
	}
	return rec.Chart
}

func helmStatusTone(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "deployed", "superseded":
		return "healthy"
	case "failed", "uninstalled", "uninstalling":
		return "critical"
	case "pending-install", "pending-upgrade", "pending-rollback":
		return "degraded"
	default:
		return "unknown"
	}
}

func stringsTitle(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "Unknown"
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func formatHelmRelative(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.UTC().Format(time.RFC3339)
}

func splitHelmNotes(notes string) []string {
	lines := strings.Split(strings.TrimSpace(notes), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if line != "" {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		return []string{notes}
	}
	return out
}

func listHelmManagedResources(ctx context.Context, req *Request, rec *kube.HelmReleaseRecord) ([][]string, error) {
	c := cs(req)
	if c == nil || rec == nil {
		return nil, fmt.Errorf("no client")
	}
	ns := nsOr(req, rec.Namespace)
	releaseName := rec.Name
	rows := make([][]string, 0)

	matchesHelmRelease := func(labels map[string]string) bool {
		if labels == nil {
			return false
		}
		if strings.TrimSpace(labels["meta.helm.sh/release-name"]) != releaseName {
			return false
		}
		if v := strings.TrimSpace(labels["meta.helm.sh/release-namespace"]); v != "" && v != ns {
			return false
		}
		return true
	}

	if deps, err := c.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{}); err == nil {
		for _, item := range deps.Items {
			if matchesHelmRelease(item.Labels) {
				rows = append(rows, []string{"Deployment", item.Name})
			}
		}
	}
	if sts, err := c.AppsV1().StatefulSets(ns).List(ctx, metav1.ListOptions{}); err == nil {
		for _, item := range sts.Items {
			if matchesHelmRelease(item.Labels) {
				rows = append(rows, []string{"StatefulSet", item.Name})
			}
		}
	}
	if ds, err := c.AppsV1().DaemonSets(ns).List(ctx, metav1.ListOptions{}); err == nil {
		for _, item := range ds.Items {
			if matchesHelmRelease(item.Labels) {
				rows = append(rows, []string{"DaemonSet", item.Name})
			}
		}
	}
	if svcs, err := c.CoreV1().Services(ns).List(ctx, metav1.ListOptions{}); err == nil {
		for _, item := range svcs.Items {
			if matchesHelmRelease(item.Labels) {
				rows = append(rows, []string{"Service", item.Name})
			}
		}
	}
	if cms, err := c.CoreV1().ConfigMaps(ns).List(ctx, metav1.ListOptions{}); err == nil {
		for _, item := range cms.Items {
			if matchesHelmRelease(item.Labels) {
				rows = append(rows, []string{"ConfigMap", item.Name})
			}
		}
	}
	if secs, err := c.CoreV1().Secrets(ns).List(ctx, metav1.ListOptions{}); err == nil {
		for _, item := range secs.Items {
			if matchesHelmRelease(item.Labels) {
				rows = append(rows, []string{"Secret", item.Name})
			}
		}
	}
	if ing, err := c.NetworkingV1().Ingresses(ns).List(ctx, metav1.ListOptions{}); err == nil {
		for _, item := range ing.Items {
			if matchesHelmRelease(item.Labels) {
				rows = append(rows, []string{"Ingress", item.Name})
			}
		}
	}
	return rows, nil
}
