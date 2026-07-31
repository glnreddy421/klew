package kube

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/glnreddy421/klew/internal/model"
)

const maxLogLines = 200

// CollectOptions tunes evidence gathering.
type CollectOptions struct {
	Namespace string
	Query     string
	LogLines  int
}

// Collector gathers read-only evidence from a cluster.
type Collector struct {
	Client *Client
}

// Collect builds an EvidenceBundle for the query.
func (col *Collector) Collect(ctx context.Context, opts CollectOptions) (model.EvidenceBundle, error) {
	ns := opts.Namespace
	if ns == "" {
		ns = col.Client.Namespace
	}
	bundle := model.EvidenceBundle{
		CollectedAt: time.Now().UTC(),
		Namespace:   ns,
		Query:       opts.Query,
		KubeContext: model.KubeContext{
			Context:   col.Client.Context,
			Cluster:   col.Client.Cluster,
			User:      col.Client.User,
			Namespace: ns,
		},
	}

	perms, permWarnings := CheckPermissions(ctx, col.Client, ns)
	bundle.Permissions = perms
	bundle.Warnings = append(bundle.Warnings, permWarnings...)

	matches, err := DiscoverMatches(ctx, col.Client, ns, opts.Query)
	if err != nil {
		bundle.Warnings = append(bundle.Warnings, err.Error())
	} else {
		bundle.MatchedObjects = matches
	}

	if crds, err := DetectCRDs(ctx, col.Client); err == nil && len(crds) > 0 {
		bundle.DetectedCRDKinds = crds[:min(20, len(crds))]
	}

	// Resolve primary deployment targets from matches
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

	if allowed(perms, "deployments", "list") {
		for _, dn := range deployNames {
			w, err := col.collectDeployment(ctx, ns, dn)
			if err != nil {
				bundle.Warnings = append(bundle.Warnings, err.Error())
				continue
			}
			bundle.Workloads = append(bundle.Workloads, w)
		}
	}

	var selectors []labels.Selector
	if allowed(perms, "replicasets", "list") {
		rss, err := col.Client.Clientset.AppsV1().ReplicaSets(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("list replicasets: %v", err))
		} else {
			for _, rs := range rss.Items {
				if !matchesWorkload(deployNames, rs.Name, rs.Labels, rs.OwnerReferences) {
					continue
				}
				bundle.ReplicaSets = append(bundle.ReplicaSets, summarizeRS(rs))
				if s, err := metav1.LabelSelectorAsSelector(rs.Spec.Selector); err == nil {
					selectors = append(selectors, s)
				}
			}
		}
	}

	if allowed(perms, "pods", "list") {
		pods, err := col.Client.Clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("list pods: %v", err))
		} else {
			for _, pod := range pods.Items {
				if !podMatchesQuery(pod, deployNames, selectors) {
					continue
				}
				ps := summarizePod(pod)
				bundle.Pods = append(bundle.Pods, ps)
				bundle.ConfigRefs = append(bundle.ConfigRefs, podConfigRefs(pod)...)
				bundle.SecretRefs = append(bundle.SecretRefs, podSecretRefs(pod)...)
				bundle.PVCRefs = append(bundle.PVCRefs, podPVCRefs(pod)...)
			}
		}
	}

	if allowed(perms, "services", "list") {
		bundle.Services = col.collectServices(ctx, ns, selectors, deployNames, perms)
	}

	if allowed(perms, "ingresses", "list") {
		ings, err := col.Client.Clientset.NetworkingV1().Ingresses(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("list ingresses: %v", err))
		} else {
			for _, ing := range ings.Items {
				if !ingressMatches(ing, deployNames) {
					continue
				}
				bundle.Ingresses = append(bundle.Ingresses, summarizeIngress(ing))
			}
		}
	}

	if allowed(perms, "events", "list") {
		bundle.Events = col.collectEvents(ctx, ns, deployNames)
	}

	logLimit := opts.LogLines
	if logLimit <= 0 {
		logLimit = maxLogLines
	}
	if allowed(perms, "pods/log", "get") {
		for _, pod := range bundle.Pods {
			for _, c := range pod.Containers {
				if lr, err := col.fetchLogs(ctx, ns, pod.Name, c.Name, false, logLimit); err == nil {
					bundle.Logs = append(bundle.Logs, lr)
				} else {
					bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("logs %s/%s: %v", pod.Name, c.Name, err))
				}
				if c.RestartCount > 0 || c.LastReason != "" {
					if lr, err := col.fetchLogs(ctx, ns, pod.Name, c.Name, true, logLimit); err == nil {
						bundle.PreviousLogs = append(bundle.PreviousLogs, lr)
					}
				}
			}
		}
	}

	if allowed(perms, "horizontalpodautoscalers", "list") {
		hpas, err := col.Client.Clientset.AutoscalingV2().HorizontalPodAutoscalers(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			bundle.Warnings = append(bundle.Warnings, fmt.Sprintf("list hpa: %v", err))
		} else {
			for _, h := range hpas.Items {
				if !hpaMatches(h, deployNames) {
					continue
				}
				bundle.HPAs = append(bundle.HPAs, summarizeHPA(h))
			}
		}
	}

	if allowed(perms, "nodes", "list") {
		bundle.Nodes = col.collectNodes(ctx, bundle.Pods)
	} else {
		bundle.Warnings = append(bundle.Warnings, "Missing permission: nodes list — node pressure summary unavailable")
	}

	if allowed(perms, "pods", "list") && len(bundle.Nodes) > 0 {
		bundle.NodePods = col.collectPodsOnNodes(ctx, bundle.Nodes, bundle.Pods)
	}

	return bundle, nil
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
	return model.ReplicaSetSummary{
		Name:            rs.Name,
		Namespace:       rs.Namespace,
		Replicas:        deref32(rs.Spec.Replicas),
		Ready:           rs.Status.ReadyReplicas,
		DeploymentOwner: owner,
		CreatedAt:       rs.CreationTimestamp.Time,
	}
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
		CreatedAt:   pod.CreationTimestamp.Time,
	}
	for _, o := range pod.OwnerReferences {
		ps.OwnerRefs = append(ps.OwnerRefs, model.ObjectRef{Kind: o.Kind, Name: o.Name, UID: string(o.UID)})
	}
	ready := true
	var restarts int32
	for _, cs := range pod.Status.ContainerStatuses {
		if !cs.Ready {
			ready = false
		}
		restarts += cs.RestartCount
		ps.Containers = append(ps.Containers, containerFromStatus(pod.Name, cs, pod))
	}
	ps.Ready = ready
	ps.RestartCount = restarts
	return ps
}

