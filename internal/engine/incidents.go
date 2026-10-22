package engine

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

// HealthyLabel is the hypothesis label when nothing is failing now.
const HealthyLabel = "Workload operating normally"

// IncidentQuietPeriod is how long an incident must go without a Warning
// (with its objects healthy now) before it counts as recovered.
const IncidentQuietPeriod = 10 * time.Minute

// incidentGrouping joins Warning events this close together into one incident.
const incidentGrouping = 10 * time.Minute

// BuildIncidents groups Warning events (live and recorded) into incidents by
// time proximity, oldest first. An incident is recovered only when every
// affected object is healthy now and its last Warning is at least
// IncidentQuietPeriod old; otherwise it is ongoing.
func BuildIncidents(b model.EvidenceBundle, now time.Time) []model.Incident {
	type warn struct {
		first, last time.Time
		ev          model.EventRecord
	}
	var warns []warn
	for _, e := range b.Events {
		if !strings.EqualFold(e.Type, "Warning") {
			continue
		}
		last := e.Timestamp.Time()
		if last.IsZero() {
			continue
		}
		first := e.FirstSeen.Time()
		if first.IsZero() || first.After(last) {
			first = last
		}
		warns = append(warns, warn{first: first, last: last, ev: e})
	}
	if len(warns) == 0 {
		return nil
	}
	sort.SliceStable(warns, func(i, j int) bool { return warns[i].last.Before(warns[j].last) })

	var groups [][]warn
	for _, w := range warns {
		if n := len(groups); n > 0 {
			prev := groups[n-1][len(groups[n-1])-1].last
			if w.last.Sub(prev) <= incidentGrouping {
				groups[n-1] = append(groups[n-1], w)
				continue
			}
		}
		groups = append(groups, []warn{w})
	}

	out := make([]model.Incident, 0, len(groups))
	for _, g := range groups {
		inc := model.Incident{Recorded: true}
		start, end := g[0].first, g[0].last
		reasonCount := map[string]int{}
		seenObj := map[string]bool{}
		for _, w := range g {
			if w.first.Before(start) {
				start = w.first
			}
			if w.last.After(end) {
				end = w.last
			}
			c := int(w.ev.Count)
			if c <= 0 {
				c = 1
			}
			inc.Count += c
			reasonCount[w.ev.Reason] += c
			inc.Recorded = inc.Recorded && w.ev.Recorded
			ref := w.ev.InvolvedObject
			if key := ref.Kind + "/" + ref.Namespace + "/" + ref.Name; ref.Name != "" && !seenObj[key] {
				seenObj[key] = true
				inc.Objects = append(inc.Objects, model.ObjectRef{Kind: ref.Kind, Name: ref.Name, Namespace: ref.Namespace})
			}
		}
		inc.Start, inc.End = model.TimestampFrom(start), model.TimestampFrom(end)
		inc.LeadingReason, inc.Severity = leadingReason(reasonCount)
		inc.Recovered = now.Sub(end) >= IncidentQuietPeriod && objectsHealthyNow(b, inc.Objects)
		out = append(out, inc)
	}
	return out
}

// leadingReason picks the most severe reason, then the most frequent.
func leadingReason(counts map[string]int) (string, model.Severity) {
	best, bestSev, bestN := "", model.SeverityInfo, -1
	for reason, n := range counts {
		sev := eventRecordSeverity(reason)
		if r, br := severityRank(sev), severityRank(bestSev); r > br || (r == br && (n > bestN || (n == bestN && reason < best))) {
			best, bestSev, bestN = reason, sev, n
		}
	}
	return best, bestSev
}

// objectsHealthyNow reports whether every object is healthy in the current
// snapshot. Objects that no longer exist are judged by their owning workload
// (a replaced pod) or, failing that, cannot be unhealthy now.
func objectsHealthyNow(b model.EvidenceBundle, objs []model.ObjectRef) bool {
	for _, o := range objs {
		if !objectHealthyNow(b, o) {
			return false
		}
	}
	return true
}

func objectHealthyNow(b model.EvidenceBundle, o model.ObjectRef) bool {
	switch o.Kind {
	case "Pod":
		for _, p := range b.Pods {
			if p.Name == o.Name {
				return p.Ready && (p.Phase == "Running" || p.Phase == "Succeeded")
			}
		}
		if w, ok := owningWorkload(b, o.Name); ok {
			return workloadHealthy(w)
		}
		return true
	case "ReplicaSet":
		for _, rs := range b.ReplicaSets {
			if rs.Name == o.Name {
				return rs.Ready >= rs.Replicas
			}
		}
		if w, ok := owningWorkload(b, o.Name); ok {
			return workloadHealthy(w)
		}
		return true
	case "Deployment", "StatefulSet", "DaemonSet":
		for _, w := range b.Workloads {
			if w.Name == o.Name && (w.Kind == "" || w.Kind == o.Kind) {
				return workloadHealthy(w)
			}
		}
		return true
	case "Node":
		for _, n := range b.Nodes {
			if n.Name == o.Name {
				return n.Ready
			}
		}
		return true
	case "Service":
		for _, s := range b.Services {
			if s.Name == o.Name {
				return s.TotalEndpoints == 0 || s.ReadyEndpoints > 0
			}
		}
		return true
	default:
		return true
	}
}

// owningWorkload matches "<workload>-…" names (pods and ReplicaSets).
func owningWorkload(b model.EvidenceBundle, name string) (model.WorkloadSummary, bool) {
	var best model.WorkloadSummary
	found := false
	for _, w := range b.Workloads {
		if strings.HasPrefix(name, w.Name+"-") && len(w.Name) > len(best.Name) {
			best, found = w, true
		}
	}
	return best, found
}

func workloadHealthy(w model.WorkloadSummary) bool {
	return w.Ready >= w.Replicas && w.Available >= w.Replicas
}

// HealthyNowLabel is the headline when nothing fails now but Warnings occurred
// in the window, e.g. "Healthy now · 2 incidents in the last 6h (recovered)".
// Returns "" when there were no incidents.
func HealthyNowLabel(incidents []model.Incident, window time.Duration) string {
	if len(incidents) == 0 {
		return ""
	}
	ongoing := 0
	for _, inc := range incidents {
		if !inc.Recovered {
			ongoing++
		}
	}
	recovered := len(incidents) - ongoing
	span := "in the last " + windowLabel(window)
	switch {
	case ongoing == 0:
		return fmt.Sprintf("Healthy now · %s %s (recovered)", plural(recovered, "incident"), span)
	case recovered == 0:
		return fmt.Sprintf("Healthy now · %s %s", plural(ongoing, "ongoing incident"), span)
	default:
		return fmt.Sprintf("Healthy now · %d ongoing, %d recovered %s %s", ongoing, recovered, pluralWord(recovered, "incident"), span)
	}
}

func windowLabel(d time.Duration) string {
	if d >= time.Hour {
		return fmt.Sprintf("%dh", int(d.Round(time.Hour)/time.Hour))
	}
	m := int(d.Round(time.Minute) / time.Minute)
	if m < 1 {
		m = 1
	}
	return fmt.Sprintf("%dm", m)
}

func plural(n int, word string) string {
	return fmt.Sprintf("%d %s", n, pluralWord(n, word))
}

func pluralWord(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}
