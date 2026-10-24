package changes

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

// HelmRevision is one decoded Helm release revision (user-supplied values only).
type HelmRevision struct {
	Revision     int
	Time         time.Time
	Chart        string
	ChartVersion string
	AppVersion   string
	Status       string
	Description  string
	Values       map[string]interface{}
}

// helmMatchWindow pairs a Helm upgrade with the rollout it caused.
const helmMatchWindow = 2 * time.Minute

// HelmChanges diffs consecutive Helm revisions (values and chart version),
// newest first, the latest marked current.
func HelmChanges(revs []HelmRevision) []model.WorkloadChange {
	sorted := append([]HelmRevision(nil), revs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Revision < sorted[j].Revision })
	out := make([]model.WorkloadChange, 0, len(sorted))
	for i, r := range sorted {
		var diffs []model.FieldDiff
		if i > 0 {
			diffs = DiffHelmRevisions(sorted[i-1], r)
		}
		rev := r.Revision
		ch := model.WorkloadChange{
			Time: model.TimestampFrom(r.Time), Source: model.ChangeSourceHelm, Actor: "helm",
			ChangeCause: r.Description, Diffs: diffs, HelmRevision: &rev,
		}
		switch {
		case i == 0:
			ch.Summary = fmt.Sprintf("Helm rev %d · %s %s", r.Revision, r.Chart, r.ChartVersion)
		case len(diffs) == 0:
			ch.Summary = fmt.Sprintf("Helm rev %d · no value changes", r.Revision)
		default:
			ch.Summary = Summarize(diffs)
		}
		out = append(out, ch)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	if len(out) > 0 {
		out[0].Current = true
	}
	return out
}

// DiffHelmRevisions compares chart version and user-supplied values. Values
// are flattened to dotted paths and redacted like env vars.
func DiffHelmRevisions(prev, next HelmRevision) []model.FieldDiff {
	var out []model.FieldDiff
	pc := strings.TrimSpace(prev.Chart + " " + prev.ChartVersion)
	nc := strings.TrimSpace(next.Chart + " " + next.ChartVersion)
	out = appendChanged(out, "chart", pc, nc, AreaChart)
	pv, nv := flattenValues("", prev.Values), flattenValues("", next.Values)
	redactPath := func(path, v string) string { return RedactValue(path, v) }
	return append(out, diffStringMap("values", pv, nv, AreaValues, redactPath)...)
}

// flattenValues turns nested values into "a.b.c" → scalar strings.
func flattenValues(prefix string, v interface{}) map[string]string {
	out := map[string]string{}
	var walk func(p string, x interface{})
	walk = func(p string, x interface{}) {
		switch t := x.(type) {
		case map[string]interface{}:
			if len(t) == 0 && p != "" {
				out[p] = "{}"
			}
			for k, val := range t {
				key := k
				if p != "" {
					key = p + "." + k
				}
				walk(key, val)
			}
		case []interface{}:
			b, _ := json.Marshal(t)
			out[p] = string(b)
		case nil:
			out[p] = "null"
		default:
			out[p] = fmt.Sprint(t)
		}
	}
	walk(prefix, v)
	delete(out, "")
	return out
}

// MergeHelm folds Helm revisions into a workload's rollout history: a Helm
// upgrade that caused a rollout (within helmMatchWindow) is attached to that
// row (actor "helm", values diff appended); the rest are their own rows.
func MergeHelm(wc *model.WorkloadChanges, helm []model.WorkloadChange) {
	used := make([]bool, len(wc.Changes))
	var extra []model.WorkloadChange
	for _, h := range helm {
		ht := h.Time.Time()
		match := -1
		for i, c := range wc.Changes {
			if used[i] {
				continue
			}
			gap := c.Time.Time().Sub(ht)
			if gap < 0 {
				gap = -gap
			}
			if gap <= helmMatchWindow {
				match = i
				break
			}
		}
		if match < 0 {
			h.Current = false
			extra = append(extra, h)
			continue
		}
		used[match] = true
		c := &wc.Changes[match]
		c.HelmRevision = h.HelmRevision
		c.Actor = "helm"
		if len(h.Diffs) > 0 {
			c.Diffs = append(c.Diffs, h.Diffs...)
			if c.Source != model.ChangeSourceRollback {
				c.Summary = Summarize(c.Diffs)
			}
		}
		if c.ChangeCause == "" {
			c.ChangeCause = h.ChangeCause
		}
	}
	wc.Changes = append(wc.Changes, extra...)
	sort.SliceStable(wc.Changes, func(i, j int) bool {
		return wc.Changes[i].Time.After(wc.Changes[j].Time)
	})
	finish(wc)
}
