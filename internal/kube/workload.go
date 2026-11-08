package kube

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/glnreddy421/klew/internal/model"
)

const maxLogLines = 200

// maxConcurrentLogFetches bounds parallel log requests during a snapshot.
const maxConcurrentLogFetches = 8

// CollectOptions tunes evidence gathering.
type CollectOptions struct {
	Namespace string
	Query     string
	LogLines  int
	// CheckAdmissionPerimeter lists webhook configs when workloads expect pods but none exist (opt-in).
	CheckAdmissionPerimeter bool
	// Collectors run in order after the scope is resolved; nil runs DefaultCollectors.
	Collectors []EvidenceCollector
}

// Collector gathers read-only evidence from a cluster.
type Collector struct {
	Client *Client
}

// Collect resolves the investigation scope, then runs each collector in order
// on one EvidenceBundle.
func (col *Collector) Collect(ctx context.Context, opts CollectOptions) (model.EvidenceBundle, error) {
	ns := opts.Namespace
	if ns == "" {
		ns = col.Client.Namespace
	}
	bundle := model.EvidenceBundle{
		CollectedAt: model.TimestampFrom(time.Now().UTC()),
		Namespace:   ns,
		Query:       opts.Query,
		KubeContext: model.KubeContext{
			Context:   col.Client.Context,
			Cluster:   col.Client.Cluster,
			User:      col.Client.User,
			Namespace: ns,
		},
	}
	collectors := opts.Collectors
	if collectors == nil {
		collectors = DefaultCollectors()
	}
	env := col.resolveScope(ctx, ns, opts, collectors, &bundle)
	for _, c := range collectors {
		if ctx.Err() != nil {
			return bundle, ctx.Err()
		}
		if p, ok := c.(parallelCollector); ok {
			p.Collect(ctx, env, &bundle)
			continue
		}
		env.runGated(ctx, c, &bundle)
	}
	narrowResourcesToTarget(ctx, env, &bundle)
	bundle.Coverage = env.coverageList()
	bundle.Warnings = append(bundle.Warnings, coverageWarning(bundle.Coverage)...)
	bundle.Warnings = append(bundle.Warnings, targetWarnings(env, collectors, bundle.Target, ns)...)
	return bundle, nil
}

// targetWarnings says which targets were not investigated and why: no
// collector reads their kind, or they do not exist in ns.
func targetWarnings(env *CollectEnv, collectors []EvidenceCollector, t *model.ScopeTarget, ns string) []string {
	if t == nil {
		return nil
	}
	missing := t.Missing
	if len(missing) == 0 && !t.Found {
		missing = []string{t.Ref.Kind + "/" + t.Ref.Name}
	}
	if len(missing) == 0 {
		return nil
	}
	kinds, known := collectedKinds(env, collectors)
	var notFound, unsupported []string
	var unsupportedKinds []string
	for _, key := range missing {
		kind, _, _ := strings.Cut(key, "/")
		if known && !kinds[kind] {
			unsupported = append(unsupported, key)
			unsupportedKinds = appendUnique(unsupportedKinds, kind)
			continue
		}
		notFound = append(notFound, key)
	}
	var out []string
	if len(unsupported) > 0 {
		out = append(out, fmt.Sprintf("Not investigated: %s. Klew has no collector for %s; add a Collector extension to read it",
			strings.Join(unsupported, ", "), strings.Join(unsupportedKinds, ", ")))
	}
	switch {
	case len(notFound) == 1 && len(t.Missing) == 0:
		out = append(out, fmt.Sprintf("%s was not found in %s", notFound[0], ns))
	case len(notFound) > 0:
		out = append(out, fmt.Sprintf("Not found in %s: %s", ns, strings.Join(notFound, ", ")))
	}
	return out
}

// safeCollect runs one collector, turning a panic into a warning so a faulty
// collector cannot stop the investigation.
func safeCollect(ctx context.Context, c EvidenceCollector, env *CollectEnv, b *model.EvidenceBundle) {
	defer func() {
		if p := recover(); p != nil {
			b.Warnings = append(b.Warnings, fmt.Sprintf("collector %s failed and was skipped: %v", c.Name(), p))
		}
	}()
	c.Collect(ctx, env, b)
}

