package mcpserver

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glnreddy421/klew/v2/internal/engine"
	"github.com/glnreddy421/klew/v2/internal/model"
	"github.com/glnreddy421/klew/v2/internal/redact"
)

// Compact projections of engine state. Tool results land in the model's
// context and from there go to the model provider, so every free-text field
// passes through clean (best-effort redaction, then a length cap) and UI-only
// fields (sparklines, labels, layout) are dropped.

const maxTextLen = 400

type containerBrief struct {
	Name           string   `json:"name"`
	Image          string   `json:"image,omitempty"`
	Ready          bool     `json:"ready"`
	Restarts       int32    `json:"restarts"`
	State          string   `json:"state"`
	Reason         string   `json:"reason,omitempty"`
	Message        string   `json:"message,omitempty"`
	ExitCode       int32    `json:"exit_code,omitempty"`
	LastState      string   `json:"last_state,omitempty"`
	LastReason     string   `json:"last_reason,omitempty"`
	LastExitCode   int32    `json:"last_exit_code,omitempty"`
	RequestsCPU    string   `json:"requests_cpu,omitempty"`
	RequestsMem    string   `json:"requests_mem,omitempty"`
	LimitsCPU      string   `json:"limits_cpu,omitempty"`
	LimitsMem      string   `json:"limits_mem,omitempty"`
	Ports          []string `json:"ports,omitempty"`
	ReadinessProbe string   `json:"readiness_probe,omitempty"`
	LivenessProbe  string   `json:"liveness_probe,omitempty"`
	Mounts         []string `json:"mounts,omitempty"`
}

