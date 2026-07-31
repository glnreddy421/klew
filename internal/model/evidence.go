package model

import (
	"strings"
)

// VerdictStatus is the overall workload health classification.
type VerdictStatus string

const (
	VerdictHealthy  VerdictStatus = "healthy"
	VerdictWarning  VerdictStatus = "warning"
	VerdictCritical VerdictStatus = "critical"
	VerdictUnknown  VerdictStatus = "unknown"
)

// Severity for timeline events and signals.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// KubeContext captures cluster identity at collection time.
type KubeContext struct {
	Context   string `json:"context"`
	Cluster   string `json:"cluster"`
	User      string `json:"user"`
	Namespace string `json:"namespace"`
}

// PermissionCheck records RBAC probe results.
type PermissionCheck struct {
	Resource  string `json:"resource"`
	Verb      string `json:"verb"`
	Namespace string `json:"namespace,omitempty"`
	Allowed   bool   `json:"allowed"`
}

// ObjectRef is a lightweight Kubernetes object reference.
type ObjectRef struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	UID       string `json:"uid,omitempty"`
}

// MatchedObject is a query match result.
type MatchedObject struct {
	Ref     ObjectRef `json:"ref"`
	MatchBy string    `json:"matchBy"` // name, label, owner
	Score   float64   `json:"score"`
}