// resolveScope checks permissions and finds the Deployments the query names.
func (col *Collector) resolveScope(ctx context.Context, ns string, opts CollectOptions, collectors []EvidenceCollector, bundle *model.EvidenceBundle) *CollectEnv {
	// Review every permission the scope and the collectors need, up front.
	access := checkAccess(ctx, col.Client, ns, declaredAccess(collectors))
	perms := make([]model.PermissionCheck, 0, len(access))
	for _, p := range access {
		perms = append(perms, p)
		if p.Error != "" {
			bundle.Warnings = appendUnique(bundle.Warnings, "Could not check permissions ("+p.Error+"); collectors were tried anyway")
		}
	}
	sortPermissions(perms)
	bundle.Permissions = perms
	for _, a := range scopeAccess {
		if p := access[a.key(ns)]; a.Verb == "watch" && !p.Allowed && p.Error == "" {
			bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("No live updates for %s: needs %s", a.Resource, a))
		}
	}

	matches, err := DiscoverMatches(ctx, col.Client, model.NamespaceScope{Primary: ns, Namespaces: []string{ns}}, opts.Query)
	if err != nil {
		bundle.Warnings = append(bundle.Warnings, err.Error())
	} else {
		bundle.MatchedObjects = matches
	}

	var deployNames []string
	for _, m := range matches {
		if m.Ref.Kind == "Deployment" {
			deployNames = append(deployNames, m.Ref.Name)
		}
	}
	if len(deployNames) == 0 && opts.Query != "" {
		_, name, free := ParseQuery(opts.Query)
		if name != "" {
			deployNames = append(deployNames, name)
		} else if free != "" {
			deployNames = append(deployNames, free)
		}
	}

	// Without a query every Deployment matches, but they must not narrow the
	// investigation: that would drop DaemonSet, StatefulSet, Job and static pods
	// and anything else no Deployment owns.
	var scopeNames []string
	if opts.Query != "" {
		scopeNames = deployNames
	}
	env := &CollectEnv{
		Collector: col, Client: col.Client, Options: opts, Namespace: ns,
		Permissions: perms, DeployNames: deployNames, ScopeNames: scopeNames, access: access,
	}
	col.scopeToTarget(ctx, env, matches, bundle)
	return env
}

// scopeToTarget narrows the investigation to exactly one object when the
// query names one. Free text that names no object keeps name matching.
func (col *Collector) scopeToTarget(ctx context.Context, env *CollectEnv, matches []model.MatchedObject, bundle *model.EvidenceBundle) {
	if parts := splitTargets(env.Options.Query); len(parts) > 1 {
		col.scopeToTargets(ctx, env, parts, bundle)
		return
	}
	ref, ok := targetRef(env.Options.Query, matches)
	if !ok {
		return
	}
	lists, ok := col.listScope(ctx, env)
	if !ok {
		bundle.Warnings = append(bundle.Warnings, "Cannot list pods, so the investigation falls back to matching names")
		return
	}
	ref.Namespace = env.Namespace
	matched := false
	for _, m := range matches {
		if m.Ref.Kind == ref.Kind && m.Ref.Name == ref.Name {
			matched = true
		}
	}
	t := lists.resolveTarget(ref, matched)
	// A bare name covers every object with exactly that name: "gateway" is
	// the Deployment and the Service called gateway.
	if _, name, _ := ParseQuery(env.Options.Query); name == "" {
		for _, m := range matches {
			if m.Ref.Name == ref.Name && m.Ref.Kind != ref.Kind && knownTargetKinds[m.Ref.Kind] {
				t = mergeTargets(t, lists.resolveTarget(model.ObjectRef{Kind: m.Ref.Kind, Name: m.Ref.Name, Namespace: env.Namespace}, true))
			}
		}
	}
	bundle.Target = t
	env.target = newTargetSets(t)
	env.DeployNames, env.ScopeNames = nil, nil
	for _, o := range t.Objects {
		if name, ok := strings.CutPrefix(o, "Deployment/"); ok {
			env.DeployNames = append(env.DeployNames, name)
		}
	}
	env.ScopeNames = env.DeployNames
	var kept []model.MatchedObject
	for _, m := range matches {
		if env.target.hasObject(m.Ref.Kind, m.Ref.Name) || (m.Ref.Kind == "Pod" && env.target.hasPod(m.Ref.Name)) {
			kept = append(kept, m)
		}
	}
	if !matched && t.Found {
		kept = append([]model.MatchedObject{{Ref: ref, MatchBy: "name", Score: 1}}, kept...)
	}
	bundle.MatchedObjects = kept
}

