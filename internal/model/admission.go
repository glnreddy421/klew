package model

// AdmissionPerimeterSummary is optional investigation enrichment when workloads
// expect pods but none exist — checks cluster admission webhooks (opt-in).
type AdmissionPerimeterSummary struct {
	CheckedAt       Timestamp                    `json:"checkedAt"`
	Status          string                       `json:"status"` // ok | candidates | denied | skipped
	PermissionNote  string                       `json:"permissionNote,omitempty"`
	MissingWorkload bool                         `json:"missingWorkload"`
	Candidates      []AdmissionWebhookCandidate  `json:"candidates,omitempty"`
}

// AdmissionWebhookCandidate is one webhook configuration entry that applies to the namespace.
type AdmissionWebhookCandidate struct {
	ConfigKind          string                    `json:"configKind"` // MutatingWebhookConfiguration | ValidatingWebhookConfiguration
	ConfigName          string                    `json:"configName"`
	WebhookName         string                    `json:"webhookName"`
	NamespaceScopeLabel string                    `json:"namespaceScopeLabel"`
	FailurePolicy       string                    `json:"failurePolicy,omitempty"`
	TimeoutSeconds      int32                     `json:"timeoutSeconds,omitempty"`
	ServiceRef          string                    `json:"serviceRef,omitempty"`
	MatchSummary        string                    `json:"matchSummary,omitempty"`
	RecentSignals       []AdmissionWebhookSignal  `json:"recentSignals,omitempty"`
}

// AdmissionWebhookSignal is a recent admission-related event (no metrics dependency).
type AdmissionWebhookSignal struct {
	Timestamp Timestamp `json:"timestamp"`
	Reason    string    `json:"reason,omitempty"`
	Message   string    `json:"message"`
	Outcome   string    `json:"outcome,omitempty"` // timeout | deny | fail | warn
}
