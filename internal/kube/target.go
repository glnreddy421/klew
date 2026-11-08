package kube

import (
	"context"
	"slices"
	"sort"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/glnreddy421/klew/v2/internal/model"
)

// Exact scoping. When a query names one object ("deployment/api", or a bare
// name that matches an object exactly), the investigation covers that object,
// the pods it reaches through real relationships, and the objects related to
// those pods. Names are never matched by substring, so "api" does not pull in
// "payment-api".

// targetKindPriority ranks kinds when a bare name matches several objects.
var targetKindPriority = []string{
	"Deployment", "StatefulSet", "DaemonSet", "CronJob", "Job", "Pod", "Service", "Ingress",
	"ReplicaSet", "PersistentVolumeClaim", "ConfigMap", "Secret",
}

// knownTargetKinds are the kinds ParseQuery normalises aliases to.
var knownTargetKinds = map[string]bool{
	"Pod": true, "Deployment": true, "StatefulSet": true, "DaemonSet": true, "ReplicaSet": true,
	"Job": true, "CronJob": true, "Service": true, "Ingress": true, "ConfigMap": true, "Secret": true,
	"PersistentVolumeClaim": true, "HorizontalPodAutoscaler": true, "PodDisruptionBudget": true,
	"ServiceAccount": true, "NetworkPolicy": true, "Role": true, "RoleBinding": true,
	"LimitRange": true, "ResourceQuota": true,
}

// targetRef names the object a query targets, or reports false when the query
// is empty or free text that names no object exactly.
func targetRef(query string, matches []model.MatchedObject) (model.ObjectRef, bool) {
	kind, name, free := ParseQuery(query)
	if name != "" {
		// Kinds from collectors (built-in or user) are written as Kinds,
		// e.g. "Certificate/web-tls"; anything else is not a kind.
		if !knownTargetKinds[kind] && !isKindName(kind) {
			return model.ObjectRef{}, false
		}
		return model.ObjectRef{Kind: kind, Name: name}, true
	}
	if free == "" {
		return model.ObjectRef{}, false
	}
	best, bestRank := model.ObjectRef{}, len(targetKindPriority)
	for _, m := range matches {
		if m.Ref.Name != free {
			continue
		}
		for i, k := range targetKindPriority {
			if k == m.Ref.Kind && i < bestRank {
				best, bestRank = model.ObjectRef{Kind: k, Name: free}, i
			}
		}
	}
	return best, bestRank < len(targetKindPriority)
}