// collectLogs fetches current and previous logs for every container in the
// bundle, at most maxConcurrentLogFetches at a time. Results keep pod and
// container order regardless of which request finishes first.
func (col *Collector) collectLogs(ctx context.Context, ns string, tailLines int, bundle *model.EvidenceBundle) {
	type logFetch struct {
		pod, container string
		previous       bool
		rec            model.LogRecord
		err            error
	}
	var fetches []*logFetch
	for _, pod := range bundle.Pods {
		for _, c := range pod.Containers {
			fetches = append(fetches, &logFetch{pod: pod.Name, container: c.Name})
			if c.RestartCount > 0 || c.LastReason != "" {
				fetches = append(fetches, &logFetch{pod: pod.Name, container: c.Name, previous: true})
			}
		}
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentLogFetches)
	for _, f := range fetches {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			f.rec, f.err = col.fetchLogs(ctx, ns, f.pod, f.container, f.previous, tailLines)
		}()
	}
	wg.Wait()

	for _, f := range fetches {
		switch {
		case f.err == nil && f.previous:
			bundle.PreviousLogs = append(bundle.PreviousLogs, f.rec)
		case f.err == nil:
			bundle.Logs = append(bundle.Logs, f.rec)
		case !f.previous:
			bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("logs %s/%s: %v", f.pod, f.container, f.err))
		}
	}
}

func (col *Collector) collectDeployment(ctx context.Context, ns, name string) (model.WorkloadSummary, error) {
	d, err := col.Client.Clientset.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return model.WorkloadSummary{}, fmt.Errorf("get deployment %s: %w", name, err)
	}
	sel, _ := metav1.LabelSelectorAsSelector(d.Spec.Selector)
	w := model.WorkloadSummary{
		Kind:        "Deployment",
		Name:        d.Name,
		Namespace:   d.Namespace,
		UID:         string(d.UID),
		Replicas:    deref32(d.Spec.Replicas),
		Ready:       d.Status.ReadyReplicas,
		Available:   d.Status.AvailableReplicas,
		Updated:     d.Status.UpdatedReplicas,
		Generation:  d.Generation,
		ObservedGen: d.Status.ObservedGeneration,
		Selector:    sel.String(),
		Labels:      d.Labels,
		Annotations: d.Annotations,
	}
	for _, c := range d.Status.Conditions {
		w.Conditions = append(w.Conditions, fmt.Sprintf("%s=%s", c.Type, c.Status))
		if c.Status == corev1.ConditionFalse {
			w.FailedConditions = append(w.FailedConditions, fmt.Sprintf("%s: %s: %s", c.Type, c.Reason, c.Message))
		}
	}
	return w, nil
}

func summarizeRS(rs appsv1.ReplicaSet) model.ReplicaSetSummary {
	owner := ""
	for _, o := range rs.OwnerReferences {
		if o.Kind == "Deployment" {
			owner = o.Name
		}
	}
	out := model.ReplicaSetSummary{
		Name:            rs.Name,
		Namespace:       rs.Namespace,
		Replicas:        deref32(rs.Spec.Replicas),
		Ready:           rs.Status.ReadyReplicas,
		DeploymentOwner: owner,
		CreatedAt:       model.TimestampFrom(rs.CreationTimestamp.Time),
	}
	for _, c := range rs.Status.Conditions {
		if c.Type == appsv1.ReplicaSetReplicaFailure && c.Status == corev1.ConditionTrue {
			out.CreateFailure = c.Message
		}
	}
	return out
}

