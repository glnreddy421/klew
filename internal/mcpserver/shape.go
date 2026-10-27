package mcpserver

import (
	"sort"

	"github.com/glnreddy421/klew/internal/engine"
	"github.com/glnreddy421/klew/internal/model"
)

// Compact projections of engine state. Tool results land in the model's
// context, so they drop UI-only fields (sparklines, labels, layout) and cap
// long text.

const maxTextLen = 400

type containerBrief struct {
	Name         string `json:"name"`
	Image        string `json:"image,omitempty"`
	Ready        bool   `json:"ready"`
	Restarts     int32  `json:"restarts"`
	State        string `json:"state"`
	Reason       string `json:"reason,omitempty"`
	Message      string `json:"message,omitempty"`
	ExitCode     int32  `json:"exit_code,omitempty"`
	LastState    string `json:"last_state,omitempty"`
	LastReason   string `json:"last_reason,omitempty"`
	LastExitCode int32  `json:"last_exit_code,omitempty"`
	RequestsCPU  string `json:"requests_cpu,omitempty"`
	RequestsMem  string `json:"requests_mem,omitempty"`
	LimitsCPU    string `json:"limits_cpu,omitempty"`
	LimitsMem    string `json:"limits_mem,omitempty"`
}

type podBrief struct {
	Name       string           `json:"name"`
	Phase      string           `json:"phase"`
	Ready      bool             `json:"ready"`
	Restarts   int32            `json:"restarts"`
	Node       string           `json:"node,omitempty"`
	Owner      string           `json:"owner,omitempty"`
	CreatedAt  model.Timestamp  `json:"created_at,omitempty"`
	Containers []containerBrief `json:"containers"`
}

type signalBrief struct {
	Label    string         `json:"label"`
	Severity model.Severity `json:"severity"`
	Strength string         `json:"strength"`
	Source   string         `json:"source,omitempty"`
	Count    int            `json:"count,omitempty"`
	Evidence string         `json:"evidence,omitempty"`
	Object   string         `json:"object,omitempty"`
}

type sampleBrief struct {
	Pod     string          `json:"pod,omitempty"`
	Time    model.Timestamp `json:"time,omitempty"`
	Message string          `json:"message"`
}

type patternBrief struct {
	Template string         `json:"template"`
	Count    int            `json:"count"`
	Pct      float64        `json:"pct"`
	Trend    string         `json:"trend,omitempty"`
	TrendPct float64        `json:"trend_pct,omitempty"`
	Severity model.Severity `json:"severity"`
	Pods     []string       `json:"pods,omitempty"`
	Samples  []sampleBrief  `json:"samples,omitempty"`
}

type eventBrief struct {
	Time      model.Timestamp `json:"time"`
	FirstSeen model.Timestamp `json:"first_seen,omitempty"`
	Type      string          `json:"type"`
	Reason    string          `json:"reason"`
	Object    string          `json:"object"`
	Namespace string          `json:"namespace,omitempty"`
	Message   string          `json:"message"`
	Count     int32           `json:"count,omitempty"`
	Source    string          `json:"source,omitempty"`
	// Recorded marks events that came from Klew's local history (expired from the API).
	Recorded bool `json:"recorded,omitempty"`
}

type evidenceBrief struct {
	Time     model.Timestamp  `json:"time"`
	Source   model.SourceType `json:"source"`
	Object   string           `json:"object,omitempty"`
	Pod      string           `json:"pod,omitempty"`
	Severity model.Severity   `json:"severity"`
	Reason   string           `json:"reason,omitempty"`
	Message  string           `json:"message"`
	Count    int              `json:"count,omitempty"`
	Recorded bool             `json:"recorded,omitempty"`
}

const maxCoverageGaps = 10

// coverageBrief is event-history coverage with only the newest gaps; a week of
// laptop sleep can produce hundreds.
type coverageBrief struct {
	Since          model.Timestamp   `json:"since,omitempty"`
	RecordedEvents int               `json:"recorded_events,omitempty"`
	Gaps           []model.TimeRange `json:"gaps,omitempty"`
	GapsTotal      int               `json:"gaps_total,omitempty"`
}

func briefCoverage(c *model.EventHistoryCoverage) *coverageBrief {
	if c == nil {
		return nil
	}
	gaps := c.Gaps
	if len(gaps) > maxCoverageGaps {
		gaps = gaps[len(gaps)-maxCoverageGaps:]
	}
	return &coverageBrief{Since: c.Since, RecordedEvents: c.RecordedEvents, Gaps: gaps, GapsTotal: len(c.Gaps)}
}

// truncate caps s at maxTextLen runes.
func truncate(s string) string {
	r := []rune(s)
	if len(r) <= maxTextLen {
		return s
	}
	return string(r[:maxTextLen]) + "…"
}

func refString(r model.ObjectRef) string {
	if r.Name == "" {
		return r.Kind
	}
	if r.Kind == "" {
		return r.Name
	}
	return r.Kind + "/" + r.Name
}

