package kube

import (
	"context"
	"sort"

	"github.com/glnreddy421/klew/v2/internal/model"
)

// InvestigableTypes lists the namespaced resource types an investigation can
// target: the kinds the pod-centric collectors list and every namespaced
// generic collector, including user collectors (extra). It is the one list
// the desktop and MCP use to say what can be investigated; it does not ask
// the cluster, so a type may still be absent or denied there.
func InvestigableTypes(extra []EvidenceCollector) []model.InvestigableType {
	var out []model.InvestigableType
	seen := map[string]bool{}
	add := func(t model.InvestigableType) {
		key := t.Group + "/" + t.Resource
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, t)
	}
	for _, l := range typedTargetListers(context.Background(), nil, "") {
		add(model.InvestigableType{Group: l.access.Group, Resource: l.access.Resource, Kind: l.kind, Collector: l.collector})
	}
	builtin := builtinResourceNames()
	for _, rc := range namespacedResourceCollectors(append(resourceCollectors(), extra...)) {
		add(model.InvestigableType{Group: rc.spec.Group, Resource: rc.spec.Resource, Collector: rc.spec.Name, Custom: !builtin[rc.spec.Name]})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Custom != out[j].Custom && !out[i].Custom })
	return out
}

// namespacedResourceCollectors returns the generic collectors in cs that list
// one namespace, looking inside parallel groups.
func namespacedResourceCollectors(cs []EvidenceCollector) []*resourceCollector {
	var out []*resourceCollector
	for _, c := range cs {
		switch c := c.(type) {
		case parallelCollector:
			out = append(out, namespacedResourceCollectors(c.children)...)
		case *resourceCollector:
			if !c.spec.ClusterScoped {
				out = append(out, c)
			}
		}
	}
	return out
}

// podScopedKinds are kinds a target resolves through the pods that use them,
// without a collector listing the kind itself. Secret values are never read.
var podScopedKinds = []string{"Secret"}

// collectedKinds is every kind collectors can target in this investigation.
// Generic collectors name a resource, not a kind, so their kinds come from
// discovery. ok is false when discovery failed: the caller then cannot say a
// kind is unsupported.
func collectedKinds(env *CollectEnv, collectors []EvidenceCollector) (map[string]bool, bool) {
	kinds := map[string]bool{}
	for _, l := range typedTargetListers(context.Background(), nil, "") {
		kinds[l.kind] = true
	}
	for _, k := range podScopedKinds {
		kinds[k] = true
	}
	if env == nil || env.Client == nil || env.Client.Clientset == nil {
		return kinds, false
	}
	byGV := map[string][]string{}
	for _, rc := range namespacedResourceCollectors(collectors) {
		version := rc.spec.Version
		if version == "" {
			version = env.preferredVersion(rc.spec.Group)
		}
		if version == "" {
			continue // not served: nothing of this group can be a target
		}
		gv := apiVersion(rc.spec.Group, version)
		byGV[gv] = append(byGV[gv], rc.spec.Resource)
	}
	for gv, resources := range byGV {
		list, err := env.Client.Clientset.Discovery().ServerResourcesForGroupVersion(gv)
		if errNotServed(err) {
			continue // that version is not served, so none of its kinds can be a target
		}
		if err != nil {
			return kinds, false
		}
		for _, r := range list.APIResources {
			for _, want := range resources {
				if r.Name == want {
					kinds[r.Kind] = true
				}
			}
		}
	}
	return kinds, true
}