func summarizePod(pod corev1.Pod) model.PodSummary {
	ps := model.PodSummary{
		Name:        pod.Name,
		Namespace:   pod.Namespace,
		UID:         string(pod.UID),
		Node:        pod.Spec.NodeName,
		Phase:       string(pod.Status.Phase),
		Labels:      pod.Labels,
		Annotations: pod.Annotations,
		CreatedAt:   model.TimestampFrom(pod.CreationTimestamp.Time),

		ServiceAccount: pod.Spec.ServiceAccountName,
	}
	for _, o := range pod.OwnerReferences {
		ps.OwnerRefs = append(ps.OwnerRefs, model.ObjectRef{Kind: o.Kind, Name: o.Name, UID: string(o.UID)})
	}
	statuses := map[string]corev1.ContainerStatus{}
	for _, cs := range pod.Status.ContainerStatuses {
		statuses[cs.Name] = cs
	}
	// A pod with no container statuses (not yet scheduled or started) is not
	// ready. Spec containers without a status are still listed so their
	// requests and probes are visible.
	ready := len(pod.Spec.Containers) > 0
	var restarts int32
	initStatuses := map[string]corev1.ContainerStatus{}
	for _, cs := range pod.Status.InitContainerStatuses {
		initStatuses[cs.Name] = cs
	}
	for _, c := range pod.Spec.InitContainers {
		cs, ok := initStatuses[c.Name]
		if !ok {
			continue
		}
		restarts += cs.RestartCount
		ic := containerFromSpec(pod.Name, cs, c)
		ic.Init = true
		// A completed init container did its job; only one that has not
		// finished can hold the pod back.
		ic.Ready = cs.Ready || (cs.State.Terminated != nil && cs.State.Terminated.ExitCode == 0)
		ps.Containers = append(ps.Containers, ic)
	}
	for _, c := range pod.Spec.Containers {
		cs, ok := statuses[c.Name]
		if !ok {
			cs = corev1.ContainerStatus{Name: c.Name, Image: c.Image}
		}
		if !cs.Ready {
			ready = false
		}
		restarts += cs.RestartCount
		ps.Containers = append(ps.Containers, containerFromSpec(pod.Name, cs, c))
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionFalse {
			ps.Unschedulable = strings.TrimSpace(cond.Reason + ": " + cond.Message)
		}
	}
	ps.Ready = ready
	ps.RestartCount = restarts
	ps.ConfigMapRefs = podConfigMapNames(pod)
	ps.SecretRefs = podSecretNames(pod)
	ps.PVCRefs = podPVCNames(pod)
	return ps
}

func podConfigMapNames(pod corev1.Pod) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, ref := range podConfigRefs(pod) {
		if _, ok := seen[ref.Name]; ok {
			continue
		}
		seen[ref.Name] = struct{}{}
		out = append(out, ref.Name)
	}
	return out
}

func podSecretNames(pod corev1.Pod) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, ref := range podSecretRefs(pod) {
		if _, ok := seen[ref.Name]; ok {
			continue
		}
		seen[ref.Name] = struct{}{}
		out = append(out, ref.Name)
	}
	return out
}

func podPVCNames(pod corev1.Pod) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, ref := range podPVCRefs(pod) {
		if _, ok := seen[ref.Name]; ok {
			continue
		}
		seen[ref.Name] = struct{}{}
		out = append(out, ref.Name)
	}
	return out
}

// containerFromSpec combines a container's status with its spec: resources,
// command, ports, probes and mounts.
func containerFromSpec(podName string, cs corev1.ContainerStatus, c corev1.Container) model.ContainerStatus {
	out := model.ContainerStatus{
		PodName:      podName,
		Name:         cs.Name,
		Image:        cs.Image,
		Ready:        cs.Ready,
		RestartCount: cs.RestartCount,
	}
	if c.Resources.Requests != nil {
		out.RequestsCPU = c.Resources.Requests.Cpu().String()
		out.RequestsMem = c.Resources.Requests.Memory().String()
	}
	if c.Resources.Limits != nil {
		out.LimitsCPU = c.Resources.Limits.Cpu().String()
		out.LimitsMem = c.Resources.Limits.Memory().String()
	}
	if len(c.Command) > 0 {
		out.Command = append([]string(nil), c.Command...)
	}
	if len(c.Args) > 0 {
		out.Args = append([]string(nil), c.Args...)
	}
	for _, p := range c.Ports {
		out.Ports = append(out.Ports, fmt.Sprintf("%d/%s", p.ContainerPort, p.Protocol))
	}
	out.ReadinessProbe = describeProbe(c.ReadinessProbe)
	out.LivenessProbe = describeProbe(c.LivenessProbe)
	for _, m := range c.VolumeMounts {
		if strings.HasPrefix(m.Name, "kube-api-access-") {
			continue
		}
		out.Mounts = append(out.Mounts, fmt.Sprintf("%s (%s)", m.MountPath, m.Name))
	}
	switch {
	case cs.State.Running != nil:
		out.State = "running"
		out.StartedAt = model.TimestampPtrFrom(&cs.State.Running.StartedAt.Time)
	case cs.State.Waiting != nil:
		out.State = "waiting"
		out.Reason = cs.State.Waiting.Reason
		out.Message = cs.State.Waiting.Message
	case cs.State.Terminated != nil:
		out.State = "terminated"
		out.Reason = cs.State.Terminated.Reason
		out.Message = cs.State.Terminated.Message
		out.ExitCode = cs.State.Terminated.ExitCode
		out.FinishedAt = model.TimestampPtrFrom(&cs.State.Terminated.FinishedAt.Time)
	}
	if cs.LastTerminationState.Terminated != nil {
		out.LastState = "terminated"
		out.LastReason = cs.LastTerminationState.Terminated.Reason
		out.LastExitCode = cs.LastTerminationState.Terminated.ExitCode
	} else if cs.LastTerminationState.Waiting != nil {
		out.LastState = "waiting"
		out.LastReason = cs.LastTerminationState.Waiting.Reason
	}
	return out
}

