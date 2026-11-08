package model

// Finding is one rule's observation about one object, e.g. a pod stuck in
// CrashLoopBackOff or a Service whose selector matches no pods.
type Finding struct {
	Rule     string    `json:"rule"`
	Object   ObjectRef `json:"object"`
	Severity Severity  `json:"severity"`
	// State is a short name for what is wrong, e.g. CrashLoopBackOff or PodCreationRejected.
	State string `json:"state"`
	// Facts are the spec and status details behind State.
	Facts []string `json:"facts,omitempty"`
}

// Problem groups the findings about one workload (or one object that no
// workload owns), so every failing thing in scope is listed once.
type Problem struct {
	// Object is the owner, e.g. "Deployment/checkout", "Job/export", "Service/web".
	Object   string   `json:"object"`
	Severity Severity `json:"severity"`
	State    string   `json:"state"`
	Facts    []string `json:"facts,omitempty"`
	Pods     int      `json:"pods,omitempty"`
	Unready  int      `json:"unready,omitempty"`
	Restarts int32    `json:"restarts,omitempty"`
	// Rules are the rules whose findings make up this problem.
	Rules []string `json:"rules"`
}

// ScopeTarget is the exact scope of an investigation of one object: the
// object, the pods it covers, and the related objects (owners, Services,
// Ingresses, claims, ConfigMaps, Secrets by name) whose state and events count.
type ScopeTarget struct {
	Ref ObjectRef `json:"ref"`
	// Found is false when no such object exists in the namespace.
	Found bool     `json:"found"`
	Pods  []string `json:"pods"`
	// Objects are "Kind/name" keys of the target and related objects.
	Objects []string `json:"objects"`
	// Missing lists targets of a multi-target investigation that do not exist.
	Missing []string `json:"missing,omitempty"`
}

// HasObject reports whether kind/name is the target or related to it.
func (t *ScopeTarget) HasObject(kind, name string) bool {
	if t == nil {
		return true
	}
	key := kind + "/" + name
	for _, o := range t.Objects {
		if o == key {
			return true
		}
	}
	return false
}

// HasPod reports whether the pod is in scope.
func (t *ScopeTarget) HasPod(name string) bool {
	if t == nil {
		return true
	}
	for _, p := range t.Pods {
		if p == name {
			return true
		}
	}
	return false
}

// TargetGroup is one kind of object an investigation can target in a
// namespace, as listed by a collector.
type TargetGroup struct {
	Kind string `json:"kind"`
	// Collector is the collector that lists it; user collectors are marked Custom.
	Collector string   `json:"collector"`
	Custom    bool     `json:"custom,omitempty"`
	Names     []string `json:"names"`
}

// CollectorStatus is one collector's outcome in an investigation, so results
// say plainly what was and was not read.
type CollectorStatus struct {
	Collector string `json:"collector"`
	// Status is collected, no-permission, not-installed, failed or off.
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// TargetList is what Investigate mode can target in a namespace, and the
// collectors whose objects could not be listed, with the reason.
type TargetList struct {
	Groups      []TargetGroup     `json:"groups"`
	Unavailable []CollectorStatus `json:"unavailable,omitempty"`
}

// InvestigableType is a namespaced resource type an investigation can target:
// one that some collector, built-in or user, lists. Kind is set when it is
// known without asking the cluster.
type InvestigableType struct {
	Group     string `json:"group"`
	Resource  string `json:"resource"`
	Kind      string `json:"kind,omitempty"`
	Collector string `json:"collector"`
	Custom    bool   `json:"custom,omitempty"`
}
