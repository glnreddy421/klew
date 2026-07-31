package engine

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/glnreddy421/klew/internal/logpatterns"
	"github.com/glnreddy421/klew/internal/model"
)

const (
	maxTimeline            = 400
	logPatternsMinInterval = 2 * time.Second // avoid rebuilding Drain3 miners every event
)

// StateStore (a.k.a. Reducer) is the InvestigationStateStore: it consumes
// evidence, maintains a bounded ring buffer, and recomputes signals, correlation,
// verdict, and hypothesis on every update.
type StateStore struct {
	mu             sync.RWMutex
	state          *model.InvestigationState
	ring           *RingBuffer
	corr           CorrelationEngine
	seq            int64
	prevLeading    string
	prevConfidence float64
	hasPrev        bool
	lastPatternsAt time.Time
}

// Reducer is retained as an alias for the InvestigationStateStore.
type Reducer = StateStore

// NewStore creates a state store around an initial state.
func NewStore(state *model.InvestigationState) *StateStore {
	bufCap := 2000
	return &StateStore{state: state, ring: NewRingBuffer(bufCap)}
}

// NewReducer is kept for backwards compatibility.
func NewReducer(state *model.InvestigationState) *Reducer { return NewStore(state) }

func (r *StateStore) State() model.InvestigationState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := r.state.Clone()
	out.LiveEvidence = r.ring.Snapshot()
	out.DroppedEvidence = r.ring.Dropped()
	return out
}

func (r *StateStore) ApplyEvent(e model.EvidenceEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state.Paused {
		return
	}
	r.seq++
	if e.ID == "" {
		e.ID = fmt.Sprintf("ev-%d", r.seq)
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = model.TimestampFrom(time.Now().UTC())
	}
	if e.Fingerprint == "" {
		e.Fingerprint = Fingerprint(e)
	}

	r.ring.Add(e)

	r.state.Counters.EventsIngested++
	r.state.Counters.LastEventAt = e.Timestamp
	switch e.SourceType {
	case model.SourceLog:
		r.state.Counters.LogsIngested++
	case model.SourceObjectChange:
		r.state.Counters.ObjectChanges++
	case model.SourceMetric:
		r.state.Counters.MetricSamples++
	}

	te := evidenceToTimeline(e)
	r.state.Timeline = append(r.state.Timeline, te)
	sort.SliceStable(r.state.Timeline, func(i, j int) bool {
		return r.state.Timeline[i].Timestamp.Before(r.state.Timeline[j].Timestamp)
	})
	if len(r.state.Timeline) > maxTimeline {
		r.state.Timeline = r.state.Timeline[len(r.state.Timeline)-maxTimeline:]
	}

	r.recompute()
	r.state.LastUpdatedAt = model.TimestampFrom(time.Now().UTC())
}

// ApplySnapshot refreshes structural snapshot data and recomputes.
func (r *StateStore) ApplySnapshot(bundle model.EvidenceBundle, graph model.WorkloadGraph, timeline []model.TimelineEvent, _ model.Verdict) {
	r.mu.Lock()
	defer r.mu.Unlock()
	wasActive := IncidentActive(r.state.Snapshot)
	r.state.Snapshot = bundle
	if len(graph.Nodes) > 0 {
		r.state.WorkloadGraph = graph
	}
	r.recompute()
	if wasActive && WorkloadNominal(bundle) && !IncidentActive(bundle) {
		r.ring.Add(model.EvidenceEvent{
			Timestamp:  model.TimestampFrom(time.Now().UTC()),
			SourceType: model.SourceSystem,
			Severity:   model.SeverityInfo,
			Reason:     "Recovered",
			Message:    "Workload pods are ready — incident cleared",
			Confidence: 1,
		})
		r.state.Timeline = append(r.state.Timeline, model.TimelineEvent{
			Timestamp: model.TimestampFrom(time.Now().UTC()),
			Type:      string(model.SourceSystem),
			Severity:  model.SeverityInfo,
			Reason:    "Recovered",
			Message:   "Workload pods are ready — incident cleared",
		})
	}
	r.state.LastUpdatedAt = model.TimestampFrom(time.Now().UTC())
}

func (r *StateStore) SetPaused(p bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state.Paused = p
}

func (r *StateStore) SetWatches(w []model.ActiveWatch) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.state.ActiveWatches = w
}