// describeProbe renders a probe's action compactly, e.g. "http-get :8080/healthz".
func describeProbe(p *corev1.Probe) string {
	if p == nil {
		return ""
	}
	switch h := p.ProbeHandler; {
	case h.HTTPGet != nil:
		return fmt.Sprintf("http-get :%s%s", h.HTTPGet.Port.String(), h.HTTPGet.Path)
	case h.TCPSocket != nil:
		return "tcp :" + h.TCPSocket.Port.String()
	case h.GRPC != nil:
		return fmt.Sprintf("grpc :%d", h.GRPC.Port)
	case h.Exec != nil:
		return "exec " + strings.Join(h.Exec.Command, " ")
	}
	return ""
}

func (col *Collector) collectServices(ctx context.Context, ns string, selectors []labels.Selector, deployNames []string, perms []model.PermissionCheck) []model.ServiceSummary {
	var out []model.ServiceSummary
	svcs, err := col.Client.Clientset.CoreV1().Services(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return out
	}
	for _, svc := range svcs.Items {
		if len(deployNames) > 0 && !stringSliceContains(deployNames, svc.Name) && !selectorMatchesAny(selectors, labels.Set(svc.Spec.Selector)) {
			// still include if name fuzzy matches deploy
			match := false
			for _, d := range deployNames {
				if strings.Contains(svc.Name, d) {
					match = true
					break
				}
			}
			if !match {
				continue
			}
		}
		sum := model.ServiceSummary{
			Name:      svc.Name,
			Namespace: svc.Namespace,
			Type:      string(svc.Spec.Type),
			ClusterIP: svc.Spec.ClusterIP,
			Selector:  labels.Set(svc.Spec.Selector).String(),
		}
		for _, p := range svc.Spec.Ports {
			sum.Ports = append(sum.Ports, fmt.Sprintf("%d/%s", p.Port, p.Protocol))
		}
		if allowed(perms, "endpointslices", "list") {
			sum.ReadyEndpoints, sum.TotalEndpoints = col.serviceEndpointCounts(ctx, ns, svc.Name)
		}
		out = append(out, sum)
	}
	return out
}

func (col *Collector) serviceEndpointCounts(ctx context.Context, ns, svcName string) (ready, total int) {
	slices, err := col.Client.Clientset.DiscoveryV1().EndpointSlices(ns).List(ctx, metav1.ListOptions{
		LabelSelector: discoveryv1.LabelServiceName + "=" + svcName,
	})
	if err != nil {
		return 0, 0
	}
	return countReadyEndpointSliceAddresses(slices.Items)
}

// countReadyEndpointSliceAddresses counts every endpoint address in total and
// the ready ones (nil Ready is treated as ready) in ready, so a Service whose
// pods all fail readiness reads 0/N rather than 0/0.
func countReadyEndpointSliceAddresses(slices []discoveryv1.EndpointSlice) (ready, total int) {
	for _, sl := range slices {
		for _, ep := range sl.Endpoints {
			n := len(ep.Addresses)
			total += n
			if ep.Conditions.Ready == nil || *ep.Conditions.Ready {
				ready += n
			}
		}
	}
	return ready, total
}

