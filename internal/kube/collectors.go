package kube

import (
	"context"
	"fmt"
	"strings"
	"sync"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/glnreddy421/klew/v2/internal/model"
)

// EvidenceCollector adds one kind of evidence to a bundle. Collectors run in
// order, so one may read what earlier ones added (logs need pods). They must
// only list, get or watch: Klew never changes the cluster.
type EvidenceCollector interface {
	Name() string
	Collect(ctx context.Context, env *CollectEnv, b *model.EvidenceBundle)
}

// CollectEnv is the scope every collector shares, resolved before any runs.
type CollectEnv struct {
	Collector   *Collector
	Client      *Client
	Options     CollectOptions
	Namespace   string
	Permissions []model.PermissionCheck
	// DeployNames are the Deployments the query matched; without a query, every Deployment.
	DeployNames []string
	// ScopeNames narrow workload-owned objects to the query; empty means the whole namespace.
	ScopeNames []string
	// Selectors are pod selectors of the matched ReplicaSets, set by the
	// replicasets collector when there is a query.
	Selectors []labels.Selector

	// target, when set, is the exact scope of a one-object investigation;
	// collectors keep only what it contains instead of matching names.
	target *targetSets

	// access holds the reviewed permissions, keyed by Access.key.
	access map[string]model.PermissionCheck

	mu         sync.Mutex
	coverage   map[string]model.CollectorStatus
	groupsOnce sync.Once
	groups     *metav1.APIGroupList
	// served caches discovery of each group version's resources, by
	// "group/version"; a nil entry means discovery failed for it.
	servedMu sync.Mutex
	served   map[string]map[string]bool

	// deployments are the in-scope Deployments the workloads collector read,
	// by name, so later collectors need not read them again.
	deployments map[string]*appsv1.Deployment
	// rsOnce lists the namespace's ReplicaSets once for every collector.
	rsOnce sync.Once
	rsList []appsv1.ReplicaSet
	rsErr  error
}

// replicaSets lists the namespace's ReplicaSets on first use and returns the
// same list afterwards. Callers must not modify it.
func (e *CollectEnv) replicaSets(ctx context.Context) ([]appsv1.ReplicaSet, error) {
	e.rsOnce.Do(func() {
		list, err := e.Client.Clientset.AppsV1().ReplicaSets(e.Namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			e.rsErr = err
			return
		}
		e.rsList = list.Items
	})
	return e.rsList, e.rsErr
}

// Allowed reports whether the permission probe allowed verb on resource.
func (e *CollectEnv) Allowed(resource, verb string) bool {
	return allowed(e.Permissions, resource, verb)
}

type collectorFunc struct {
	name   string
	access []Access
	fn     func(ctx context.Context, env *CollectEnv, b *model.EvidenceBundle)
}

func (c collectorFunc) Name() string     { return c.name }
func (c collectorFunc) Access() []Access { return c.access }
func (c collectorFunc) Collect(ctx context.Context, env *CollectEnv, b *model.EvidenceBundle) {
	c.fn(ctx, env, b)
}

// NewCollector wraps fn as a named collector that checks access itself.
func NewCollector(name string, fn func(ctx context.Context, env *CollectEnv, b *model.EvidenceBundle)) EvidenceCollector {
	return collectorFunc{name: name, fn: fn}
}

// NewCollectorWithAccess wraps fn as a collector that runs only when every
// access is allowed (or cannot be reviewed); otherwise it is reported as
// no-permission.
func NewCollectorWithAccess(name string, access []Access, fn func(ctx context.Context, env *CollectEnv, b *model.EvidenceBundle)) EvidenceCollector {
	return collectorFunc{name: name, access: access, fn: fn}
}

func list(group, resource string) Access {
	return Access{Group: group, Resource: resource, Verb: "list"}
}

