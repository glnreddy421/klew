package cli

import (
	"sort"
	"strings"
	"time"

	"github.com/glnreddy421/klew/internal/engine"
	"github.com/glnreddy421/klew/internal/model"
)

// Report is what klew investigate prints: the desktop app's Overview,
// Failures, Patterns and Evidence views, without the graph. The text and JSON
// output are both rendered from it.
type Report struct {
	Context         string              `json:"context"`
	Namespace       string              `json:"namespace"`
	Workload        string              `json:"workload,omitempty"`
	CollectedAt     model.Timestamp     `json:"collectedAt"`
	ObservedSeconds int                 `json:"observedSeconds"`
	Status          model.VerdictStatus `json:"status"`
	Confidence      float64             `json:"confidence"`
	ConfidenceLabel string              `json:"confidenceLabel"`
	LeadingSignal   string              `json:"leadingSignal,omitempty"`
	Summary         string              `json:"summary,omitempty"`
	LikelyTrigger   string              `json:"likelyTrigger,omitempty"`
	Headline        string              `json:"headline,omitempty"`
	Pods            PodCounts           `json:"pods"`
	EndpointsReady  int                 `json:"endpointsReady"`
	EndpointsTotal  int                 `json:"endpointsTotal"`

	Overview Overview `json:"overview"`
	Failures Failures `json:"failures"`
	Patterns Patterns `json:"patterns"`
	Evidence Evidence `json:"evidence"`
	Warnings []string `json:"warnings,omitempty"`
}

type PodCounts struct {
	Ready    int `json:"ready"`
	Unready  int `json:"unready"`
	Affected int `json:"affected"`
	Restarts int `json:"restarts"`
}

type Overview struct {
	Problems          []model.Problem    `json:"problems,omitempty"`
	ProblemsTotal     int                `json:"problemsTotal"`
	CausalChain       []string           `json:"causalChain,omitempty"`
	Hypothesis        string             `json:"hypothesis,omitempty"`
	HypothesisReasons []string           `json:"hypothesisReasons,omitempty"`
	Alternatives      []model.Hypothesis `json:"alternatives,omitempty"`
	// Incidents are newest last; repeats of one reason on the same objects are merged.
	Incidents      []model.Incident      `json:"incidents,omitempty"`
	IncidentsTotal int                   `json:"incidentsTotal"`
	PinnedChange   *model.PinnedChange   `json:"pinnedChange,omitempty"`
	RecentChange   *model.RecentChange   `json:"recentChange,omitempty"`
	Timeline       []model.TimelineEvent `json:"timeline,omitempty"`
	// Timeline holds warning and worse entries, newest first.
	TimelineTotal int                         `json:"timelineTotal"`
	EventHistory  *model.EventHistoryCoverage `json:"eventHistory,omitempty"`
}

type Failures struct {
	PodsTotal    int                              `json:"podsTotal"`
	FailingTotal int                              `json:"failingTotal"`
	Pods         []FailingPod                     `json:"pods,omitempty"`
	Admission    *model.AdmissionPerimeterSummary `json:"admission,omitempty"`
}

type FailingPod struct {
	Name       string                  `json:"name"`
	Node       string                  `json:"node,omitempty"`
	Phase      string                  `json:"phase"`
	Ready      bool                    `json:"ready"`
	Restarts   int32                   `json:"restarts"`
	Reason     string                  `json:"reason,omitempty"`
	Pending    string                  `json:"unschedulable,omitempty"`
	Containers []model.ContainerStatus `json:"containers,omitempty"`
	Events     []Observation           `json:"events,omitempty"`
}

type Patterns struct {
	Logs        []Pattern `json:"logs,omitempty"`
	LogsTotal   int       `json:"logsTotal"`
	Events      []Pattern `json:"events,omitempty"`
	EventsTotal int       `json:"eventsTotal"`
}

type Pattern struct {
	Template string         `json:"template"`
	Count    int            `json:"count"`
	Pct      float64        `json:"pct"`
	Trend    string         `json:"trend,omitempty"`
	Severity model.Severity `json:"severity,omitempty"`
	Pods     []string       `json:"pods,omitempty"`
	Sample   string         `json:"sample,omitempty"`
}

type Evidence struct {
	Correlated        []Correlation           `json:"correlated,omitempty"`
	Signals           []model.Signal          `json:"signals,omitempty"`
	SignalsTotal      int                     `json:"signalsTotal"`
	Observations      []Observation           `json:"observations,omitempty"`
	ObservationsTotal int                     `json:"observationsTotal"`
	Gaps              []string                `json:"gaps,omitempty"`
	NextChecks        []string                `json:"nextChecks,omitempty"`
	SuggestedActions  []string                `json:"suggestedActions,omitempty"`
	NotCollected      []model.CollectorStatus `json:"notCollected,omitempty"`
}

// Correlation is an evidence board card: an infrastructure event pattern and
// the log patterns whose volume moved with it.
type Correlation struct {
	Confidence float64   `json:"confidence"`
	Event      Pattern   `json:"event"`
	Logs       []Pattern `json:"logs"`
}