func summarizeIngress(ing networkingv1.Ingress) model.IngressSummary {
	s := model.IngressSummary{Name: ing.Name, Namespace: ing.Namespace}
	if ing.Spec.IngressClassName != nil {
		s.ClassName = *ing.Spec.IngressClassName
	}
	if d := ing.Spec.DefaultBackend; d != nil && d.Service != nil {
		s.Backends = append(s.Backends, d.Service.Name)
	}
	for _, r := range ing.Spec.Rules {
		if r.Host != "" {
			s.Hosts = append(s.Hosts, r.Host)
		}
		if r.HTTP != nil {
			for _, p := range r.HTTP.Paths {
				if p.Backend.Service != nil {
					s.Backends = append(s.Backends, p.Backend.Service.Name)
				}
			}
		}
	}
	return s
}

func summarizeHPA(h autoscalingv2.HorizontalPodAutoscaler) model.HPASummary {
	sum := model.HPASummary{
		Name:            h.Name,
		Namespace:       h.Namespace,
		MinReplicas:     deref32(h.Spec.MinReplicas),
		MaxReplicas:     h.Spec.MaxReplicas,
		CurrentReplicas: h.Status.CurrentReplicas,
		DesiredReplicas: h.Status.DesiredReplicas,
	}
	if h.Spec.ScaleTargetRef.Kind != "" {
		sum.TargetKind = h.Spec.ScaleTargetRef.Kind
		sum.TargetName = h.Spec.ScaleTargetRef.Name
	}
	sum.AtMax = sum.CurrentReplicas >= sum.MaxReplicas && sum.MaxReplicas > 0
	return sum
}

func (col *Collector) collectNodes(ctx context.Context, pods []model.PodSummary) []model.NodeSummary {
	nodeNames := map[string]bool{}
	for _, p := range pods {
		if p.Node != "" {
			nodeNames[p.Node] = true
		}
	}
	var out []model.NodeSummary
	for name := range nodeNames {
		n, err := col.Client.Clientset.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			continue
		}
		sum := model.NodeSummary{Name: n.Name, KubeletVersion: n.Status.NodeInfo.KubeletVersion}
		if cpu := n.Status.Allocatable.Cpu(); cpu != nil {
			sum.AllocatableCPUM = cpu.MilliValue()
		}
		if mem := n.Status.Allocatable.Memory(); mem != nil {
			sum.AllocatableMemMi = mem.Value() / (1024 * 1024)
		}
		if cpu := n.Status.Capacity.Cpu(); cpu != nil {
			sum.CapacityCPUM = cpu.MilliValue()
		}
		if mem := n.Status.Capacity.Memory(); mem != nil {
			sum.CapacityMemMi = mem.Value() / (1024 * 1024)
		}
		for _, c := range n.Status.Conditions {
			switch c.Type {
			case corev1.NodeReady:
				sum.Ready = c.Status == corev1.ConditionTrue
			case corev1.NodeMemoryPressure:
				sum.MemoryPressure = c.Status == corev1.ConditionTrue
			case corev1.NodeDiskPressure:
				sum.DiskPressure = c.Status == corev1.ConditionTrue
			case corev1.NodePIDPressure:
				sum.PIDPressure = c.Status == corev1.ConditionTrue
			}
			sum.Conditions = append(sum.Conditions, fmt.Sprintf("%s=%s", c.Type, c.Status))
		}
		sum.Unschedulable = n.Spec.Unschedulable
		out = append(out, sum)
	}
	return out
}

func (col *Collector) collectPodsOnNodes(ctx context.Context, nodes []model.NodeSummary, scoped []model.PodSummary) []model.PodSummary {
	scopedKeys := map[string]bool{}
	for _, p := range scoped {
		scopedKeys[p.Namespace+"/"+p.Name] = true
	}
	var out []model.PodSummary
	seen := map[string]bool{}
	for _, node := range nodes {
		list, err := col.Client.Clientset.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
			FieldSelector: "spec.nodeName=" + node.Name,
		})
		if err != nil {
			continue
		}
		for _, pod := range list.Items {
			if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
				continue
			}
			key := pod.Namespace + "/" + pod.Name
			if scopedKeys[key] || seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, summarizePod(pod))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Node != out[j].Node {
			return out[i].Node < out[j].Node
		}
		return out[i].Namespace+"/"+out[i].Name < out[j].Namespace+"/"+out[j].Name
	})
	return out
}