// isKindName reports whether s looks like a Kubernetes kind: an upper-case
// letter followed by letters and digits.
func isKindName(s string) bool {
	if s == "" || s[0] < 'A' || s[0] > 'Z' {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

// scopeLists holds the namespace objects exact scoping is computed from.
type scopeLists struct {
	pods      []corev1.Pod
	rsOwner   map[string]string // ReplicaSet -> Deployment
	jobOwner  map[string]string // Job -> CronJob
	rsNames   map[string]bool
	jobNames  map[string]bool
	services  []corev1.Service
	ingresses []networkingv1.Ingress
}

func (col *Collector) listScope(ctx context.Context, env *CollectEnv) (*scopeLists, bool) {
	if !env.Allowed("pods", "list") {
		return nil, false
	}
	cs := col.Client.Clientset
	ns := env.Namespace
	pods, err := cs.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, false
	}
	l := &scopeLists{pods: pods.Items, rsOwner: map[string]string{}, jobOwner: map[string]string{},
		rsNames: map[string]bool{}, jobNames: map[string]bool{}}
	if env.Allowed("replicasets", "list") {
		if rss, err := cs.AppsV1().ReplicaSets(ns).List(ctx, metav1.ListOptions{}); err == nil {
			for _, rs := range rss.Items {
				l.rsNames[rs.Name] = true
				if d := ownerName(rs.OwnerReferences, "Deployment"); d != "" {
					l.rsOwner[rs.Name] = d
				}
			}
		}
	}
	if env.Allowed("jobs", "list") {
		if jobs, err := cs.BatchV1().Jobs(ns).List(ctx, metav1.ListOptions{}); err == nil {
			for _, j := range jobs.Items {
				l.jobNames[j.Name] = true
				if c := ownerName(j.OwnerReferences, "CronJob"); c != "" {
					l.jobOwner[j.Name] = c
				}
			}
		}
	}
	if env.Allowed("services", "list") {
		if svcs, err := cs.CoreV1().Services(ns).List(ctx, metav1.ListOptions{}); err == nil {
			l.services = svcs.Items
		}
	}
	if env.Allowed("ingresses", "list") {
		if ings, err := cs.NetworkingV1().Ingresses(ns).List(ctx, metav1.ListOptions{}); err == nil {
			l.ingresses = ings.Items
		}
	}
	return l, true
}

func ownerName(refs []metav1.OwnerReference, kind string) string {
	for _, o := range refs {
		if o.Kind == kind {
			return o.Name
		}
	}
	return ""
}

// podOwnerKeys is the pod's owner chain as "Kind/name" keys, e.g.
// ReplicaSet/api-7d9 and Deployment/api.
func (l *scopeLists) podOwnerKeys(p corev1.Pod) []string {
	var out []string
	for _, o := range p.OwnerReferences {
		out = append(out, o.Kind+"/"+o.Name)
		switch o.Kind {
		case "ReplicaSet":
			if d := l.rsOwner[o.Name]; d != "" {
				out = append(out, "Deployment/"+d)
			}
		case "Job":
			if c := l.jobOwner[o.Name]; c != "" {
				out = append(out, "CronJob/"+c)
			}
		}
	}
	return out
}

// resolveTarget computes the exact scope for ref.
func (l *scopeLists) resolveTarget(ref model.ObjectRef, matched bool) *model.ScopeTarget {
	objects := map[string]bool{ref.Kind + "/" + ref.Name: true}
	pods, found := l.targetPods(ref, objects)
	found = found || matched || len(pods) > 0
	podNames := l.addPodRelations(pods, objects)
	l.addTrafficRelations(pods, objects)

	t := &model.ScopeTarget{Ref: ref, Found: found}
	for p := range podNames {
		t.Pods = append(t.Pods, p)
	}
	for o := range objects {
		t.Objects = append(t.Objects, o)
	}
	sort.Strings(t.Pods)
	sort.Strings(t.Objects)
	return t
}

// targetPods returns the pods ref reaches, adds objects it owns that have no
// pods, and reports whether ref was seen in the namespace.
func (l *scopeLists) targetPods(ref model.ObjectRef, objects map[string]bool) ([]corev1.Pod, bool) {
	switch ref.Kind {
	case "Pod":
		return l.podsWhere(func(p corev1.Pod) bool { return p.Name == ref.Name }), false
	case "Deployment", "ReplicaSet", "StatefulSet", "DaemonSet", "Job", "CronJob":
		key := ref.Kind + "/" + ref.Name
		pods := l.podsWhere(func(p corev1.Pod) bool { return slices.Contains(l.podOwnerKeys(p), key) })
		return pods, l.addOwnedObjects(ref, objects)
	case "Service":
		for _, s := range l.services {
			if s.Name == ref.Name {
				return l.selectorPods(s.Spec.Selector), true
			}
		}
	case "Ingress":
		for _, ing := range l.ingresses {
			if ing.Name == ref.Name {
				return l.ingressPods(ing, objects), true
			}
		}
	case "ConfigMap", "Secret", "PersistentVolumeClaim":
		return l.podsWhere(func(p corev1.Pod) bool {
			return slices.ContainsFunc(podRefsOfKind(p, ref.Kind), func(r model.ResourceRef) bool { return r.Name == ref.Name })
		}), false
	}
	return nil, false
}

func (l *scopeLists) podsWhere(keep func(corev1.Pod) bool) []corev1.Pod {
	var out []corev1.Pod
	for _, p := range l.pods {
		if keep(p) {
			out = append(out, p)
		}
	}
	return out
}

func (l *scopeLists) selectorPods(sel map[string]string) []corev1.Pod {
	if len(sel) == 0 {
		return nil
	}
	s := labels.SelectorFromSet(sel)
	return l.podsWhere(func(p corev1.Pod) bool { return s.Matches(labels.Set(p.Labels)) })
}

// ingressPods returns the pods behind an Ingress's backend Services.
func (l *scopeLists) ingressPods(ing networkingv1.Ingress, objects map[string]bool) []corev1.Pod {
	var pods []corev1.Pod
	for _, backend := range summarizeIngress(ing).Backends {
		objects["Service/"+backend] = true
		for _, s := range l.services {
			if s.Name == backend {
				pods = append(pods, l.selectorPods(s.Spec.Selector)...)
			}
		}
	}
	return pods
}

// addOwnedObjects adds objects ref owns even with no pods (a Deployment scaled
// to zero, a ReplicaSet whose pods the API rejects) and reports whether ref
// exists among them.
func (l *scopeLists) addOwnedObjects(ref model.ObjectRef, objects map[string]bool) bool {
	found := false
	switch ref.Kind {
	case "Deployment":
		for rs, d := range l.rsOwner {
			if d == ref.Name {
				objects["ReplicaSet/"+rs] = true
				found = true
			}
		}
	case "CronJob":
		for j, c := range l.jobOwner {
			if c == ref.Name {
				objects["Job/"+j] = true
				found = true
			}
		}
	case "ReplicaSet":
		found = l.rsNames[ref.Name]
		if d := l.rsOwner[ref.Name]; d != "" {
			objects["Deployment/"+d] = true
		}
	case "Job":
		found = l.jobNames[ref.Name]
	}
	return found
}

// podRefsOfKind lists the ConfigMaps, Secrets (names only; values are never
// read) or claims a pod uses.
func podRefsOfKind(p corev1.Pod, kind string) []model.ResourceRef {
	switch kind {
	case "ConfigMap":
		return podConfigRefs(p)
	case "Secret":
		return podSecretRefs(p)
	default:
		return podPVCRefs(p)
	}
}

// addPodRelations adds each pod's owners, claims, config and service account.
func (l *scopeLists) addPodRelations(pods []corev1.Pod, objects map[string]bool) map[string]bool {
	podNames := map[string]bool{}
	for _, p := range pods {
		if podNames[p.Name] {
			continue
		}
		podNames[p.Name] = true
		for _, k := range l.podOwnerKeys(p) {
			objects[k] = true
		}
		for kind, refs := range map[string][]model.ResourceRef{
			"PersistentVolumeClaim": podPVCRefs(p), "ConfigMap": podConfigRefs(p), "Secret": podSecretRefs(p),
		} {
			for _, r := range refs {
				objects[kind+"/"+r.Name] = true
			}
		}
		if sa := p.Spec.ServiceAccountName; sa != "" {
			objects["ServiceAccount/"+sa] = true
		}
	}
	return podNames
}

// addTrafficRelations adds Services that select the pods and Ingresses in
// front of those Services.
func (l *scopeLists) addTrafficRelations(pods []corev1.Pod, objects map[string]bool) {
	for _, s := range l.services {
		if len(s.Spec.Selector) == 0 {
			continue
		}
		sel := labels.SelectorFromSet(s.Spec.Selector)
		if slices.ContainsFunc(pods, func(p corev1.Pod) bool { return sel.Matches(labels.Set(p.Labels)) }) {
			objects["Service/"+s.Name] = true
		}
	}
	for _, ing := range l.ingresses {
		for _, backend := range summarizeIngress(ing).Backends {
			if objects["Service/"+backend] {
				objects["Ingress/"+ing.Name] = true
			}
		}
	}
}

// targetSets indexes a ScopeTarget for fast membership checks.
type targetSets struct {
	pods    map[string]bool
	objects map[string]bool
}

func newTargetSets(t *model.ScopeTarget) *targetSets {
	if t == nil {
		return nil
	}
	s := &targetSets{pods: map[string]bool{}, objects: map[string]bool{}}
	for _, p := range t.Pods {
		s.pods[p] = true
	}
	for _, o := range t.Objects {
		s.objects[o] = true
	}
	return s
}

func (s *targetSets) hasObject(kind, name string) bool { return s == nil || s.objects[kind+"/"+name] }
func (s *targetSets) hasPod(name string) bool          { return s == nil || s.pods[name] }

// eventInTarget keeps events about the target, its pods and related objects,
// and about nodes, which can affect any pod.
func (s *targetSets) eventInTarget(kind, name string) bool {
	if s == nil || kind == "Node" {
		return true
	}
	if kind == "Pod" {
		return s.pods[name]
	}
	return s.objects[kind+"/"+name]
}

// podInTarget decides whether a pod seen after the snapshot belongs to the
// target: a known pod, or one whose owner chain reaches the scope, including
// pods of a new ReplicaSet created by a rollout of a Deployment in scope.
func (s *targetSets) podInTarget(p *corev1.Pod) bool {
	if s == nil || s.pods[p.Name] {
		return true
	}
	for _, o := range p.OwnerReferences {
		if s.objects[o.Kind+"/"+o.Name] {
			return true
		}
		if o.Kind == "ReplicaSet" {
			if hash := p.Labels["pod-template-hash"]; hash != "" && len(o.Name) > len(hash)+1 {
				if s.objects["Deployment/"+o.Name[:len(o.Name)-len(hash)-1]] {
					return true
				}
			}
		}
	}
	return false
}

// resourceKinds maps the built-in generic collectors to their kinds.
var resourceKinds = map[string]string{
	"persistentvolumeclaims": "PersistentVolumeClaim", "configmaps": "ConfigMap", "serviceaccounts": "ServiceAccount",
	"statefulsets": "StatefulSet", "daemonsets": "DaemonSet", "cronjobs": "CronJob",
}

// clusterReference are cluster-scoped kinds rules read for context (does a
// StorageClass exist?); they stay whole.
var clusterReference = map[string]bool{"storageclasses": true, "ingressclasses": true, "gatewayclasses": true}

// narrowResourcesToTarget keeps, in a one-object investigation, only the
// generic-collector objects related to the target: claims, ConfigMaps and
// ServiceAccounts its pods use, budgets and policies that select its pods,
// routes to its Services, and so on. Namespace-wide concerns (OLM, admission
// policies, webhooks) are left out. User collectors are kept whole unless they
// list the target itself.
func narrowResourcesToTarget(_ context.Context, env *CollectEnv, b *model.EvidenceBundle) {
	t := env.target
	if t == nil || b.Resources == nil {
		return
	}
	n := resourceNarrower{t: t, b: b}
	for _, p := range b.Pods {
		n.podLabels = append(n.podLabels, labels.Set(p.Labels))
	}
	n.confirmTarget()
	n.narrowNamed()
	n.narrowSelecting()
	n.narrowRBAC()
	n.narrowRoutes()
	n.dropNamespaceWide()
}

type resourceNarrower struct {
	t         *targetSets
	b         *model.EvidenceBundle
	podLabels []labels.Set
}

func (n resourceNarrower) keep(name string, pred func(obj map[string]any) bool) {
	objs, ok := n.b.Resources[name]
	if !ok {
		return
	}
	var out []map[string]any
	for _, o := range objs {
		if pred(o) {
			out = append(out, o)
		}
	}
	n.b.Resources[name] = out
}

func resourceName(o map[string]any) string { return nestedString(o, "metadata", "name") }

func (n resourceNarrower) isTarget(o map[string]any) bool {
	ref := n.b.Target.Ref
	return nestedString(o, "kind") == ref.Kind && resourceName(o) == ref.Name
}

// confirmTarget marks a target that only a collector lists (a custom kind, a
// PDB, …) as found, and narrows a user collector that lists it to it.
func (n resourceNarrower) confirmTarget() {
	t := n.b.Target
	if t == nil {
		return
	}
	for name, objs := range n.b.Resources {
		if slices.ContainsFunc(objs, n.isTarget) {
			t.Found = true
			if isUserCollector(name) {
				n.keep(name, n.isTarget)
			}
		}
		// A missing target of a multi-target investigation may be a kind only
		// a collector lists.
		t.Missing = slices.DeleteFunc(t.Missing, func(key string) bool {
			return slices.ContainsFunc(objs, func(o map[string]any) bool { return nestedString(o, "kind")+"/"+resourceName(o) == key })
		})
	}
}

// narrowNamed keeps objects the scope names: claims, ConfigMaps, owners, …
func (n resourceNarrower) narrowNamed() {
	for name, kind := range resourceKinds {
		n.keep(name, func(o map[string]any) bool { return n.t.hasObject(kind, resourceName(o)) })
	}
	n.keep("persistentvolumes", func(o map[string]any) bool {
		return n.t.hasObject("PersistentVolumeClaim", nestedString(o, "spec", "claimRef", "name"))
	})
}

// narrowSelecting keeps budgets and policies whose selector matches a pod in
// scope. A missing selector selects nothing; an empty one selects every pod.
func (n resourceNarrower) narrowSelecting() {
	selects := func(sel *metav1.LabelSelector) bool {
		if sel == nil {
			return false
		}
		s, err := metav1.LabelSelectorAsSelector(sel)
		return err == nil && slices.ContainsFunc(n.podLabels, func(l labels.Set) bool { return s.Matches(l) })
	}
	n.keep("poddisruptionbudgets", func(o map[string]any) bool { return selects(labelSelectorAt(o, "spec", "selector")) })
	n.keep("networkpolicies", func(o map[string]any) bool { return selects(labelSelectorAt(o, "spec", "podSelector")) })
}

// narrowRBAC keeps bindings for service accounts in scope and the Roles they grant.
func (n resourceNarrower) narrowRBAC() {
	n.keep("rolebindings", func(o map[string]any) bool {
		subjects, _ := o["subjects"].([]any)
		return slices.ContainsFunc(subjects, func(s any) bool {
			m, _ := s.(map[string]any)
			return nestedString(m, "kind") == "ServiceAccount" && n.t.hasObject("ServiceAccount", nestedString(m, "name"))
		})
	})
	bound := map[string]bool{}
	for _, rb := range n.b.Resources["rolebindings"] {
		if nestedString(rb, "roleRef", "kind") == "Role" {
			bound[nestedString(rb, "roleRef", "name")] = true
		}
	}
	n.keep("roles", func(o map[string]any) bool { return bound[resourceName(o)] })
}

// narrowRoutes keeps HTTPRoutes to Services in scope and their Gateways.
func (n resourceNarrower) narrowRoutes() {
	refsTo := func(o map[string]any, path ...string) []string {
		var out []string
		list, _ := nestedValue(o, path...).([]any)
		for _, item := range list {
			m, _ := item.(map[string]any)
			out = append(out, nestedString(m, "name"))
		}
		return out
	}
	n.keep("httproutes", func(o map[string]any) bool {
		rules, _ := nestedValue(o, "spec", "rules").([]any)
		for _, r := range rules {
			rm, _ := r.(map[string]any)
			if slices.ContainsFunc(refsTo(rm, "backendRefs"), func(s string) bool { return n.t.hasObject("Service", s) }) {
				return true
			}
		}
		return n.t.hasObject("HTTPRoute", resourceName(o))
	})
	parents := map[string]bool{}
	for _, r := range n.b.Resources["httproutes"] {
		for _, p := range refsTo(r, "spec", "parentRefs") {
			parents[p] = true
		}
	}
	n.keep("gateways", func(o map[string]any) bool {
		return parents[resourceName(o)] || n.t.hasObject("Gateway", resourceName(o))
	})
}

// dropNamespaceWide removes namespace-wide concerns (OLM, admission policies,
// webhooks, resource claims), which are not about one object unless they are
// the target.
func (n resourceNarrower) dropNamespaceWide() {
	handled := map[string]bool{"persistentvolumes": true, "poddisruptionbudgets": true, "networkpolicies": true,
		"rolebindings": true, "roles": true, "httproutes": true, "gateways": true}
	for name := range n.b.Resources {
		if _, named := resourceKinds[name]; named || handled[name] || clusterReference[name] || isUserCollector(name) {
			continue
		}
		n.keep(name, func(o map[string]any) bool { return n.t.hasObject(nestedString(o, "kind"), resourceName(o)) })
	}
}

var builtinResourceNames = sync.OnceValue(func() map[string]bool {
	out := map[string]bool{}
	for _, c := range resourceCollectors() {
		out[c.Name()] = true
	}
	return out
})

func isUserCollector(name string) bool { return !builtinResourceNames()[name] }

func nestedValue(obj map[string]any, path ...string) any {
	var cur any = obj
	for _, p := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[p]
	}
	return cur
}