// DefaultCollectors returns Klew's built-in collectors in run order.
func DefaultCollectors() []EvidenceCollector {
	return []EvidenceCollector{
		NewCollectorWithAccess("workloads", []Access{list("apps", "deployments"), {Group: "apps", Resource: "deployments", Verb: "get"}}, collectWorkloads),
		NewCollectorWithAccess("changes", []Access{list("apps", "replicasets")}, collectChanges),
		NewCollectorWithAccess("replicasets", []Access{list("apps", "replicasets")}, collectReplicaSets),
		NewCollectorWithAccess("pods", []Access{list("", "pods")}, collectPods),
		NewCollectorWithAccess("jobs", []Access{list("batch", "jobs")}, collectJobs),
		NewCollectorWithAccess("services", []Access{list("", "services")}, collectServicesStep),
		NewCollectorWithAccess("ingresses", []Access{list("networking.k8s.io", "ingresses")}, collectIngresses),
		NewCollectorWithAccess("events", []Access{list("", "events")}, collectEventsStep),
		NewCollectorWithAccess("logs", []Access{{Resource: "pods", Subresource: "log", Verb: "get"}}, collectLogsStep),
		NewCollectorWithAccess("hpas", []Access{list("autoscaling", "horizontalpodautoscalers")}, collectHPAs),
		NewCollectorWithAccess("nodes", []Access{{Resource: "nodes", Verb: "list", ClusterScoped: true}}, collectNodesStep),
		NewCollectorWithAccess("node-pods", []Access{list("", "pods")}, collectNodePods),
		NewCollector("admission", collectAdmission),
		NewCollector("metrics", collectMetricsStep),
		parallelCollector{name: "resources", children: resourceCollectors()},
	}
}

// ResourceCollectorNames lists the built-in generic resource collectors,
// whose objects rules can read by name.
func ResourceCollectorNames() []string {
	var out []string
	for _, c := range resourceCollectors() {
		out = append(out, c.Name())
	}
	return out
}

// DefaultCollectorNames lists every built-in collector, including the
// generic resource collectors that run as one parallel group.
func DefaultCollectorNames() []string {
	var out []string
	for _, c := range DefaultCollectors() {
		if p, ok := c.(parallelCollector); ok {
			for _, child := range p.children {
				out = append(out, child.Name())
			}
			continue
		}
		out = append(out, c.Name())
	}
	return out
}

func collectWorkloads(ctx context.Context, env *CollectEnv, b *model.EvidenceBundle) {
	if !env.Allowed("deployments", "list") {
		return
	}
	if b.Target != nil && !b.Target.Found {
		return // the coverage note says the target was not found
	}
	deployments, warnings := env.Collector.readDeployments(ctx, env.Namespace, env.DeployNames)
	b.Warnings = append(b.Warnings, warnings...)
	env.deployments = deployments
	for _, dn := range env.DeployNames {
		if d := deployments[dn]; d != nil {
			b.Workloads = append(b.Workloads, summarizeDeployment(d))
		}
	}
}

func collectChanges(ctx context.Context, env *CollectEnv, b *model.EvidenceBundle) {
	if !env.Allowed("replicasets", "list") {
		return
	}
	// Errors are reported by the replicasets collector; markers are optional.
	rss, err := env.replicaSets(ctx)
	if err != nil {
		return
	}
	b.WorkloadChanges = env.Collector.collectChangeMarkers(ctx, b.Workloads, env.deployments, rss)
}

func collectReplicaSets(ctx context.Context, env *CollectEnv, b *model.EvidenceBundle) {
	if !env.Allowed("replicasets", "list") {
		return
	}
	rss, err := env.replicaSets(ctx)
	if err != nil {
		b.Warnings = append(b.Warnings, fmt.Sprintf("list replicasets: %v", err))
		return
	}
	for _, rs := range rss {
		if env.target != nil {
			if env.target.hasObject("ReplicaSet", rs.Name) {
				b.ReplicaSets = append(b.ReplicaSets, summarizeRS(rs))
			}
			continue
		}
		if !matchesWorkload(env.DeployNames, rs.Name, rs.OwnerReferences) {
			continue
		}
		b.ReplicaSets = append(b.ReplicaSets, summarizeRS(rs))
		if env.Options.Query == "" {
			continue
		}
		if s, err := metav1.LabelSelectorAsSelector(rs.Spec.Selector); err == nil {
			env.Selectors = append(env.Selectors, s)
		}
	}
}

