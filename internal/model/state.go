package model

import (
	"time"

	"github.com/glnreddy421/klew/internal/investigation"
)

// SourceType identifies evidence origin.
type SourceType string

const (
	SourceLog          SourceType = "log"
	SourceK8sEvent     SourceType = "k8s_event"
	SourceObjectChange SourceType = "object_change"
	SourceMetric       SourceType = "metric"
	SourceSystem       SourceType = "system"
)

// Mode is LIVE cluster investigation or offline BUNDLE review.
type Mode string

const (
	ModeLive   Mode = "LIVE"
	ModeBundle Mode = "BUNDLE"
)

// NamespaceScope describes query scope.
type NamespaceScope struct {
	AllNamespaces bool     `json:"allNamespaces"`
	Namespaces    []string `json:"namespaces,omitempty"`
	Primary       string   `json:"primary"`
}

// EvidenceEvent is a single piece of live or snapshot evidence.
type EvidenceEvent struct {
	ID                string            `json:"id"`
	Timestamp         Timestamp         `json:"timestamp"`
	SourceType        SourceType        `json:"sourceType"`
	SourceKind        string            `json:"sourceKind,omitempty"`
	SourceName        string            `json:"sourceName,omitempty"`
	Namespace         string            `json:"namespace,omitempty"`
	Pod               string            `json:"pod,omitempty"`
	Container         string            `json:"container,omitempty"`
	Node              string            `json:"node,omitempty"`
	Severity          Severity          `json:"severity"`
	Reason            string            `json:"reason,omitempty"`
	Message           string            `json:"message"`
	Raw               string            `json:"raw,omitempty"`
	Fingerprint       string            `json:"fingerprint,omitempty"`
	Count             int               `json:"count,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	Confidence        float64           `json:"confidence"`
	RelatedObjectRefs []ObjectRef       `json:"relatedObjectRefs,omitempty"`
}

// ActiveWatch tracks a running watch/stream.
type ActiveWatch struct {
	Name      string    `json:"name"`
	Resource  string    `json:"resource"`
	Namespace string    `json:"namespace,omitempty"`
	StartedAt Timestamp `json:"startedAt"`
}

// StreamCounters tracks live collection stats.
type StreamCounters struct {
	EventsIngested int64     `json:"eventsIngested"`
	LogsIngested   int64     `json:"logsIngested"`
	ObjectChanges  int64     `json:"objectChanges"`
	MetricSamples  int64     `json:"metricSamples"`
	LastEventAt    Timestamp `json:"lastEventAt"`
}

// RecentChange captures automatic enrichment about what recently changed for a
// workload (deployment rollout, image, Helm/ArgoCD/Rollouts). Fields are only
// populated when the corresponding data is available; empty fields are hidden.
type RecentChange struct {
	RevisionFrom  string    `json:"revisionFrom,omitempty"`
	RevisionTo    string    `json:"revisionTo,omitempty"`
	DeployedAt    Timestamp `json:"deployedAt,omitempty"`
	Image         string    `json:"image,omitempty"`
	HelmRelease   string    `json:"helmRelease,omitempty"`
	HelmRevision  string    `json:"helmRevision,omitempty"`
	GitSHA        string    `json:"gitSHA,omitempty"`
	SyncState     string    `json:"syncState,omitempty"`
	RolloutStatus string    `json:"rolloutStatus,omitempty"`
}

// Hypothesis is one candidate explanation with its qualitative confidence. The
// engine keeps a ranked set so the UI can show the leading belief plus the
// alternatives it is still weighing (never a single unquestioned verdict).
type Hypothesis struct {
	Label      string  `json:"label"`
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
	Leading    bool    `json:"leading"`
}

// HypothesisTransition records the most recent leading-signal flip so the UI can
// show "was X → now Y" with the confidence delta.
type HypothesisTransition struct {
	From      string  `json:"from"`
	To        string  `json:"to"`
	ConfDelta float64 `json:"confDelta"`
}

// InvestigationState is the central mutable investigation snapshot.
type InvestigationState struct {
	CollectedAt       Timestamp         `json:"collectedAt"`
	LastUpdatedAt     Timestamp         `json:"lastUpdatedAt"`
	Mode              Mode              `json:"mode"`
	KubeContext       KubeContext       `json:"kubeContext"`
	NamespaceScope    NamespaceScope    `json:"namespaceScope"`
	Query             string            `json:"query"`
	Window            int64             `json:"window"` // milliseconds
	TailLines         int               `json:"tailLines"`
	MatchedObjects    []MatchedObject   `json:"matchedObjects"`
	Scope             *investigation.InvestigationScope `json:"scope,omitempty"`
	RecentChange      *RecentChange     `json:"recentChange,omitempty"`
	Snapshot          EvidenceBundle    `json:"snapshot"`
	WorkloadGraph     WorkloadGraph     `json:"workloadGraph"`
	Timeline          []TimelineEvent   `json:"timeline"`
	LiveEvidence      []EvidenceEvent   `json:"liveEvidence"`
	Verdict           Verdict           `json:"verdict"`
	Hypothesis        string            `json:"hypothesis"`
	HypothesisLabel   string            `json:"hypothesisLabel"`
	HypothesisReasons []string          `json:"hypothesisReasons"`
	HypothesisStatus  string            `json:"hypothesisStatus,omitempty"`
	HypothesisAlts    []Hypothesis      `json:"hypothesisAlternatives,omitempty"`
	ConfidenceTrend   string            `json:"confidenceTrend,omitempty"` // up | down | flat
	CausalChain       []string          `json:"causalChain,omitempty"`
	NextChecks        []string          `json:"nextChecks,omitempty"`
	FixActions        []string          `json:"fixActions,omitempty"`
	LastTransition    *HypothesisTransition `json:"lastTransition,omitempty"`
	Correlation       []string          `json:"correlation"`
	Permissions       []PermissionCheck `json:"permissions"`
	Warnings          []string          `json:"warnings"`
	ActiveWatches     []ActiveWatch     `json:"activeWatches"`
	ExpectedWatches   int               `json:"expectedWatches"`
	WatchNote         string            `json:"watchNote,omitempty"`
	Counters          StreamCounters    `json:"counters"`
	DroppedEvidence   int64             `json:"droppedEvidence"`
	HypothesisChanges int               `json:"hypothesisChanges"`
	Paused            bool              `json:"paused"`
	LogPatterns       *LogPatterns      `json:"logPatterns,omitempty"`
	// LogTailPods is the active log-gather allowlist; empty = not tailing logs.
	LogTailPods       []string          `json:"logTailPods,omitempty"`
	// LogTailPaused is true when log follows are stopped but the gather session
	// (pod selection) is retained for resume.
	LogTailPaused     bool              `json:"logTailPaused,omitempty"`
}

// Clone returns a shallow copy safe for TUI rendering.
func (s InvestigationState) Clone() InvestigationState {
	out := s
	out.LiveEvidence = append([]EvidenceEvent(nil), s.LiveEvidence...)
	out.Timeline = append([]TimelineEvent(nil), s.Timeline...)
	out.Warnings = append([]string(nil), s.Warnings...)
	out.Correlation = append([]string(nil), s.Correlation...)
	out.HypothesisReasons = append([]string(nil), s.HypothesisReasons...)
	out.HypothesisAlts = append([]Hypothesis(nil), s.HypothesisAlts...)
	out.CausalChain = append([]string(nil), s.CausalChain...)
	out.NextChecks = append([]string(nil), s.NextChecks...)
	out.FixActions = append([]string(nil), s.FixActions...)
	out.LogTailPods = append([]string(nil), s.LogTailPods...)
	return out
}

// Investigation adapts state for legacy view helpers.
func (s InvestigationState) Investigation() Investigation {
	return Investigation{
		Bundle:   s.Snapshot,
		Timeline: s.Timeline,
		Graph:    s.WorkloadGraph,
		Verdict:  s.Verdict,
	}
}

// ToBundle exports state for offline bundle persistence.
func (s InvestigationState) ToBundle() EvidenceBundle {
	b := s.Snapshot
	b.CollectedAt = s.CollectedAt
	b.Query = s.Query
	b.KubeContext = s.KubeContext
	b.Permissions = s.Permissions
	b.Warnings = s.Warnings
	b.MatchedObjects = s.MatchedObjects
	if s.NamespaceScope.Primary != "" {
		b.Namespace = s.NamespaceScope.Primary
	}
	return b
}

// NewInvestigationState creates an empty live state.
func NewInvestigationState(query string, mode Mode) InvestigationState {
	now := time.Now().UTC()
	return InvestigationState{
		CollectedAt:   TimestampFrom(now),
		LastUpdatedAt: TimestampFrom(now),
		Mode:          mode,
		Query:         query,
		Verdict: Verdict{
			Status:  VerdictUnknown,
			Summary: "Collecting evidence…",
		},
		LiveEvidence: make([]EvidenceEvent, 0, 256),
	}
}
