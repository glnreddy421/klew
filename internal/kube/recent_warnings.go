package kube

import (
	"context"
	"sort"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/glnreddy421/klew/v2/internal/model"
	"github.com/glnreddy421/klew/v2/internal/redact"
)

const (
	// RecentWarningsLimit caps the warnings returned to the Overview.
	RecentWarningsLimit = 60
	// recentWarningsListLimit bounds one List call; Warning events are a
	// small share of all events, so this rarely truncates.
	recentWarningsListLimit = 2000
	recentWarningMessageMax = 240
)

// RecentWarnings is the newest Warning events in a browse scope, one row per
// object and reason.
type RecentWarnings struct {
	Events      []model.EventRecord       `json:"events"`
	Total       int                       `json:"total"`
	AccessState model.ResourceAccessState `json:"accessState"`
	Error       string                    `json:"error,omitempty"`
}

// ListRecentWarnings lists Warning events in namespaces ("" lists all
// namespaces), newest first. It only reads. Repeats for the same object and
// reason collapse into one row with the summed count and the latest message.
// Messages are masked with redact (best effort) and shortened.
func ListRecentWarnings(ctx context.Context, c *Client, namespaces []string, limit int) (RecentWarnings, error) {
	out := RecentWarnings{AccessState: model.ResourceAccessUnknown}
	if c == nil || c.Clientset == nil {
		return out, nil
	}
	if limit <= 0 {
		limit = RecentWarningsLimit
	}
	if len(namespaces) == 0 {
		namespaces = []string{""}
	}

	var records []model.EventRecord
	listed := false
	for _, ns := range namespaces {
		list, err := c.Clientset.CoreV1().Events(ns).List(ctx, metav1.ListOptions{
			FieldSelector: "type=Warning",
			Limit:         recentWarningsListLimit,
		})
		if err != nil {
			if apierrors.IsForbidden(err) {
				continue
			}
			out.AccessState = model.ResourceAccessError
			out.Error = err.Error()
			return out, err
		}
		listed = true
		for _, e := range list.Items {
			if e.Type == corev1.EventTypeWarning {
				records = append(records, coreEventToRecord(e))
			}
		}
	}
	if !listed {
		out.AccessState = model.ResourceAccessForbidden
		return out, nil
	}
	out.AccessState = model.ResourceAccessAllowed

	rows := collapseWarnings(records)
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].LastSeen.After(rows[j].LastSeen) })
	out.Total = len(rows)
	if len(rows) > limit {
		rows = rows[:limit]
	}
	for i := range rows {
		rows[i].Message = shortenMessage(redact.String(rows[i].Message), recentWarningMessageMax)
	}
	out.Events = rows
	return out, nil
}

// collapseWarnings merges events for the same object and reason, keeping the
// newest message and the summed count.
func collapseWarnings(in []model.EventRecord) []model.EventRecord {
	byKey := map[string]int{}
	out := make([]model.EventRecord, 0, len(in))
	for _, e := range in {
		if e.Count < 1 {
			e.Count = 1
		}
		o := e.InvolvedObject
		key := o.Kind + "|" + o.Namespace + "|" + o.Name + "|" + e.Reason
		idx, ok := byKey[key]
		if !ok {
			byKey[key] = len(out)
			out = append(out, e)
			continue
		}
		kept := &out[idx]
		kept.Count += e.Count
		if e.FirstSeen != "" && (kept.FirstSeen == "" || e.FirstSeen.Before(kept.FirstSeen)) {
			kept.FirstSeen = e.FirstSeen
		}
		if e.LastSeen.After(kept.LastSeen) {
			kept.LastSeen, kept.Timestamp, kept.Message = e.LastSeen, e.Timestamp, e.Message
		}
	}
	return out
}

func shortenMessage(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max-1]) + "…"
}