func collectPods(ctx context.Context, env *CollectEnv, b *model.EvidenceBundle) {
	if !env.Allowed("pods", "list") {
		return
	}
	pods, err := env.Client.Clientset.CoreV1().Pods(env.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		b.Warnings = append(b.Warnings, fmt.Sprintf("list pods: %v", err))
		return
	}
	for _, pod := range pods.Items {
		if env.target != nil && !env.target.hasPod(pod.Name) {
			continue
		}
		if env.target == nil && !podMatchesQuery(pod, env.ScopeNames, env.Selectors) {
			continue
		}
		b.Pods = append(b.Pods, summarizePod(pod))
		b.ConfigRefs = append(b.ConfigRefs, podConfigRefs(pod)...)
		b.SecretRefs = append(b.SecretRefs, podSecretRefs(pod)...)
		b.PVCRefs = append(b.PVCRefs, podPVCRefs(pod)...)
	}
}

func collectServicesStep(ctx context.Context, env *CollectEnv, b *model.EvidenceBundle) {
	if !env.Allowed("services", "list") {
		return
	}
	if env.target == nil {
		b.Services = env.Collector.collectServices(ctx, env.Namespace, env.Selectors, env.ScopeNames, env.Permissions)
		return
	}
	for _, svc := range env.Collector.collectServices(ctx, env.Namespace, nil, nil, env.Permissions) {
		if env.target.hasObject("Service", svc.Name) {
			b.Services = append(b.Services, svc)
		}
	}
}

func collectIngresses(ctx context.Context, env *CollectEnv, b *model.EvidenceBundle) {
	if !env.Allowed("ingresses", "list") {
		return
	}
	ings, err := env.Client.Clientset.NetworkingV1().Ingresses(env.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		b.Warnings = append(b.Warnings, fmt.Sprintf("list ingresses: %v", err))
		return
	}
	for _, ing := range ings.Items {
		if (env.target != nil && env.target.hasObject("Ingress", ing.Name)) || (env.target == nil && ingressMatches(ing, env.ScopeNames)) {
			b.Ingresses = append(b.Ingresses, summarizeIngress(ing))
		}
	}
}

func collectEventsStep(ctx context.Context, env *CollectEnv, b *model.EvidenceBundle) {
	scopePods := make([]string, 0, len(b.Pods))
	for _, p := range b.Pods {
		if p.Name != "" {
			scopePods = append(scopePods, p.Name)
		}
	}
	if env.target == nil {
		b.Events, b.EventHistory = env.Collector.collectEvents(ctx, env.Namespace, env.Options.Query, env.DeployNames, scopePods, env.Allowed("events", "list"))
		return
	}
	events, history := env.Collector.collectEvents(ctx, env.Namespace, "", nil, scopePods, env.Allowed("events", "list"))
	for _, e := range events {
		if env.target.eventInTarget(e.InvolvedObject.Kind, e.InvolvedObject.Name) {
			b.Events = append(b.Events, e)
		}
	}
	b.EventHistory = history
}

func collectLogsStep(ctx context.Context, env *CollectEnv, b *model.EvidenceBundle) {
	limit := env.Options.LogLines
	if limit <= 0 {
		limit = maxLogLines
	}
	if env.Allowed("pods/log", "get") {
		env.Collector.collectLogs(ctx, env.Namespace, limit, b)
	}
}