// labelSelectorAt reads a metav1.LabelSelector from an unstructured object.
func labelSelectorAt(obj map[string]any, path ...string) *metav1.LabelSelector {
	m, ok := nestedValue(obj, path...).(map[string]any)
	if !ok {
		return nil
	}
	var sel metav1.LabelSelector
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(m, &sel); err != nil {
		return nil
	}
	return &sel
}

// mergeTargets combines the scope of b into a, keeping a's target.
func mergeTargets(a, b *model.ScopeTarget) *model.ScopeTarget {
	pods := map[string]bool{}
	objects := map[string]bool{}
	for _, t := range []*model.ScopeTarget{a, b} {
		for _, p := range t.Pods {
			pods[p] = true
		}
		for _, o := range t.Objects {
			objects[o] = true
		}
	}
	out := &model.ScopeTarget{Ref: a.Ref, Found: a.Found || b.Found}
	for p := range pods {
		out.Pods = append(out.Pods, p)
	}
	for o := range objects {
		out.Objects = append(out.Objects, o)
	}
	sort.Strings(out.Pods)
	sort.Strings(out.Objects)
	return out
}

// splitTargets splits a query naming several objects ("Deployment/api,
// Service/web") into its parts. Anything else is one part.
func splitTargets(query string) []string {
	var parts []string
	for _, p := range strings.Split(query, ",") {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) < 2 {
		return []string{query}
	}
	for _, p := range parts {
		if _, ok := targetRef(p, nil); !ok || !strings.Contains(p, "/") {
			return []string{query}
		}
	}
	return parts
}