// WorkloadSummary describes a primary investigation target.
type WorkloadSummary struct {
	Kind        string            `json:"kind"`
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	UID         string            `json:"uid,omitempty"`
	Replicas    int32             `json:"replicas"`
	Ready       int32             `json:"ready"`
	Available   int32             `json:"available"`
	Updated     int32             `json:"updated"`
	Generation  int64             `json:"generation"`
	ObservedGen int64             `json:"observedGeneration"`
	Selector    string            `json:"selector,omitempty"`
	Conditions  []string          `json:"conditions,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	RootOwner   *ObjectRef        `json:"rootOwner,omitempty"`
}

// PodSummary is a pod under investigation.
type PodSummary struct {
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace"`
	UID             string            `json:"uid,omitempty"`
	Node            string            `json:"node,omitempty"`
	Phase           string            `json:"phase"`
	Ready           bool              `json:"ready"`
	RestartCount    int32             `json:"restartCount"`
	Containers      []ContainerStatus `json:"containers"`
	OwnerRefs       []ObjectRef       `json:"ownerRefs,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Annotations     map[string]string `json:"annotations,omitempty"`
	CreatedAt       Timestamp         `json:"createdAt"`
	ConfigMapRefs   []string          `json:"configMapRefs,omitempty"`
	SecretRefs      []string          `json:"secretRefs,omitempty"`
	PVCRefs         []string          `json:"pvcRefs,omitempty"`
	RelatedCRDKinds []string          `json:"relatedCRDKinds,omitempty"`
}

// ContainerStatus is container-level state.
type ContainerStatus struct {
	PodName      string     `json:"podName"`
	Name         string     `json:"name"`
	Image        string     `json:"image"`
	Ready        bool       `json:"ready"`
	RestartCount int32      `json:"restartCount"`
	State        string     `json:"state"` // running, waiting, terminated
	Reason       string     `json:"reason,omitempty"`
	ExitCode     int32      `json:"exitCode,omitempty"`
	StartedAt    *Timestamp `json:"startedAt,omitempty"`
	FinishedAt   *Timestamp `json:"finishedAt,omitempty"`
	LastState    string     `json:"lastState,omitempty"`
	LastReason   string     `json:"lastReason,omitempty"`
	LastExitCode int32      `json:"lastExitCode,omitempty"`
	RequestsCPU  string     `json:"requestsCPU,omitempty"`
	RequestsMem  string     `json:"requestsMem,omitempty"`
	LimitsCPU    string     `json:"limitsCPU,omitempty"`
	LimitsMem    string     `json:"limitsMem,omitempty"`
	Command      []string   `json:"command,omitempty"`
	Args         []string   `json:"args,omitempty"`
}

// ReplicaSetSummary links rollout generations.
type ReplicaSetSummary struct {
	Name            string    `json:"name"`
	Namespace       string    `json:"namespace"`
	Replicas        int32     `json:"replicas"`
	Ready           int32     `json:"ready"`
	DeploymentOwner string    `json:"deploymentOwner,omitempty"`
	CreatedAt       Timestamp `json:"createdAt"`
}

// ServiceSummary includes endpoint readiness.
type ServiceSummary struct {
	Name           string   `json:"name"`
	Namespace      string   `json:"namespace"`
	Type           string   `json:"type"`
	ClusterIP      string   `json:"clusterIP"`
	Selector       string   `json:"selector,omitempty"`
	ReadyEndpoints int      `json:"readyEndpoints"`
	TotalEndpoints int      `json:"totalEndpoints"`
	Ports          []string `json:"ports,omitempty"`
}

// IngressSummary routes traffic to services.
type IngressSummary struct {
	Name      string   `json:"name"`
	Namespace string   `json:"namespace"`
	Hosts     []string `json:"hosts,omitempty"`
	Backends  []string `json:"backends,omitempty"`
}

// EventRecord is a normalized Kubernetes event.
type EventRecord struct {
	Timestamp      Timestamp `json:"timestamp"`
	Type           string    `json:"type"`
	Reason         string    `json:"reason"`
	Message        string    `json:"message"`
	Count          int32     `json:"count"`
	Source         string    `json:"source,omitempty"`
	InvolvedObject ObjectRef `json:"involvedObject"`
}

// LogRecord holds container log text.
type LogRecord struct {
	PodName       string    `json:"podName"`
	ContainerName string    `json:"containerName"`
	Previous      bool      `json:"previous"`
	Lines         []string  `json:"lines"`
	Truncated     bool      `json:"truncated"`
	CollectedAt   Timestamp `json:"collectedAt"`
}

// NodeSummary is node pressure context for scheduled pods.
type NodeSummary struct {
	Name              string   `json:"name"`
	Ready             bool     `json:"ready"`
	MemoryPressure    bool     `json:"memoryPressure"`
	DiskPressure      bool     `json:"diskPressure"`
	PIDPressure       bool     `json:"pidPressure"`
	Unschedulable     bool     `json:"unschedulable"`
	KubeletVersion    string   `json:"kubeletVersion,omitempty"`
	Conditions        []string `json:"conditions,omitempty"`
	AllocatableCPUM   int64    `json:"allocatableCpuMillicores,omitempty"`
	AllocatableMemMi  int64    `json:"allocatableMemMi,omitempty"`
	CapacityCPUM      int64    `json:"capacityCpuMillicores,omitempty"`
	CapacityMemMi     int64    `json:"capacityMemMi,omitempty"`
}

// HPASummary horizontal pod autoscaler state.
type HPASummary struct {
	Name            string `json:"name"`
	Namespace       string `json:"namespace"`
	TargetKind      string `json:"targetKind"`
	TargetName      string `json:"targetName"`
	MinReplicas     int32  `json:"minReplicas"`
	MaxReplicas     int32  `json:"maxReplicas"`
	CurrentReplicas int32  `json:"currentReplicas"`
	DesiredReplicas int32  `json:"desiredReplicas"`
	AtMax           bool   `json:"atMax"`
}

// ResourceRef is a config/secret/pvc reference.
type ResourceRef struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	UsedBy    string `json:"usedBy,omitempty"`
}

// MetricsSummary holds aggregate resource usage (metrics.k8s.io) when available.
type MetricsSummary struct {
	Available    bool   `json:"available"`
	CPURequestM  int64  `json:"cpuRequestMillicores"`
	CPULimitM    int64  `json:"cpuLimitMillicores"`
	CPUUsageM    int64  `json:"cpuUsageMillicores"`
	MemRequestMi int64  `json:"memRequestMi"`
	MemLimitMi   int64  `json:"memLimitMi"`
	MemUsageMi   int64  `json:"memUsageMi"`
	Note         string `json:"note,omitempty"`
}

// EvidenceBundle is the collected read-only investigation snapshot.
type EvidenceBundle struct {
	CollectedAt      Timestamp           `json:"collectedAt"`
	KubeContext      KubeContext         `json:"kubeContext"`
	Namespace        string              `json:"namespace"`
	Query            string              `json:"query"`
	MatchedObjects   []MatchedObject     `json:"matchedObjects"`
	Workloads        []WorkloadSummary   `json:"workloads"`
	Pods             []PodSummary        `json:"pods"`
	ReplicaSets      []ReplicaSetSummary `json:"replicaSets"`
	Services         []ServiceSummary    `json:"services"`
	Ingresses        []IngressSummary    `json:"ingresses"`
	Events           []EventRecord       `json:"events"`
	Logs             []LogRecord         `json:"logs"`
	PreviousLogs     []LogRecord         `json:"previousLogs"`
	Nodes            []NodeSummary       `json:"nodes"`
	NodePods         []PodSummary        `json:"nodePods,omitempty"` // co-located pods on investigation node(s)
	HPAs             []HPASummary        `json:"hpas"`
	ConfigRefs       []ResourceRef       `json:"configRefs"`
	SecretRefs       []ResourceRef       `json:"secretRefs"`
	PVCRefs          []ResourceRef       `json:"pvcRefs"`
	Permissions      []PermissionCheck   `json:"permissions"`
	Warnings         []string            `json:"warnings"`
	DetectedCRDKinds []string            `json:"detectedCRDKinds,omitempty"`
	Metrics          MetricsSummary      `json:"metrics"`
}

// TimelineEvent is a correlated incident timeline entry.
type TimelineEvent struct {
	Timestamp      Timestamp `json:"timestamp"`
	Type           string    `json:"type"`
	Severity       Severity  `json:"severity"`
	SourceKind     string    `json:"sourceKind"`
	SourceName     string    `json:"sourceName"`
	Namespace      string    `json:"namespace"`
	Message        string    `json:"message"`
	Reason         string    `json:"reason,omitempty"`
	InvolvedObject ObjectRef `json:"involvedObject"`
	Confidence     float64   `json:"confidence"`
	EvidenceRefs   []string  `json:"evidenceRefs"`
}

// DrilldownTab returns the tab a future "enter to drill down" action should
// open for this timeline event. Navigation is intentionally not wired yet — this
// prepares the event model so every row already knows where it points.
func (e TimelineEvent) DrilldownTab() string {
	reason := strings.ToLower(e.Reason + " " + e.Message)
	switch {
	case e.Type == "metric":
		return "resources"
	case strings.Contains(reason, "oom") || strings.Contains(reason, "crashloop"):
		return "containers"
	}
	switch e.SourceKind {
	case "Deployment", "ReplicaSet", "Service", "Ingress", "Endpoints", "EndpointSlice", "NetworkPolicy":
		return "graph"
	case "Pod", "Container":
		return "containers"
	case "ConfigMap", "Secret", "PersistentVolumeClaim", "PVC":
		return "objects"
	case "Node":
		return "resources"
	}
	if e.Type == "event" || e.Type == "k8s_event" {
		return "events"
	}
	return "objects"
}

// Signal is scored evidence for verdict generation.
type Signal struct {
	ID         string    `json:"id"`
	Label      string    `json:"label"`
	Severity   Severity  `json:"severity"`
	Strength   string    `json:"strength"`         // strong, medium, weak
	Source     string    `json:"source,omitempty"` // EVENT, LOG, OBJECT, METRIC
	Count      int       `json:"count,omitempty"`
	Score      float64   `json:"score"`
	Confidence float64   `json:"confidence,omitempty"`
	Evidence   string    `json:"evidence"`
	ObjectRef  ObjectRef `json:"objectRef,omitempty"`
}

// Verdict is the deterministic investigation conclusion.
type Verdict struct {
	Status                VerdictStatus `json:"status"`
	LeadingSignal         string        `json:"leadingSignal,omitempty"`
	LikelyTrigger         string        `json:"likelyTrigger"`
	Confidence            float64       `json:"confidence"`
	Summary               string        `json:"summary"`
	StrongSignals         []Signal      `json:"strongSignals"`
	MediumSignals         []Signal      `json:"mediumSignals"`
	WeakSignals           []Signal      `json:"weakSignals"`
	AffectedObjects       []ObjectRef   `json:"affectedObjects"`
	AffectedPods          []string      `json:"affectedPods,omitempty"`
	AffectedServices      []string      `json:"affectedServices,omitempty"`
	RecommendedNextChecks []string      `json:"recommendedNextChecks"`
	MissingDataWarnings   []string      `json:"missingDataWarnings,omitempty"`
}

// GraphNode is a node in the workload relationship graph.
type GraphNode struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Health string `json:"health"` // healthy, warning, critical, unknown
}

// GraphEdge connects Kubernetes objects.
type GraphEdge struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Relation   string `json:"relation"`
	Annotation string `json:"annotation,omitempty"`
}

// WorkloadGraph is the ownership and traffic graph.
type WorkloadGraph struct {
	Nodes           []GraphNode       `json:"nodes"`
	Edges           []GraphEdge       `json:"edges"`
	Health          string            `json:"health"`
	HealthByNode    map[string]string `json:"healthByNode,omitempty"`
	PropagationPath []string          `json:"propagationPath"`
}

// Investigation is bundle + engine output for TUI/CLI.
type Investigation struct {
	Bundle   EvidenceBundle  `json:"bundle"`
	Timeline []TimelineEvent `json:"timeline"`
	Graph    WorkloadGraph   `json:"graph"`
	Verdict  Verdict         `json:"verdict"`
}
