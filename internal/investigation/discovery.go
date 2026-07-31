package investigation

import (
	"sort"
	"strings"
)

// Object is one namespace-scoped Kubernetes object as seen by the discovery
// engine. Collectors populate a Dataset of these;
// discovery and scope building operate purely on this in-memory view so that
// nothing downstream ever talks to Kubernetes directly.
type Object struct {
	Ref            Ref
	Labels         map[string]string
	Selector       map[string]string // Service/Deployment/PDB/NetworkPolicy pod selector
	Owner          *Ref              // ownerReference (RS→Deployment, Pod→RS, Job→CronJob)
	ConfigMaps     []string          // referenced ConfigMap names
	Secrets        []string          // referenced Secret names
	PVCs           []string          // referenced PVC names
	ServiceAccount string            // ServiceAccount used by workload/pod
	Node           string            // node a Pod is scheduled on
	Target         *Ref              // HPA target / Ingress backend Service / RoleBinding subject
	Role           *Ref              // RoleBinding → Role
	Involved       *Ref              // Event involved object
	CRDGroup       string            // API group for CRD instances (Tier 3)
}

// ExtensionData is an installed operator plus CRD instances related to a workload.
type ExtensionData struct {
	Name string
	CRDs []Ref
}

// Dataset is the complete set of namespace objects discovery searches over.
type Dataset struct {
	Namespace  string
	Objects    []Object
	Extensions []ExtensionData // installed operators + related CRD instances
}

// MatchKind describes how a query matched an object.
type MatchKind string

const (
	MatchExact    MatchKind = "exact"
	MatchPrefix   MatchKind = "prefix"
	MatchContains MatchKind = "contains"
	MatchLabel    MatchKind = "label"
	MatchNone     MatchKind = ""
)

// Match reports whether a query matches a name (or its labels) and how.
func Match(query, name string, labels map[string]string) (MatchKind, bool) {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return MatchNone, false
	}
	n := strings.ToLower(name)
	switch {
	case n == q:
		return MatchExact, true
	case strings.HasPrefix(n, q):
		return MatchPrefix, true
	case strings.Contains(n, q):
		return MatchContains, true
	}
	for k, v := range labels {
		if strings.Contains(strings.ToLower(k), q) || strings.Contains(strings.ToLower(v), q) {
			return MatchLabel, true
		}
	}
	return MatchNone, false
}

// WorkloadGroup is a discovered workload plus a shallow summary of its members.
type WorkloadGroup struct {
	Root    Ref       `json:"root"`
	Match   MatchKind `json:"match"`
	Pods    int       `json:"pods"`
	Service string    `json:"service,omitempty"`
	Members []Ref     `json:"members,omitempty"`
}

// Discover searches a Dataset and groups matches by workload. A workload matches
// when its own name matches the query, or when one of its pods matches. Results
// are returned in a deterministic order (match strength, then name).
func Discover(ds Dataset, query string) []WorkloadGroup {
	idx := newIndex(ds)
	groups := map[string]*WorkloadGroup{}

	consider := func(root Object, mk MatchKind) {
		g, ok := groups[root.Ref.key()]
		if !ok {
			g = &WorkloadGroup{Root: root.Ref, Match: mk}
			groups[root.Ref.key()] = g
		}
		if matchRank(mk) > matchRank(g.Match) {
			g.Match = mk
		}
	}

	for _, o := range ds.Objects {
		if !WorkloadRootKinds[o.Ref.Kind] {
			continue
		}
		if o.Owner != nil { // only top-level controllers root a workload
			continue
		}
		if mk, ok := Match(query, o.Ref.Name, o.Labels); ok {
			consider(o, mk)
			continue
		}
		// match via owned pods
		for _, pod := range idx.podsOf(o.Ref) {
			if mk, ok := Match(query, pod.Name, nil); ok {
				consider(o, mk)
				break
			}
		}
	}

	out := make([]WorkloadGroup, 0, len(groups))
	for _, g := range groups {
		pods := idx.podsOf(g.Root)
		g.Pods = len(pods)
		g.Members = idx.membersOf(g.Root)
		if svc := idx.serviceFor(g.Root); svc != "" {
			g.Service = svc
		}
		out = append(out, *g)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if matchRank(out[i].Match) != matchRank(out[j].Match) {
			return matchRank(out[i].Match) > matchRank(out[j].Match)
		}
		return out[i].Root.Name < out[j].Root.Name
	})
	return out
}

func matchRank(mk MatchKind) int {
	switch mk {
	case MatchExact:
		return 4
	case MatchPrefix:
		return 3
	case MatchContains:
		return 2
	case MatchLabel:
		return 1
	default:
		return 0
	}
}