// scopeToTargets scopes the investigation to the union of several objects.
func (col *Collector) scopeToTargets(ctx context.Context, env *CollectEnv, parts []string, bundle *model.EvidenceBundle) {
	lists, ok := col.listScope(ctx, env)
	if !ok {
		bundle.Warnings = append(bundle.Warnings, "Cannot list pods, so the investigation falls back to matching names")
		return
	}
	var merged *model.ScopeTarget
	var kept []model.MatchedObject
	for _, part := range parts {
		ref, _ := targetRef(part, nil)
		ref.Namespace = env.Namespace
		matches, _ := DiscoverMatches(ctx, col.Client, model.NamespaceScope{Primary: env.Namespace, Namespaces: []string{env.Namespace}}, part)
		matched := slices.ContainsFunc(matches, func(m model.MatchedObject) bool { return m.Ref.Kind == ref.Kind && m.Ref.Name == ref.Name })
		t := lists.resolveTarget(ref, matched)
		if !t.Found {
			t.Missing = []string{ref.Kind + "/" + ref.Name}
		}
		if t.Found {
			kept = append(kept, model.MatchedObject{Ref: ref, MatchBy: "name", Score: 1})
		}
		if merged == nil {
			merged = t
			continue
		}
		missing := slices.Concat(merged.Missing, t.Missing)
		merged = mergeTargets(merged, t)
		merged.Missing = missing
	}
	bundle.Target = merged
	env.target = newTargetSets(merged)
	env.DeployNames = nil
	for _, o := range merged.Objects {
		if name, ok := strings.CutPrefix(o, "Deployment/"); ok {
			env.DeployNames = append(env.DeployNames, name)
		}
	}
	env.ScopeNames = env.DeployNames
	bundle.MatchedObjects = kept
}