type Observation struct {
	Time     model.Timestamp `json:"time"`
	Kind     string          `json:"kind"`
	Object   string          `json:"object,omitempty"`
	Reason   string          `json:"reason,omitempty"`
	Message  string          `json:"message"`
	Warning  bool            `json:"warning,omitempty"`
	Recorded bool            `json:"recorded,omitempty"`
}

// buildReport projects st. limit caps every list; 0 keeps everything.
func buildReport(st model.InvestigationState, observe time.Duration, limit int) Report {
	sum := engine.BuildIncidentSummary(st)
	v := st.Verdict
	r := Report{
		Context: sum.Context, Namespace: sum.Namespace, Workload: st.Query,
		CollectedAt: st.CollectedAt, ObservedSeconds: int(observe.Seconds()),
		Status: v.Status, Confidence: v.Confidence, ConfidenceLabel: engine.ConfidenceLabel(v.Confidence),
		LeadingSignal: v.LeadingSignal, Summary: v.Summary, LikelyTrigger: v.LikelyTrigger, Headline: st.Headline,
		Pods:           PodCounts{Ready: sum.ReadyPods, Unready: sum.UnreadyPods, Affected: sum.AffectedPods, Restarts: sum.Restarts},
		EndpointsReady: sum.EndpointsReady, EndpointsTotal: sum.EndpointsTotal,
	}

	r.Overview = Overview{
		Problems: capped(st.Problems, limit), ProblemsTotal: len(st.Problems),
		CausalChain: st.CausalChain, Hypothesis: st.HypothesisLabel, HypothesisReasons: st.HypothesisReasons,
		Alternatives: st.HypothesisAlts,
		PinnedChange: st.PinnedChange, RecentChange: st.RecentChange,
		EventHistory: st.Snapshot.EventHistory,
	}
	incidents := mergeIncidents(st.Incidents)
	r.Overview.Incidents, r.Overview.IncidentsTotal = latest(incidents, limit), len(incidents)
	// Info entries are mostly log lines, which Patterns already summarises.
	var timeline []model.TimelineEvent
	seen := map[string]bool{}
	for _, t := range st.Timeline {
		key := string(t.Timestamp) + "|" + t.SourceKind + "/" + t.SourceName + "|" + t.Reason + "|" + t.Message
		if t.Severity != model.SeverityInfo && t.Severity != "" && !seen[key] {
			seen[key] = true
			timeline = append(timeline, t)
		}
	}
	sort.SliceStable(timeline, func(i, j int) bool { return timeline[i].Timestamp > timeline[j].Timestamp })
	r.Overview.Timeline, r.Overview.TimelineTotal = capped(timeline, limit), len(timeline)

	pods := rankPods(st.Snapshot.Pods)
	var failing []FailingPod
	for _, p := range pods {
		if engine.PodCountsAsUnready(p) || p.RestartCount > 0 {
			failing = append(failing, failingPod(st, p, limit))
		}
	}
	r.Failures = Failures{PodsTotal: len(pods), FailingTotal: len(failing), Pods: capped(failing, limit)}
	if ap := st.AdmissionPerimeter; ap != nil && ap.MissingWorkload {
		r.Failures.Admission = ap
	}

	if lp := st.LogPatterns; lp != nil {
		r.Patterns = Patterns{
			Logs: mapped(capped(lp.Templates, limit), pattern), LogsTotal: len(lp.Templates),
			Events: mapped(capped(lp.EventTemplates, limit), pattern), EventsTotal: len(lp.EventTemplates),
		}
		if b := lp.EvidenceBoard; b != nil {
			for _, c := range capped(b.Cards, limit) {
				r.Evidence.Correlated = append(r.Evidence.Correlated, Correlation{
					Confidence: c.Confidence, Event: pattern(c.RootEvent), Logs: mapped(c.TriggeredLogs, pattern),
				})
			}
		}
	}

	signals := append(append(append([]model.Signal{}, v.StrongSignals...), v.MediumSignals...), v.WeakSignals...)
	r.Evidence.Signals, r.Evidence.SignalsTotal = capped(signals, limit), len(signals)
	var obs []Observation
	for _, e := range st.LiveEvidence {
		if e.SourceType == model.SourceLog || e.SourceType == model.SourceSystem {
			continue
		}
		obs = append(obs, evidenceObservation(e))
	}
	sort.SliceStable(obs, func(i, j int) bool { return obs[i].Time > obs[j].Time })
	r.Evidence.Observations, r.Evidence.ObservationsTotal = capped(obs, limit), len(obs)
	r.Evidence.Gaps = dedupe(v.MissingDataWarnings)
	gap := map[string]bool{}
	for _, g := range r.Evidence.Gaps {
		gap[g] = true
	}
	for _, w := range dedupe(st.Warnings) {
		if !gap[w] {
			r.Warnings = append(r.Warnings, w)
		}
	}
	r.Evidence.NextChecks = st.NextChecks
	if len(r.Evidence.NextChecks) == 0 {
		r.Evidence.NextChecks = v.RecommendedNextChecks
	}
	r.Evidence.SuggestedActions = st.FixActions
	for _, c := range st.Snapshot.Coverage {
		if c.Status != "collected" && c.Status != "not-installed" {
			r.Evidence.NotCollected = append(r.Evidence.NotCollected, c)
		}
	}
	return r
}

