package model

// Change sources for a workload revision.
const (
	ChangeSourceRollout  = "rollout"
	ChangeSourceRollback = "rollback"
	ChangeSourceRestart  = "restart"
	ChangeSourceHelm     = "helm"
)

// Field diff kinds.
const (
	DiffAdded   = "added"
	DiffRemoved = "removed"
	DiffChanged = "changed"
)

// FieldDiff is one changed field between two revisions. Secret-like values are
// redacted; secretKeyRef/configMapKeyRef values appear as references.
type FieldDiff struct {
	Path   string `json:"path"`
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
	Kind   string `json:"kind"`
	// Area groups fields for summaries: image, env, resources, probes, …
	Area string `json:"area,omitempty"`
}

// WorkloadChange is one revision of a workload and what changed from the one before.
type WorkloadChange struct {
	Revision int64     `json:"revision"`
	Time     Timestamp `json:"time"`
	Source   string    `json:"source"`
	// Actor is set only when it can be matched (managedFields, Helm, GitOps tooling).
	Actor       string      `json:"actor,omitempty"`
	ChangeCause string      `json:"changeCause,omitempty"`
	Summary     string      `json:"summary"`
	Diffs       []FieldDiff `json:"diffs,omitempty"`
	// RollbackTo is the original revision whose template a rollback restored.
	RollbackTo   int64  `json:"rollbackTo,omitempty"`
	HelmRevision *int   `json:"helmRevision,omitempty"`
	Current      bool   `json:"current,omitempty"`
	ObjectName   string `json:"objectName,omitempty"` // ReplicaSet / ControllerRevision name
}

// Revision history access states.
const (
	ChangesAccessAllowed = "allowed"
	ChangesAccessDenied  = "denied"
	ChangesAccessError   = "error"
)

// WorkloadChanges is the revision history of one workload, newest first.
type WorkloadChanges struct {
	Kind            string           `json:"kind"`
	Namespace       string           `json:"namespace"`
	Name            string           `json:"name"`
	UID             string           `json:"uid,omitempty"`
	Generation      int64            `json:"generation,omitempty"`
	Changes         []WorkloadChange `json:"changes"`
	CurrentRevision int64            `json:"currentRevision,omitempty"`
	LastChangedAt   Timestamp        `json:"lastChangedAt,omitempty"`
	// HistoryLimit is the workload's revisionHistoryLimit (0 when unknown).
	HistoryLimit  int32  `json:"historyLimit,omitempty"`
	HelmRelease   string `json:"helmRelease,omitempty"`
	AccessState   string `json:"accessState"`
	AccessMessage string `json:"accessMessage,omitempty"`
	// HelmNote explains why Helm history is missing (e.g. Secrets denied).
	HelmNote string `json:"helmNote,omitempty"`
}

// PodRevision tells whether a Pod runs its workload's current revision.
type PodRevision struct {
	OwnerKind       string `json:"ownerKind,omitempty"`
	OwnerName       string `json:"ownerName,omitempty"`
	Revision        int64  `json:"revision,omitempty"`
	CurrentRevision int64  `json:"currentRevision,omitempty"`
	Behind          bool   `json:"behind,omitempty"`
	// BehindSummary is what the current revision changed, e.g. "image".
	BehindSummary string    `json:"behindSummary,omitempty"`
	CurrentSince  Timestamp `json:"currentSince,omitempty"`
	AccessState   string    `json:"accessState"`
	AccessMessage string    `json:"accessMessage,omitempty"`
}

// WorkloadChangeMarker is the latest change of an in-scope workload, used by
// investigations (snapshot refresh; never includes Helm history).
type WorkloadChangeMarker struct {
	Kind      string    `json:"kind"`
	Namespace string    `json:"namespace"`
	Name      string    `json:"name"`
	Revision  int64     `json:"revision"`
	Time      Timestamp `json:"time"`
	Source    string    `json:"source"`
	Summary   string    `json:"summary"`
	// Areas are the changed areas, e.g. ["image", "env"].
	Areas []string `json:"areas,omitempty"`
	// ObjectName is the ReplicaSet / ControllerRevision of this revision.
	ObjectName string `json:"objectName,omitempty"`
}

// PinnedChange is the workload change shown at the top of Overview during an
// investigation: e.g. "payment-api changed 12m before the incident started:
// rev 14 (image, env)".
type PinnedChange struct {
	Change WorkloadChangeMarker `json:"change"`
	Text   string               `json:"text"`
	// BeforeIncident is true when the change preceded the first incident.
	BeforeIncident bool `json:"beforeIncident,omitempty"`
}