func (r *StateStore) recompute() {
	now := time.Now().UTC()
	events := r.ring.Snapshot()
	agg := SignalAggregator{Window: model.DurationFromMS(r.state.Window)}
	evSignals := agg.Aggregate(events, now)
	corr := r.corr.Correlate(r.state.Snapshot, evSignals, events)

	if len(r.state.WorkloadGraph.Nodes) == 0 {
		r.state.WorkloadGraph = BuildGraph(r.state.Snapshot)
	}

	v := r.state.Verdict
	v.LeadingSignal = corr.LeadingSignal
	v.LikelyTrigger = corr.HypothesisLabel
	v.Confidence = corr.Confidence
	v.Status = statusFromSignals(corr.Signals, r.state.Snapshot)
	v.StrongSignals, v.MediumSignals, v.WeakSignals = bucketSignals(corr.Signals)
	v.AffectedPods = podNames(r.state.Snapshot.Pods)
	v.AffectedServices = serviceNames(r.state.Snapshot.Services)
	v.MissingDataWarnings = append([]string(nil), r.state.Warnings...)
	v.Summary = corr.HypothesisLabel
	r.state.Verdict = v
	r.state.Hypothesis = corr.HypothesisLabel
	r.state.HypothesisLabel = corr.HypothesisLabel
	r.state.HypothesisReasons = corr.Reasons
	r.state.Correlation = corr.Bullets
	r.state.HypothesisAlts = corr.Alternatives
	r.state.NextChecks = corr.NextChecks
	r.state.FixActions = corr.FixActions
	r.state.CausalChain = buildCausalChain(r.state.Timeline)
	r.state.HypothesisStatus = hypothesisStatus(corr.LeadingSignal, corr.Confidence, v.Status)
	r.state.ConfidenceTrend = confidenceTrend(corr.Confidence, r.prevConfidence, r.hasPrev)

	// Extract allocates two Drain3 miners + TF–IDF; throttle to bound alloc churn.
	// Merge snapshot K8s events so Infrastructure Patterns are not starved when
	// the live ring is dominated by container log lines.
	if r.lastPatternsAt.IsZero() || now.Sub(r.lastPatternsAt) >= logPatternsMinInterval {
		patternEvents := logpatterns.MergeSnapshotEvents(events, r.state.Snapshot.Events)
		lp := logpatterns.Extract(patternEvents, logpatterns.Options{})
		r.state.LogPatterns = &lp
		r.lastPatternsAt = now
	}

	if r.prevLeading != "" && corr.LeadingSignal != "" && corr.LeadingSignal != r.prevLeading {
		r.state.HypothesisChanges++
		r.state.LastTransition = &model.HypothesisTransition{
			From: r.prevLeading, To: corr.LeadingSignal, ConfDelta: corr.Confidence - r.prevConfidence,
		}
		r.ring.Add(model.EvidenceEvent{
			Timestamp:  model.TimestampFrom(time.Now()),
			SourceType: model.SourceSystem,
			Severity:   model.SeverityWarning,
			Reason:     "HypothesisChanged",
			Message: fmt.Sprintf("Hypothesis changed: %s → %s  (confidence %.0f%% → %.0f%%)",
				r.prevLeading, corr.LeadingSignal, r.prevConfidence*100, corr.Confidence*100),
			Confidence: 1,
		})
	}
	if corr.LeadingSignal != "" {
		r.prevLeading = corr.LeadingSignal
	}
	r.prevConfidence = corr.Confidence
	r.hasPrev = true
}

// hypothesisStatus expresses where the investigation is in its lifecycle rather
// than always reporting "Investigating".
func hypothesisStatus(leading string, conf float64, status model.VerdictStatus) string {
	if leading == "" {
		if status == model.VerdictHealthy {
			return "Nominal"
		}
		return "Collecting"
	}
	switch {
	case conf >= 0.8:
		return "Confirmed"
	case conf >= 0.55:
		return "Likely"
	default:
		return "Investigating"
	}
}

func confidenceTrend(cur, prev float64, hasPrev bool) string {
	if !hasPrev {
		return "flat"
	}
	switch {
	case cur > prev+0.02:
		return "up"
	case cur < prev-0.02:
		return "down"
	default:
		return "flat"
	}
}

func statusFromSignals(signals []model.Signal, b model.EvidenceBundle) model.VerdictStatus {
	if WorkloadNominal(b) && !IncidentActive(b) {
		return model.VerdictHealthy
	}
	worst := model.SeverityInfo
	for _, s := range signals {
		if severityRank(s.Severity) > severityRank(worst) {
			worst = s.Severity
		}
	}
	switch worst {
	case model.SeverityCritical:
		return model.VerdictCritical
	case model.SeverityHigh:
		return model.VerdictWarning
	case model.SeverityWarning:
		return model.VerdictWarning
	}
	if len(b.Pods) == 0 {
		return model.VerdictUnknown
	}
	for _, p := range b.Pods {
		if !p.Ready {
			return model.VerdictWarning
		}
	}
	return model.VerdictHealthy
}

func bucketSignals(signals []model.Signal) (strong, medium, weak []model.Signal) {
	for _, s := range signals {
		switch {
		case s.Score >= 45:
			s.Strength = "strong"
			strong = append(strong, s)
		case s.Score >= 22:
			s.Strength = "medium"
			medium = append(medium, s)
		default:
			s.Strength = "weak"
			weak = append(weak, s)
		}
	}
	return
}

func evidenceToTimeline(e model.EvidenceEvent) model.TimelineEvent {
	return model.TimelineEvent{
		Timestamp:  e.Timestamp,
		Type:       string(e.SourceType),
		Severity:   e.Severity,
		SourceKind: e.SourceKind,
		SourceName: e.SourceName,
		Namespace:  e.Namespace,
		Message:    e.Message,
		Reason:     e.Reason,
		InvolvedObject: model.ObjectRef{
			Kind: e.SourceKind, Name: e.SourceName, Namespace: e.Namespace,
		},
		Confidence:   e.Confidence,
		EvidenceRefs: []string{e.ID},
	}
}

func podNames(pods []model.PodSummary) []string {
	names := make([]string, 0, len(pods))
	for _, p := range pods {
		names = append(names, p.Namespace+"/"+p.Name)
	}
	return names
}

func serviceNames(svcs []model.ServiceSummary) []string {
	names := make([]string, 0, len(svcs))
	for _, s := range svcs {
		names = append(names, s.Namespace+"/"+s.Name)
	}
	return names
}

// ClassifyLogSeverity heuristically scores log lines.
func ClassifyLogSeverity(line string) model.Severity {
	l := strings.ToLower(line)
	switch {
	case strings.Contains(l, "oom"), strings.Contains(l, "fatal"), strings.Contains(l, "panic"):
		return model.SeverityCritical
	case strings.Contains(l, "error"), strings.Contains(l, "failed"), strings.Contains(l, "refused"):
		return model.SeverityHigh
	case strings.Contains(l, "warn"), strings.Contains(l, "timeout"):
		return model.SeverityWarning
	default:
		return model.SeverityInfo
	}
}
