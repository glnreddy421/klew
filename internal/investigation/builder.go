package investigation

import "strings"

// BuildScope expands the full relationship graph around a selected workload
// root. Only resources related to the workload are included — never unrelated
// namespace objects.
func BuildScope(ds Dataset, root Ref) InvestigationScope {
	ix := newIndex(ds)
	scope := InvestigationScope{
		RootKind:  root.Kind,
		RootName:  root.Name,
		Namespace: ds.Namespace,
	}
	scope.add(root)

	rootObj, hasRoot := ix.byKey[root.key()]
	podLabels := ix.podLabels(root)

	// ── ownership: root → ReplicaSets/Jobs → Pods → Node/PVC/Config/SA ──
	inScopePods := map[string]bool{}
	for _, child := range ix.children[root.key()] {
		scope.link(root, child.Ref, RelOwns)
		expandChild(ix, &scope, child, inScopePods)
	}
	// standalone pods owned directly by root
	for _, pod := range ix.podsOf(root) {
		if !inScopePods[pod.key()] {
			scope.link(root, pod, RelOwns)
			inScopePods[pod.key()] = true
			if o, ok := ix.byKey[pod.key()]; ok {
				expandPod(&scope, o)
			}
		}
	}

	// ── workload-level config/secret/pvc/serviceaccount references ──
	if hasRoot {
		linkRefs(&scope, root, "ConfigMap", rootObj.ConfigMaps, RelMounts)
		linkRefs(&scope, root, "Secret", rootObj.Secrets, RelMounts)
		linkRefs(&scope, root, "PersistentVolumeClaim", rootObj.PVCs, RelMounts)
		if rootObj.ServiceAccount != "" {
			sa := Ref{Kind: "ServiceAccount", Name: rootObj.ServiceAccount, Namespace: ds.Namespace}
			scope.link(root, sa, RelUses)
		}
	}

	// ── networking: Service → EndpointSlice/Endpoints → Pod; Ingress → Service ──
	services := map[string]Ref{}
	for _, o := range ds.Objects {
		if o.Ref.Kind == "Service" && selectorMatches(o.Selector, podLabels) {
			services[o.Ref.key()] = o.Ref
			scope.add(o.Ref)
			for _, pod := range scope.Pods {
				scope.link(o.Ref, pod, RelSelects)
			}
		}
	}
	for _, o := range ds.Objects {
		switch o.Ref.Kind {
		case "EndpointSlice", "Endpoints":
			if o.Owner != nil && services[o.Owner.key()] != (Ref{}) {
				scope.link(*o.Owner, o.Ref, RelRoutes)
				for _, pod := range scope.Pods {
					scope.link(o.Ref, pod, RelRoutes)
				}
			}
		case "Ingress":
			if o.Target != nil && services[o.Target.key()] != (Ref{}) {
				scope.link(o.Ref, *o.Target, RelRoutes)
			}
		}
	}

	// ── autoscaling / policies / rbac / events ──
	for _, o := range ds.Objects {
		switch o.Ref.Kind {
		case "HorizontalPodAutoscaler":
			if o.Target != nil && o.Target.key() == root.key() {
				scope.link(o.Ref, root, RelTargets)
			}
		case "PodDisruptionBudget", "NetworkPolicy":
			if selectorMatches(o.Selector, podLabels) {
				scope.add(o.Ref)
				for _, pod := range scope.Pods {
					scope.link(o.Ref, pod, RelSelects)
				}
			}
		case "RoleBinding":
			if o.Target != nil && scope.seen[o.Target.key()] {
				scope.link(o.Ref, *o.Target, RelBinds)
				if o.Role != nil {
					scope.link(o.Ref, *o.Role, RelBinds)
				}
			}
		}
	}
	for _, o := range ds.Objects {
		if o.Ref.Kind == "Event" && o.Involved != nil && scope.seen[o.Involved.key()] {
			scope.add(o.Ref)
			scope.link(o.Ref, *o.Involved, RelRelated)
		}
	}

	// ── Tier 3: operator extensions (only CRDs related to this workload) ──
	for _, ext := range ds.Extensions {
		byKind := map[string][]Ref{}
		for _, c := range ext.CRDs {
			if !crdRelated(root, c) {
				continue
			}
			byKind[c.Kind] = append(byKind[c.Kind], c)
			scope.link(root, c, RelRelated)
		}
		if len(byKind) == 0 {
			continue
		}
		scope.Extensions = append(scope.Extensions, ext.Name)
		for kind, refs := range byKind {
			group := ""
			if e, ok := ExtensionForKind(kind); ok {
				group = e.Group
			}
			scope.RelatedCRDs = append(scope.RelatedCRDs, RelatedCRD{
				Extension: ext.Name, Group: group, Kind: kind, Refs: refs})
		}
	}
	return scope
}

// crdRelated reports whether a CRD instance belongs to the workload (by name).
func crdRelated(root, crd Ref) bool {
	return strings.Contains(strings.ToLower(crd.Name), strings.ToLower(root.Name))
}

func expandChild(ix *index, scope *InvestigationScope, child Object, inScopePods map[string]bool) {
	for _, gc := range ix.children[child.Ref.key()] {
		scope.link(child.Ref, gc.Ref, RelOwns)
		if gc.Ref.Kind == "Pod" {
			inScopePods[gc.Ref.key()] = true
			expandPod(scope, gc)
		} else {
			expandChild(ix, scope, gc, inScopePods)
		}
	}
}

func expandPod(scope *InvestigationScope, pod Object) {
	if pod.Node != "" {
		scope.link(pod.Ref, Ref{Kind: "Node", Name: pod.Node}, RelSchedules)
	}
	linkRefs(scope, pod.Ref, "ConfigMap", pod.ConfigMaps, RelMounts)
	linkRefs(scope, pod.Ref, "Secret", pod.Secrets, RelMounts)
	linkRefs(scope, pod.Ref, "PersistentVolumeClaim", pod.PVCs, RelMounts)
	if pod.ServiceAccount != "" {
		scope.link(pod.Ref, Ref{Kind: "ServiceAccount", Name: pod.ServiceAccount, Namespace: pod.Ref.Namespace}, RelUses)
	}
}

func linkRefs(scope *InvestigationScope, from Ref, kind string, names []string, rel RelationKind) {
	for _, n := range names {
		scope.link(from, Ref{Kind: kind, Name: n, Namespace: from.Namespace}, rel)
	}
}
