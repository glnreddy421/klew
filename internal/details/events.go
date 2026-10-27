package details

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

func eventsSection(ctx context.Context, req *Request) Section {
	rows, tags, hist := collectEventRows(ctx, req)
	if len(rows) == 0 && hist == nil {
		return Section{}
	}
	sec := sectionTable("events", "Events", GroupEvents,
		[]string{"Time", "Type", "Reason", "Message", "Count"}, rows)
	if hasTag(tags) {
		sec.Table.RowTags = tags
	}
	sec.History = hist
	if len(rows) == 0 {
		sec.Notes = append(sec.Notes, "No events recorded for this object.")
	}
	return sec
}

func hasTag(tags []string) bool {
	for _, t := range tags {
		if t != "" {
			return true
		}
	}
	return false
}

type eventRow struct {
	ts       time.Time
	cells    []string
	recorded bool
}

func collectEventRows(ctx context.Context, req *Request) ([][]string, []string, *model.EventHistoryCoverage) {
	if req == nil {
		return nil, nil, nil
	}
	kind := req.Ref.Kind
	name := req.Ref.Name
	var collected []eventRow
	seen := map[string]bool{}
	seenUID := map[string]bool{}
	now := time.Now()

	push := func(ts time.Time, typ, reason, msg string, count int32, uid string, recorded bool) {
		if uid != "" {
			if seenUID[uid] {
				return
			}
			seenUID[uid] = true
		}
		key := typ + "|" + reason + "|" + msg
		if seen[key] {
			return
		}
		seen[key] = true
		c := ""
		if count > 1 {
			c = fmtInt32(count)
		}
		collected = append(collected, eventRow{
			ts:       ts,
			cells:    []string{formatEventTime(ts, now), typ, reason, truncate(msg, 160), c},
			recorded: recorded,
		})
	}

	// Lifetime start for gap reporting: earliest event about the object.
	var earliest time.Time
	note := func(t time.Time) {
		if !t.IsZero() && (earliest.IsZero() || t.Before(earliest)) {
			earliest = t
		}
	}

	for _, e := range req.Snapshot.Events {
		obj := e.InvolvedObject
		if !objectTouches(obj.Kind, obj.Name, kind, name) {
			continue
		}
		note(e.FirstSeen.Time())
		push(e.Timestamp.Time(), e.Type, e.Reason, e.Message, e.Count, e.UID, e.Recorded)
	}
	for _, e := range req.State.LiveEvidence {
		t := strings.ToLower(string(e.SourceType))
		if t != "k8s_event" && t != "object_change" {
			continue
		}
		if !evidenceTouches(e, kind, name) {
			continue
		}
		push(e.Timestamp.Time(), string(e.SourceType), e.Reason, e.Message, clampInt32(int64(e.Count)), "", false)
	}

	var hist *model.EventHistoryCoverage
	if c := req.Client; c != nil && c.History != nil && c.Config != nil {
		recs, cov, err := c.History.RecordedEvents(ctx, c.Context, c.Config.Host, model.EventHistoryQuery{
			Namespace:     req.Ref.Namespace,
			RegardingName: name,
			NamePrefix:    isWorkload(kind),
			Limit:         200,
		})
		if err == nil {
			for _, e := range recs {
				obj := e.InvolvedObject
				if !objectTouches(obj.Kind, obj.Name, kind, name) {
					continue
				}
				note(e.FirstSeen.Time())
				push(e.Timestamp.Time(), e.Type, e.Reason, e.Message, e.Count, e.UID, true)
			}
			if cov.Since != "" {
				start := earliest
				if start.IsZero() || start.Before(cov.Since.Time()) {
					start = cov.Since.Time()
				}
				cov.Gaps = gapsSince(cov.Gaps, start)
				hist = &cov
			}
		}
	}

	// Newest first, like kubectl describe's most recent rows at the top of a long list.
	sort.SliceStable(collected, func(i, j int) bool { return collected[i].ts.After(collected[j].ts) })
	if len(collected) > 40 {
		collected = collected[:40]
	}
	rows := make([][]string, 0, len(collected))
	tags := make([]string, 0, len(collected))
	for _, r := range collected {
		rows = append(rows, r.cells)
		if r.recorded {
			tags = append(tags, "recorded")
		} else {
			tags = append(tags, "")
		}
	}
	return rows, tags, hist
}

// formatEventTime keeps the existing HH:MM:SS for today and adds the date for
// older (recorded) events.
func formatEventTime(ts, now time.Time) string {
	if ts.IsZero() {
		return "—"
	}
	u := ts.UTC()
	if y, m, d := u.Date(); y == now.UTC().Year() && m == now.UTC().Month() && d == now.UTC().Day() {
		return u.Format("15:04:05")
	}
	return u.Format("Jan 2 15:04")
}

// gapsSince keeps gaps that end after start, clipped to start.
func gapsSince(gaps []model.TimeRange, start time.Time) []model.TimeRange {
	var out []model.TimeRange
	for _, g := range gaps {
		to := g.To.Time()
		if !to.After(start) {
			continue
		}
		if g.From.Time().Before(start) {
			g.From = model.TimestampFrom(start)
		}
		out = append(out, g)
	}
	return out
}

func objectTouches(objKind, objName, kind, name string) bool {
	if objName == "" {
		return false
	}
	if objName == name {
		if objKind == "" || objKind == kind {
			return true
		}
		return kindAliases(objKind, kind)
	}
	// Workload → pod prefix
	if isWorkload(kind) && (objKind == "Pod" || objKind == "") && strings.HasPrefix(objName, name+"-") {
		return true
	}
	return false
}

func evidenceTouches(e model.EvidenceEvent, kind, name string) bool {
	if e.SourceKind != "" && e.SourceName != "" && objectTouches(e.SourceKind, e.SourceName, kind, name) {
		return true
	}
	if kind == "Pod" && (e.Pod == name || e.SourceName == name) {
		return true
	}
	if isWorkload(kind) && e.Pod != "" && strings.HasPrefix(e.Pod, name+"-") {
		return true
	}
	for _, ref := range e.RelatedObjectRefs {
		if objectTouches(ref.Kind, ref.Name, kind, name) {
			return true
		}
	}
	return false
}

func isWorkload(kind string) bool {
	switch kind {
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "Job", "CronJob":
		return true
	default:
		return false
	}
}

func kindAliases(a, b string) bool {
	norm := func(k string) string {
		switch strings.ToLower(k) {
		case "pvc", "persistentvolumeclaim":
			return "PersistentVolumeClaim"
		case "pv", "persistentvolume":
			return "PersistentVolume"
		case "hpa", "horizontalpodautoscaler":
			return "HorizontalPodAutoscaler"
		case "sc", "storageclass":
			return "StorageClass"
		default:
			return k
		}
	}
	return norm(a) == norm(b)
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
