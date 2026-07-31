// Package investigation defines Klew's workload-centric investigation scope.
//
// Klew does not investigate namespaces — it investigates workloads. A query
// discovers matching objects across a namespace, groups them by workload, and
// (once a workload is selected) a ScopeBuilder expands the full relationship
// graph around it. Every collector, watcher, panel and correlation engine
// consumes the resulting InvestigationScope; panels never talk to Kubernetes
// directly.
package investigation

import "fmt"

// Ref is a namespaced reference to a Kubernetes object.
type Ref struct {
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Namespace  string `json:"namespace,omitempty"`
	APIVersion string `json:"apiVersion,omitempty"`
}

func (r Ref) String() string {
	if r.Kind == "" {
		return r.Name
	}
	return r.Kind + "/" + r.Name
}

func (r Ref) key() string { return r.Kind + "|" + r.Name }

// RelationKind labels an edge in the investigation graph.
type RelationKind string

const (
	RelOwns      RelationKind = "owns"      // Deployment → ReplicaSet → Pod
	RelSelects   RelationKind = "selects"   // Service/PDB/NetworkPolicy → Pod
	RelRoutes    RelationKind = "routes"    // Ingress → Service → EndpointSlice → Pod
	RelMounts    RelationKind = "mounts"    // Pod/Deployment → ConfigMap/Secret/PVC
	RelTargets   RelationKind = "targets"   // HPA → Deployment
	RelBinds     RelationKind = "binds"     // RoleBinding → ServiceAccount/Role
	RelUses      RelationKind = "uses"      // Deployment → ServiceAccount
	RelSchedules RelationKind = "schedules" // Pod → Node
	RelRelated   RelationKind = "related"   // workload → CRD (operator extension)
)

// Relationship is a directed, traversable edge between two objects.
type Relationship struct {
	From Ref          `json:"from"`
	To   Ref          `json:"to"`
	Kind RelationKind `json:"kind"`
}

// RelatedCRD is an operator/extension custom resource related to the workload.
type RelatedCRD struct {
	Extension string `json:"extension"` // e.g. "Istio", "cert-manager"
	Group     string `json:"group"`     // e.g. "networking.istio.io"
	Kind      string `json:"kind"`      // e.g. "VirtualService"
	Refs      []Ref  `json:"refs"`
}

// InvestigationScope is the complete, workload-centric view of an investigation.
// It is the single source of truth downstream components consume.
type InvestigationScope struct {
	RootKind  string `json:"rootKind"`
	RootName  string `json:"rootName"`
	Namespace string `json:"namespace"`

	// Tier 1 — always in scope
	Deployments    []Ref `json:"deployments,omitempty"`
	StatefulSets   []Ref `json:"statefulSets,omitempty"`
	DaemonSets     []Ref `json:"daemonSets,omitempty"`
	ReplicaSets    []Ref `json:"replicaSets,omitempty"`
	Pods           []Ref `json:"pods,omitempty"`
	Jobs           []Ref `json:"jobs,omitempty"`
	CronJobs       []Ref `json:"cronJobs,omitempty"`
	Services       []Ref `json:"services,omitempty"`
	Ingresses      []Ref `json:"ingresses,omitempty"`
	Endpoints      []Ref `json:"endpoints,omitempty"`
	EndpointSlices []Ref `json:"endpointSlices,omitempty"`
	Nodes          []Ref `json:"nodes,omitempty"`
	Events         []Ref `json:"events,omitempty"`

	// Tier 2 — only if referenced
	ConfigMaps      []Ref `json:"configMaps,omitempty"`
	Secrets         []Ref `json:"secrets,omitempty"`
	PVCs            []Ref `json:"pvcs,omitempty"`
	HPAs            []Ref `json:"hpas,omitempty"`
	ServiceAccounts []Ref `json:"serviceAccounts,omitempty"`
	Roles           []Ref `json:"roles,omitempty"`
	RoleBindings    []Ref `json:"roleBindings,omitempty"`
	PDBs            []Ref `json:"pdbs,omitempty"`
	NetworkPolicies []Ref `json:"networkPolicies,omitempty"`

	// Tier 3 — operator/extension detection
	RelatedCRDs []RelatedCRD `json:"relatedCRDs,omitempty"`
	Extensions  []string     `json:"extensions,omitempty"`

	Relationships []Relationship `json:"relationships,omitempty"`

	seen map[string]bool
}