func briefPod(p model.PodSummary) podBrief {
	out := podBrief{
		Name: p.Name, Phase: p.Phase, Ready: p.Ready, Restarts: p.RestartCount,
		Node: p.Node, CreatedAt: p.CreatedAt, Containers: make([]containerBrief, 0, len(p.Containers)),
	}
	if len(p.OwnerRefs) > 0 {
		out.Owner = refString(p.OwnerRefs[0])
	}
	for _, c := range p.Containers {
		out.Containers = append(out.Containers, containerBrief{
			Name: c.Name, Image: c.Image, Ready: c.Ready, Restarts: c.RestartCount,
			State: c.State, Reason: c.Reason, Message: truncate(c.Message), ExitCode: c.ExitCode,
			LastState: c.LastState, LastReason: c.LastReason, LastExitCode: c.LastExitCode,
			RequestsCPU: c.RequestsCPU, RequestsMem: c.RequestsMem, LimitsCPU: c.LimitsCPU, LimitsMem: c.LimitsMem,
		})
	}
	return out
}

// rankPods orders pods worst first: failing, then restarts, then name.
// Successfully completed Job pods are dropped.
func rankPods(pods []model.PodSummary) []model.PodSummary {
	out := make([]model.PodSummary, 0, len(pods))
	for _, p := range pods {
		if !engine.PodTerminalSuccess(p) {
			out = append(out, p)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		fi, fj := engine.PodCountsAsUnready(out[i]), engine.PodCountsAsUnready(out[j])
		if fi != fj {
			return fi
		}
		if out[i].RestartCount != out[j].RestartCount {
			return out[i].RestartCount > out[j].RestartCount
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// failingPods returns ranked pods that are unready, failing or restarting.
func failingPods(pods []model.PodSummary) []model.PodSummary {
	var out []model.PodSummary
	for _, p := range rankPods(pods) {
		if engine.PodCountsAsUnready(p) || p.RestartCount > 0 {
			out = append(out, p)
		}
	}
	return out
}

func allSignals(v model.Verdict) []model.Signal {
	out := append([]model.Signal{}, v.StrongSignals...)
	out = append(out, v.MediumSignals...)
	return append(out, v.WeakSignals...)
}

func briefSignal(s model.Signal) signalBrief {
	return signalBrief{
		Label: s.Label, Severity: s.Severity, Strength: s.Strength, Source: s.Source, Count: s.Count,
		Evidence: truncate(s.Evidence), Object: refString(s.ObjectRef),
	}
}

func briefPattern(t model.LogTemplate, samples int) patternBrief {
	out := patternBrief{
		Template: truncate(t.Template), Count: t.Count, Pct: t.Pct, Trend: t.Trend,
		TrendPct: t.TrendPct, Severity: t.Severity,
	}
	out.Pods = t.Pods
	if len(out.Pods) > 5 {
		out.Pods = out.Pods[:5]
	}
	for i, smp := range t.Samples {
		if i >= samples {
			break
		}
		out.Samples = append(out.Samples, sampleBrief{Pod: smp.Pod, Time: smp.Timestamp, Message: truncate(smp.Message)})
	}
	return out
}

func briefEvent(e model.EventRecord) eventBrief {
	t := e.LastSeen
	if t.IsZero() {
		t = e.Timestamp
	}
	return eventBrief{
		Time: t, FirstSeen: e.FirstSeen, Type: e.Type, Reason: e.Reason, Object: refString(e.InvolvedObject),
		Namespace: e.InvolvedObject.Namespace, Message: truncate(e.Message), Count: e.Count,
		Source: e.Source, Recorded: e.Recorded,
	}
}

func briefEvidence(e model.EvidenceEvent) evidenceBrief {
	obj := ""
	if e.SourceKind != "" || e.SourceName != "" {
		obj = refString(model.ObjectRef{Kind: e.SourceKind, Name: e.SourceName})
	}
	return evidenceBrief{
		Time: e.Timestamp, Source: e.SourceType, Object: obj, Pod: e.Pod, Severity: e.Severity,
		Reason: e.Reason, Message: truncate(e.Message), Count: e.Count, Recorded: e.Recorded,
	}
}

func eventTime(e model.EventRecord) model.Timestamp {
	if !e.LastSeen.IsZero() {
		return e.LastSeen
	}
	return e.Timestamp
}

// newestEventsFirst sorts events by last seen, newest first.
func newestEventsFirst(evs []model.EventRecord) {
	sort.SliceStable(evs, func(i, j int) bool {
		return eventTime(evs[j]).Before(eventTime(evs[i]))
	})
}

func capSlice[T any](in []T, n int) []T {
	if len(in) > n {
		return in[:n]
	}
	return in
}

func mapSlice[T, U any](in []T, f func(T) U) []U {
	out := make([]U, 0, len(in))
	for _, v := range in {
		out = append(out, f(v))
	}
	return out
}
