package engine

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

// SignalAggregator turns raw evidence into ranked, de-duplicated signals grouped
// by human label, scoped to a time window.
type SignalAggregator struct {
	Window time.Duration
}

// Aggregate groups evidence within the window into ranked signals.
func (a SignalAggregator) Aggregate(events []model.EvidenceEvent, now time.Time) []model.Signal {
	window := a.Window
	if window <= 0 {
		window = 15 * time.Minute
	}
	cutoff := model.TimestampFrom(now.Add(-window))

	type group struct {
		label    string
		source   string
		severity model.Severity
		count    int
		newest   time.Time
		evidence string
		conf     float64
	}
	groups := map[string]*group{}
	for _, e := range events {
		if e.SourceType == model.SourceSystem {
			continue
		}
		if !e.Timestamp.IsZero() && e.Timestamp.Before(cutoff) {
			continue
		}
		label := signalLabel(e)
		key := strings.ToLower(label)
		g := groups[key]
		if g == nil {
			g = &group{label: label, source: sourceLabel(e.SourceType), severity: e.Severity, newest: e.Timestamp.Time(), evidence: plainEvidence(e), conf: e.Confidence}
			groups[key] = g
		}
		c := e.Count
		if c <= 0 {
			c = 1
		}
		g.count += c
		if severityRank(e.Severity) > severityRank(g.severity) {
			g.severity = e.Severity
			g.source = sourceLabel(e.SourceType)
			g.evidence = plainEvidence(e)
		}
		if e.Timestamp.After(model.TimestampFrom(g.newest)) {
			g.newest = e.Timestamp.Time()
		}
		if e.Confidence > g.conf {
			g.conf = e.Confidence
		}
	}

	var signals []model.Signal
	for _, g := range groups {
		score := severityWeight(g.severity) * (1 + math.Log1p(float64(g.count)))
		if !g.newest.IsZero() {
			age := now.Sub(g.newest)
			if age < 2*time.Minute {
				score += 12
			} else if age < 5*time.Minute {
				score += 6
			}
		}
		signals = append(signals, model.Signal{
			ID:         strings.ToLower(strings.ReplaceAll(g.label, " ", "_")),
			Label:      g.label,
			Severity:   g.severity,
			Source:     g.source,
			Count:      g.count,
			Score:      score,
			Strength:   strengthFor(score),
			Evidence:   g.evidence,
			Confidence: clamp01(g.conf),
		})
	}
	sort.SliceStable(signals, func(i, j int) bool { return signals[i].Score > signals[j].Score })
	return signals
}

func signalLabel(e model.EvidenceEvent) string {
	if e.Reason != "" {
		if e.SourceType == model.SourceLog {
			return e.Reason + " logs"
		}
		return e.Reason
	}
	switch e.SourceType {
	case model.SourceLog:
		return "log signal"
	case model.SourceObjectChange:
		return e.SourceKind + " change"
	case model.SourceMetric:
		return "metric"
	default:
		return "signal"
	}
}

func sourceLabel(t model.SourceType) string {
	switch t {
	case model.SourceLog:
		return "LOG"
	case model.SourceK8sEvent:
		return "EVENT"
	case model.SourceObjectChange:
		return "OBJECT"
	case model.SourceMetric:
		return "METRIC"
	default:
		return "SYS"
	}
}

func plainEvidence(e model.EvidenceEvent) string {
	if e.Raw != "" {
		return e.Raw
	}
	return e.Message
}

func severityWeight(s model.Severity) float64 {
	switch s {
	case model.SeverityCritical:
		return 40
	case model.SeverityHigh:
		return 25
	case model.SeverityWarning:
		return 12
	default:
		return 4
	}
}

func strengthFor(score float64) string {
	switch {
	case score >= 45:
		return "strong"
	case score >= 22:
		return "medium"
	default:
		return "weak"
	}
}

func clamp01(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}