// ScopeStats is a compact summary of the scope (for headers and bootstrap).
type ScopeStats struct {
	Application   string   `json:"application"`
	Resources     int      `json:"resources"`
	Relationships int      `json:"relationships"`
	Extensions    []string `json:"extensions,omitempty"`
}

// Stats returns a summary of the scope.
func (s *InvestigationScope) Stats() ScopeStats {
	return ScopeStats{
		Application:   fmt.Sprintf("%s/%s", s.RootKind, s.RootName),
		Resources:     s.ResourceCount(),
		Relationships: len(s.Relationships),
		Extensions:    s.Extensions,
	}
}

// ResourceCount returns the number of distinct in-scope resources.
func (s *InvestigationScope) ResourceCount() int {
	n := 0
	for _, g := range s.groups() {
		n += len(*g)
	}
	for _, c := range s.RelatedCRDs {
		n += len(c.Refs)
	}
	return n
}

func (s *InvestigationScope) groups() []*[]Ref {
	return []*[]Ref{
		&s.Deployments, &s.StatefulSets, &s.DaemonSets, &s.ReplicaSets, &s.Pods,
		&s.Jobs, &s.CronJobs, &s.Services, &s.Ingresses, &s.Endpoints,
		&s.EndpointSlices, &s.Nodes, &s.Events, &s.ConfigMaps, &s.Secrets,
		&s.PVCs, &s.HPAs, &s.ServiceAccounts, &s.Roles, &s.RoleBindings,
		&s.PDBs, &s.NetworkPolicies,
	}
}

// add routes a ref into its tier slice, de-duplicating by kind+name.
func (s *InvestigationScope) add(r Ref) {
	if r.Name == "" {
		return
	}
	if s.seen == nil {
		s.seen = map[string]bool{}
	}
	if s.seen[r.key()] {
		return
	}
	s.seen[r.key()] = true
	if r.Namespace == "" {
		r.Namespace = s.Namespace
	}
	switch r.Kind {
	case "Deployment":
		s.Deployments = append(s.Deployments, r)
	case "StatefulSet":
		s.StatefulSets = append(s.StatefulSets, r)
	case "DaemonSet":
		s.DaemonSets = append(s.DaemonSets, r)
	case "ReplicaSet", "ReplicationController":
		s.ReplicaSets = append(s.ReplicaSets, r)
	case "Pod":
		s.Pods = append(s.Pods, r)
	case "Job":
		s.Jobs = append(s.Jobs, r)
	case "CronJob":
		s.CronJobs = append(s.CronJobs, r)
	case "Service":
		s.Services = append(s.Services, r)
	case "Ingress":
		s.Ingresses = append(s.Ingresses, r)
	case "Endpoints":
		s.Endpoints = append(s.Endpoints, r)
	case "EndpointSlice":
		s.EndpointSlices = append(s.EndpointSlices, r)
	case "Node":
		s.Nodes = append(s.Nodes, r)
	case "Event":
		s.Events = append(s.Events, r)
	case "ConfigMap":
		s.ConfigMaps = append(s.ConfigMaps, r)
	case "Secret":
		s.Secrets = append(s.Secrets, r)
	case "PersistentVolumeClaim":
		s.PVCs = append(s.PVCs, r)
	case "HorizontalPodAutoscaler":
		s.HPAs = append(s.HPAs, r)
	case "ServiceAccount":
		s.ServiceAccounts = append(s.ServiceAccounts, r)
	case "Role":
		s.Roles = append(s.Roles, r)
	case "RoleBinding":
		s.RoleBindings = append(s.RoleBindings, r)
	case "PodDisruptionBudget":
		s.PDBs = append(s.PDBs, r)
	case "NetworkPolicy":
		s.NetworkPolicies = append(s.NetworkPolicies, r)
	}
}

// link records a directed relationship (and ensures both ends are in scope).
func (s *InvestigationScope) link(from, to Ref, kind RelationKind) {
	s.add(from)
	s.add(to)
	s.Relationships = append(s.Relationships, Relationship{From: from, To: to, Kind: kind})
}