func podConfigRefs(pod corev1.Pod) []model.ResourceRef {
	var refs []model.ResourceRef
	for _, v := range pod.Spec.Volumes {
		if v.ConfigMap != nil {
			refs = append(refs, model.ResourceRef{Kind: "ConfigMap", Name: v.ConfigMap.Name, Namespace: pod.Namespace, UsedBy: pod.Name})
		}
	}
	for _, c := range append(pod.Spec.InitContainers, pod.Spec.Containers...) {
		for _, e := range c.Env {
			if e.ValueFrom != nil && e.ValueFrom.ConfigMapKeyRef != nil {
				name := e.ValueFrom.ConfigMapKeyRef.Name
				refs = append(refs, model.ResourceRef{Kind: "ConfigMap", Name: name, Namespace: pod.Namespace, UsedBy: pod.Name})
			}
		}
		for _, ef := range c.EnvFrom {
			if ef.ConfigMapRef != nil {
				refs = append(refs, model.ResourceRef{Kind: "ConfigMap", Name: ef.ConfigMapRef.Name, Namespace: pod.Namespace, UsedBy: pod.Name})
			}
		}
	}
	return refs
}

func podSecretRefs(pod corev1.Pod) []model.ResourceRef {
	var refs []model.ResourceRef
	for _, v := range pod.Spec.Volumes {
		if v.Secret != nil {
			refs = append(refs, model.ResourceRef{Kind: "Secret", Name: v.Secret.SecretName, Namespace: pod.Namespace, UsedBy: pod.Name})
		}
	}
	for _, c := range append(pod.Spec.InitContainers, pod.Spec.Containers...) {
		for _, e := range c.Env {
			if e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil {
				name := e.ValueFrom.SecretKeyRef.Name
				refs = append(refs, model.ResourceRef{Kind: "Secret", Name: name, Namespace: pod.Namespace, UsedBy: pod.Name})
			}
		}
		for _, ef := range c.EnvFrom {
			if ef.SecretRef != nil {
				refs = append(refs, model.ResourceRef{Kind: "Secret", Name: ef.SecretRef.Name, Namespace: pod.Namespace, UsedBy: pod.Name})
			}
		}
	}
	return refs
}

func podPVCRefs(pod corev1.Pod) []model.ResourceRef {
	var refs []model.ResourceRef
	for _, v := range pod.Spec.Volumes {
		if v.PersistentVolumeClaim != nil {
			refs = append(refs, model.ResourceRef{Kind: "PVC", Name: v.PersistentVolumeClaim.ClaimName, Namespace: pod.Namespace, UsedBy: pod.Name})
		}
	}
	return refs
}

func matchesWorkload(deployNames []string, rsName string, owners []metav1.OwnerReference) bool {
	if len(deployNames) == 0 {
		return true
	}
	for _, o := range owners {
		if o.Kind == "Deployment" && stringSliceContains(deployNames, o.Name) {
			return true
		}
	}
	for _, d := range deployNames {
		if strings.Contains(rsName, d) {
			return true
		}
	}
	return false
}

func podMatchesQuery(pod corev1.Pod, deployNames []string, selectors []labels.Selector) bool {
	if len(deployNames) == 0 && len(selectors) == 0 {
		return true
	}
	for _, o := range pod.OwnerReferences {
		if o.Kind == "ReplicaSet" || o.Kind == "Deployment" {
			for _, d := range deployNames {
				if strings.Contains(o.Name, d) || strings.Contains(pod.Name, d) {
					return true
				}
			}
		}
	}
	for _, s := range selectors {
		if s.Matches(labels.Set(pod.Labels)) {
			return true
		}
	}
	for _, d := range deployNames {
		if strings.Contains(pod.Name, d) {
			return true
		}
	}
	return false
}

func ingressMatches(ing networkingv1.Ingress, deployNames []string) bool {
	if len(deployNames) == 0 {
		return true
	}
	for _, b := range summarizeIngress(ing).Backends {
		for _, d := range deployNames {
			if strings.Contains(b, d) {
				return true
			}
		}
	}
	return false
}

func hpaMatches(h autoscalingv2.HorizontalPodAutoscaler, deployNames []string) bool {
	if len(deployNames) == 0 {
		return true
	}
	return stringSliceContains(deployNames, h.Spec.ScaleTargetRef.Name)
}

func selectorMatchesAny(selectors []labels.Selector, set labels.Set) bool {
	for _, s := range selectors {
		if s.Matches(set) {
			return true
		}
	}
	return false
}

func stringSliceContains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

func deref32(p *int32) int32 {
	if p == nil {
		return 0
	}
	return *p
}