type podBrief struct {
	Name      string          `json:"name"`
	Phase     string          `json:"phase"`
	Ready     bool            `json:"ready"`
	Restarts  int32           `json:"restarts"`
	Node      string          `json:"node,omitempty"`
	Owner     string          `json:"owner,omitempty"`
	CreatedAt model.Timestamp `json:"created_at,omitempty"`
	// Unschedulable is the scheduler's reason when the pod cannot be placed.
	Unschedulable string           `json:"unschedulable,omitempty"`
	Containers    []containerBrief `json:"containers"`
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

// clean redacts credentials and email addresses (best effort, see package
// redact), then caps s at maxTextLen runes. Redacting first keeps a secret cut
// at the boundary from surviving.
func clean(s string) string {
	s = redact.String(s)
	r := []rune(s)
	if len(r) <= maxTextLen {
		return s
	}
	return string(r[:maxTextLen]) + "…"
}

const maxWarnings = 20

// briefWarnings cleans and de-duplicates warnings, keeping the first maxWarnings
// and saying how many were left out.
func briefWarnings(lists ...[]string) []string {
	var out []string
	seen := map[string]bool{}
	for _, list := range lists {
		for _, w := range list {
			if !seen[w] {
				seen[w] = true
				out = append(out, clean(w))
			}
		}
	}
	if extra := len(out) - maxWarnings; extra > 0 {
		out = append(out[:maxWarnings], fmt.Sprintf("…and %d more warnings.", extra))
	}
	return out
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
		Node: p.Node, CreatedAt: p.CreatedAt, Unschedulable: clean(p.Unschedulable),
		Containers: make([]containerBrief, 0, len(p.Containers)),
	}
	if len(p.OwnerRefs) > 0 {
		out.Owner = refString(p.OwnerRefs[0])
	}
	for _, c := range p.Containers {
		out.Containers = append(out.Containers, containerBrief{
			Name: c.Name, Image: c.Image, Ready: c.Ready, Restarts: c.RestartCount,
			State: c.State, Reason: c.Reason, Message: clean(c.Message), ExitCode: c.ExitCode,
			LastState: c.LastState, LastReason: c.LastReason, LastExitCode: c.LastExitCode,
			RequestsCPU: c.RequestsCPU, RequestsMem: c.RequestsMem, LimitsCPU: c.LimitsCPU, LimitsMem: c.LimitsMem,
			Ports: c.Ports, ReadinessProbe: clean(c.ReadinessProbe), LivenessProbe: clean(c.LivenessProbe), Mounts: c.Mounts,
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

// shaper projects engine state for tool results. With noLogSamples it leaves
// out raw container log text: pattern samples, log evidence messages,
// termination messages, log templates that merged no differing lines, and log
// excerpts in signals, the timeline and the causal chain. Generalised templates,
// counts, trends and every non-log signal are kept.
type shaper struct {
	noLogSamples bool
}

// pod drops terminated containers' messages when log samples are off: they are
// written by the application, and with FallbackToLogsOnError are the log tail.
// Waiting messages come from the kubelet and stay.
func (sh shaper) pod(p model.PodSummary) podBrief {
	out := briefPod(p)
	if sh.noLogSamples {
		for i, c := range p.Containers {
			if c.State == "terminated" {
				out.Containers[i].Message = ""
			}
		}
	}
	return out
}

func (sh shaper) signal(s model.Signal) signalBrief {
	out := signalBrief{
		Label: s.Label, Severity: s.Severity, Strength: s.Strength, Source: s.Source, Count: s.Count,
		Evidence: clean(s.Evidence), Object: refString(s.ObjectRef),
	}
	if sh.noLogSamples && s.Source == "LOG" {
		out.Evidence = ""
	}
	return out
}

// withheldTemplate replaces a log template that is not a real generalisation.
const withheldTemplate = "‹log text withheld›"

// isGeneralised reports whether Drain merged differing lines into t. Only then
// has anything been abstracted: a line that never varied, or was seen once,
// is its own template, with at most its numbers and addresses masked.
func isGeneralised(t model.LogTemplate) bool {
	return t.Count >= 2 && strings.Contains(t.Template, "<*>")
}

func (sh shaper) pattern(t model.LogTemplate, samples int) patternBrief {
	if !sh.noLogSamples {
		return briefPattern(t, samples)
	}
	out := briefPattern(t, 0)
	if !isGeneralised(t) {
		out.Template = withheldTemplate
	}
	return out
}

func (sh shaper) evidence(e model.EvidenceEvent) evidenceBrief {
	out := briefEvidence(e)
	if sh.noLogSamples && e.SourceType == model.SourceLog {
		out.Message = ""
	}
	return out
}

type timelineBrief struct {
	Time     model.Timestamp `json:"time"`
	Type     string          `json:"type"`
	Severity model.Severity  `json:"severity"`
	Object   string          `json:"object,omitempty"`
	Reason   string          `json:"reason,omitempty"`
	Message  string          `json:"message,omitempty"`
}

func (sh shaper) timeline(e model.TimelineEvent) timelineBrief {
	out := timelineBrief{
		Time: e.Timestamp, Type: e.Type, Severity: e.Severity, Reason: e.Reason, Message: clean(e.Message),
	}
	if e.SourceKind != "" || e.SourceName != "" {
		out.Object = refString(model.ObjectRef{Kind: e.SourceKind, Name: e.SourceName})
	}
	if sh.noLogSamples && e.Type == string(model.SourceLog) {
		out.Message = ""
	}
	return out
}

type evidenceCardBrief struct {
	Confidence    float64        `json:"confidence"`
	RootEvent     patternBrief   `json:"root_event"`
	TriggeredLogs []patternBrief `json:"triggered_logs"`
}

func (sh shaper) card(c model.EvidenceCard) evidenceCardBrief {
	return evidenceCardBrief{
		Confidence: c.Confidence,
		RootEvent:  briefPattern(c.RootEvent, 1), // an infrastructure event, not a log line
		TriggeredLogs: mapSlice(c.TriggeredLogs, func(t model.LogTemplate) patternBrief {
			return sh.pattern(t, 1)
		}),
	}
}

// causalChain drops steps labelled with the opening words of a log line when
// log samples are off; steps named after a classified reason stay.
func (sh shaper) causalChain(chain []string, tl []model.TimelineEvent) []string {
	if !sh.noLogSamples {
		return redact.Strings(chain)
	}
	var logText []string
	for _, e := range tl {
		if e.Type == string(model.SourceLog) && e.Reason == "" && e.Message != "" {
			// Chain labels are whitespace-normalised (engine firstWords).
			logText = append(logText, strings.Join(strings.Fields(e.Message), " "))
		}
	}
	var out []string
	for _, step := range chain {
		label, _, _ := strings.Cut(step, " +")
		fromLog := false
		for _, m := range logText {
			if strings.HasPrefix(m, label) {
				fromLog = true
				break
			}
		}
		if !fromLog {
			out = append(out, redact.String(step))
		}
	}
	return out
}

func briefPattern(t model.LogTemplate, samples int) patternBrief {
	out := patternBrief{
		Template: clean(t.Template), Count: t.Count, Pct: t.Pct, Trend: t.Trend,
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
		out.Samples = append(out.Samples, sampleBrief{Pod: smp.Pod, Time: smp.Timestamp, Message: clean(smp.Message)})
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
		Namespace: e.InvolvedObject.Namespace, Message: clean(e.Message), Count: e.Count,
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
		Reason: e.Reason, Message: clean(e.Message), Count: e.Count, Recorded: e.Recorded,
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