func containerFromStatus(podName string, cs corev1.ContainerStatus, pod corev1.Pod) model.ContainerStatus {
	out := model.ContainerStatus{
		PodName:      podName,
		Name:         cs.Name,
		Image:        cs.Image,
		Ready:        cs.Ready,
		RestartCount: cs.RestartCount,
	}
	for _, c := range pod.Spec.Containers {
		if c.Name == cs.Name {
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
			break
		}
	}
	if cs.State.Running != nil {
		out.State = "running"
		t := cs.State.Running.StartedAt.Time
		out.StartedAt = &t
	} else if cs.State.Waiting != nil {
		out.State = "waiting"
		out.Reason = cs.State.Waiting.Reason
	} else if cs.State.Terminated != nil {
		out.State = "terminated"
		out.Reason = cs.State.Terminated.Reason
		out.ExitCode = cs.State.Terminated.ExitCode
		t := cs.State.Terminated.FinishedAt.Time
		out.FinishedAt = &t
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

// countReadyEndpointSliceAddresses mirrors legacy Endpoints subset.Addresses counting:
// only addresses on endpoints that are ready (nil Ready is treated as ready).
func countReadyEndpointSliceAddresses(slices []discoveryv1.EndpointSlice) (ready, total int) {
	for _, sl := range slices {
		for _, ep := range sl.Endpoints {
			if ep.Conditions.Ready != nil && !*ep.Conditions.Ready {
				continue
			}
			n := len(ep.Addresses)
			ready += n
			total += n
		}
	}
	return ready, total
}

func summarizeIngress(ing networkingv1.Ingress) model.IngressSummary {
	s := model.IngressSummary{Name: ing.Name, Namespace: ing.Namespace}
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
	for _, c := range pod.Spec.Containers {
		for _, e := range c.EnvFrom {
			if e.ConfigMapRef != nil {
				refs = append(refs, model.ResourceRef{Kind: "ConfigMap", Name: e.ConfigMapRef.Name, Namespace: pod.Namespace, UsedBy: pod.Name})
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

func matchesWorkload(deployNames []string, rsName string, labels map[string]string, owners []metav1.OwnerReference) bool {
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