// rankPods drops pods that finished successfully and puts unready pods first,
// then the most restarted, as the desktop app's Failures view does.
func rankPods(pods []model.PodSummary) []model.PodSummary {
	out := make([]model.PodSummary, 0, len(pods))
	for _, p := range pods {
		if !engine.PodTerminalSuccess(p) {
			out = append(out, p)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		ui, uj := engine.PodCountsAsUnready(out[i]), engine.PodCountsAsUnready(out[j])
		if ui != uj {
			return ui
		}
		if out[i].RestartCount != out[j].RestartCount {
			return out[i].RestartCount > out[j].RestartCount
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func failingPod(st model.InvestigationState, p model.PodSummary, limit int) FailingPod {
	fp := FailingPod{
		Name: p.Name, Node: p.Node, Phase: p.Phase, Ready: p.Ready, Restarts: p.RestartCount,
		Pending: p.Unschedulable, Containers: p.Containers,
	}
	if c := worstContainer(p); c != nil {
		fp.Reason = firstNonEmpty(c.Reason, c.LastReason)
	}
	var events []Observation
	for _, e := range st.Snapshot.Events {
		if n := e.InvolvedObject.Name; n == p.Name || strings.HasPrefix(n, p.Name+"-") {
			events = append(events, Observation{
				Time: firstNonEmpty(e.LastSeen, e.Timestamp), Kind: "Event", Reason: e.Reason,
				Message: e.Message, Warning: e.Type == "Warning", Recorded: e.Recorded,
			})
		}
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].Time > events[j].Time })
	fp.Events = events
	if limit > 0 {
		fp.Events = capped(events, podEventLimit)
	}
	return fp
}

// podEventLimit caps each failing pod's events; -all lifts it.
const podEventLimit = 5

// worstContainer is the container that best explains the pod's state: a
// waiting or terminated one with a reason, else the most restarted.
func worstContainer(p model.PodSummary) *model.ContainerStatus {
	var worst *model.ContainerStatus
	for i := range p.Containers {
		c := &p.Containers[i]
		if c.Init && c.State == "terminated" && c.ExitCode == 0 {
			continue
		}
		switch {
		case worst == nil:
			worst = c
		case !c.Ready && c.Reason != "" && (worst.Ready || worst.Reason == ""):
			worst = c
		case c.RestartCount > worst.RestartCount && (c.Reason != "" || worst.Reason == ""):
			worst = c
		}
	}
	return worst
}

func evidenceObservation(e model.EvidenceEvent) Observation {
	kind := map[model.SourceType]string{
		model.SourceK8sEvent: "Event", model.SourceObjectChange: "Change", model.SourceMetric: "Metric",
	}[e.SourceType]
	o := Observation{
		Time: e.Timestamp, Kind: kind, Reason: e.Reason, Message: e.Message, Recorded: e.Recorded,
		Warning: e.Severity == model.SeverityWarning || e.Severity == model.SeverityHigh || e.Severity == model.SeverityCritical,
	}
	if e.SourceName != "" {
		o.Object = strings.TrimPrefix(e.SourceKind+"/"+e.SourceName, "/")
	}
	return o
}

func pattern(t model.LogTemplate) Pattern {
	p := Pattern{Template: t.Template, Count: t.Count, Pct: t.Pct, Trend: t.Trend, Severity: t.Severity, Pods: t.Pods}
	if len(t.Samples) > 0 && t.Samples[0].Message != t.Template {
		p.Sample = t.Samples[0].Message
	}
	return p
}

func capped[T any](s []T, limit int) []T {
	if limit > 0 && len(s) > limit {
		return s[:limit]
	}
	return s
}

func mapped[T, U any](s []T, f func(T) U) []U {
	if len(s) == 0 {
		return nil
	}
	out := make([]U, len(s))
	for i, v := range s {
		out[i] = f(v)
	}
	return out
}

func dedupe(s []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range s {
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func firstNonEmpty[T ~string](vs ...T) T {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}

// mergeIncidents joins consecutive incidents with the same leading reason on
// the same objects, such as a rejected pod retried every 15 minutes, which the
// engine's time grouping keeps apart.
func mergeIncidents(in []model.Incident) []model.Incident {
	key := func(i model.Incident) string {
		k := i.LeadingReason
		for _, o := range i.Objects {
			k += "|" + o.Kind + "/" + o.Name
		}
		return k
	}
	var out []model.Incident
	for _, i := range in {
		if n := len(out); n > 0 && key(out[n-1]) == key(i) {
			m := &out[n-1]
			m.End, m.Count, m.Recovered = i.End, m.Count+i.Count, i.Recovered
			m.Recorded = m.Recorded && i.Recorded
			continue
		}
		out = append(out, i)
	}
	return out
}

// latest keeps the last n items; n <= 0 keeps all.
func latest[T any](s []T, n int) []T {
	if n > 0 && len(s) > n {
		return s[len(s)-n:]
	}
	return s
}