func collectHPAs(ctx context.Context, env *CollectEnv, b *model.EvidenceBundle) {
	if !env.Allowed("horizontalpodautoscalers", "list") {
		return
	}
	hpas, err := env.Client.Clientset.AutoscalingV2().HorizontalPodAutoscalers(env.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		b.Warnings = append(b.Warnings, fmt.Sprintf("list hpa: %v", err))
		return
	}
	for _, h := range hpas.Items {
		ref := h.Spec.ScaleTargetRef
		if (env.target != nil && env.target.hasObject(ref.Kind, ref.Name)) || (env.target == nil && hpaMatches(h, env.ScopeNames)) {
			b.HPAs = append(b.HPAs, summarizeHPA(h))
		}
	}
}

func collectNodesStep(ctx context.Context, env *CollectEnv, b *model.EvidenceBundle) {
	if !env.Allowed("nodes", "list") {
		b.Warnings = append(b.Warnings, "Missing permission: nodes list — node pressure summary unavailable")
		return
	}
	b.Nodes = env.Collector.collectNodes(ctx, b.Pods)
}

func collectNodePods(ctx context.Context, env *CollectEnv, b *model.EvidenceBundle) {
	if env.Allowed("pods", "list") && len(b.Nodes) > 0 {
		b.NodePods = env.Collector.collectPodsOnNodes(ctx, b.Nodes, b.Pods)
	}
}

func collectAdmission(ctx context.Context, env *CollectEnv, b *model.EvidenceBundle) {
	if !env.Options.CheckAdmissionPerimeter {
		return
	}
	b.AdmissionPerimeter = env.Collector.CollectAdmissionPerimeter(ctx, b)
	if b.AdmissionPerimeter != nil && b.AdmissionPerimeter.Status == "denied" {
		b.Warnings = append(b.Warnings, b.AdmissionPerimeter.PermissionNote)
	}
}

func collectMetricsStep(ctx context.Context, env *CollectEnv, b *model.EvidenceBundle) {
	b.Metrics = CollectMetrics(ctx, env.Client, b.Pods)
}

// collectJobs lists Jobs in scope: all of them without a query, otherwise those
// whose name matches the query or that own a pod in scope.
func collectJobs(ctx context.Context, env *CollectEnv, b *model.EvidenceBundle) {
	if !env.Allowed("jobs", "list") {
		return
	}
	jobs, err := env.Client.Clientset.BatchV1().Jobs(env.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		b.Warnings = append(b.Warnings, fmt.Sprintf("list jobs: %v", err))
		return
	}
	owners := map[string]bool{}
	for _, p := range b.Pods {
		for _, o := range p.OwnerRefs {
			if o.Kind == "Job" {
				owners[o.Name] = true
			}
		}
	}
	for _, j := range jobs.Items {
		if env.target != nil && !env.target.hasObject("Job", j.Name) {
			continue
		}
		if env.target == nil && len(env.ScopeNames) > 0 && !owners[j.Name] && !nameInScope(j.Name, env.ScopeNames) {
			continue
		}
		b.Jobs = append(b.Jobs, summarizeJob(j))
	}
}

func nameInScope(name string, scope []string) bool {
	for _, s := range scope {
		if s != "" && strings.Contains(name, s) {
			return true
		}
	}
	return false
}

func summarizeJob(j batchv1.Job) model.JobSummary {
	out := model.JobSummary{
		Name: j.Name, Namespace: j.Namespace, Active: j.Status.Active,
		Succeeded: j.Status.Succeeded, Failed: j.Status.Failed,
		CreatedAt: model.TimestampFrom(j.CreationTimestamp.Time),
	}
	for _, o := range j.OwnerReferences {
		if o.Kind == "CronJob" {
			out.CronJob = o.Name
		}
	}
	for _, c := range j.Status.Conditions {
		if c.Status != corev1.ConditionTrue {
			continue
		}
		switch c.Type {
		case batchv1.JobComplete:
			out.Complete = true
		case batchv1.JobFailed:
			out.FailedReason, out.FailedMessage = c.Reason, c.Message
		}
	}
	return out
}
