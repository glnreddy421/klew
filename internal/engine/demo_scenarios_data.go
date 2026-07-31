package engine

import (
	"fmt"
	"time"

	"github.com/glnreddy421/klew/internal/investigation"
	"github.com/glnreddy421/klew/internal/model"
)

// ── payment: memory regression / OOMKilled after rollout ─────────────────────

func paymentScenario() DemoScenario {
	ns := "prod"
	return DemoScenario{
		Query:      "payment",
		Namespace:  ns,
		Context:    "prod-eks",
		Conclusion: "Memory regression after rollout",
		Targets: []DemoTarget{
			{Name: "payment-api", Kind: "Deployment", Pods: 4, Extra: "Service: payment-api"},
			{Name: "payment-worker", Kind: "Deployment", Pods: 2, Extra: "queue consumer"},
			{Name: "payment-admin", Kind: "Deployment", Pods: 3, Extra: "internal UI"},
			{Name: "payment-batch", Kind: "CronJob", Pods: 0, Extra: "nightly settlement"},
		},
		base: func(target string) model.EvidenceBundle {
			img := "repo/payment-api:v2.3.2"
			pods := []model.PodSummary{
				demoPod(ns, target+"-a1", "ip-10-0-2-11", true, 0, img),
				demoPod(ns, target+"-a2", "ip-10-0-2-11", true, 0, img),
				demoPod(ns, target+"-a3", "ip-10-0-2-12", true, 0, img),
				demoPod(ns, target+"-a4", "ip-10-0-2-12", true, 0, img),
			}
			return model.EvidenceBundle{
				CollectedAt: time.Now(),
				Workloads: []model.WorkloadSummary{{Kind: "Deployment", Name: target, Namespace: ns,
					Replicas: 4, Ready: 4, Available: 4, Updated: 4, Generation: 42, Selector: "app=" + target}},
				ReplicaSets: []model.ReplicaSetSummary{
					{Name: target + "-r41", Namespace: ns, Replicas: 4, Ready: 4, DeploymentOwner: target},
				},
				Pods: pods,
				Services: []model.ServiceSummary{{Name: target, Namespace: ns, Type: "ClusterIP",
					Selector: "app=" + target, ReadyEndpoints: 4, TotalEndpoints: 4, Ports: []string{"80/TCP"}}},
				Ingresses:  []model.IngressSummary{{Name: target, Namespace: ns, Hosts: []string{"pay.example.com"}, Backends: []string{target}}},
				Nodes:      []model.NodeSummary{{Name: "ip-10-0-2-11", Ready: true}, {Name: "ip-10-0-2-12", Ready: true}},
				ConfigRefs: []model.ResourceRef{{Kind: "ConfigMap", Name: target + "-config", Namespace: ns}},
				SecretRefs: []model.ResourceRef{{Kind: "Secret", Name: target + "-secret", Namespace: ns}},
				Metrics:    model.MetricsSummary{Available: true, MemRequestMi: 512, MemLimitMi: 512, MemUsageMi: 780, CPURequestM: 1200, CPULimitM: 2000, CPUUsageM: 300},
			}
		},
		steps: func(target string) []demoStep {
			p := func(i int) string { return target + []string{"-a1", "-a2", "-a3", "-a4"}[i] }
			return []demoStep{
				{at: 0, emit: []model.EvidenceEvent{
					evSys(ns, "Investigation started: "+target),
					evObj(ns, "Deployment", target, "RevisionChanged", "rollout revision 41 → 42"),
				}, mutate: func(b *model.EvidenceBundle) {
					b.ReplicaSets = append(b.ReplicaSets, model.ReplicaSetSummary{Name: target + "-r42", Namespace: ns, Replicas: 4, Ready: 0, DeploymentOwner: target})
				}},
				{at: 2 * time.Second, emit: []model.EvidenceEvent{
					evObj(ns, "ReplicaSet", target+"-r42", "Created", "new ReplicaSet created for revision 42"),
					evEvent(ns, "Pod", p(0), model.SeverityInfo, "Scheduled", "assigned to ip-10-0-2-11"),
					evEvent(ns, "Pod", p(0), model.SeverityInfo, "Created", "created container app"),
					evEvent(ns, "Pod", p(0), model.SeverityInfo, "Started", "started container app"),
					evLog(ns, p(0), "app", model.SeverityInfo, "", "INFO Starting application"),
					evLog(ns, p(0), "app", model.SeverityInfo, "", "INFO Connected Redis"),
				}},
				{at: 5 * time.Second, emit: []model.EvidenceEvent{
					evMetric(ns, target, "memory 780Mi · cpu 300m"),
				}},
				{at: 8 * time.Second, emit: []model.EvidenceEvent{
					evEvent(ns, "Pod", p(0), model.SeverityWarning, "Readiness failed", "readiness probe failed: connection refused"),
					evEvent(ns, "Pod", p(1), model.SeverityWarning, "Readiness failed", "readiness probe failed: connection refused"),
				}, mutate: func(b *model.EvidenceBundle) { b.Pods[0].Ready = false; b.Pods[1].Ready = false }},
				{at: 12 * time.Second, emit: repeat(5, evLog(ns, target+"-a1", "app", model.SeverityHigh, "Redis timeout", "WARN Redis timeout after 5000ms"))},
				{at: 16 * time.Second, emit: append(repeat(3, evLog(ns, target+"-a2", "app", model.SeverityHigh, "Redis timeout", "WARN Redis timeout after 5000ms")),
					evEvent(ns, "Pod", p(2), model.SeverityWarning, "Readiness failed", "readiness probe failed: connection refused"))},
				{at: 18 * time.Second, emit: []model.EvidenceEvent{
					evLog(ns, target+"-a1", "app", model.SeverityHigh, "Memory allocation", "ERROR Memory allocation failed"),
					evLog(ns, target+"-a1", "app", model.SeverityHigh, "Memory allocation", "ERROR allocating 64MB"),
				}},
				{at: 22 * time.Second, emit: []model.EvidenceEvent{
					evEvent(ns, "Pod", p(0), model.SeverityCritical, "OOMKilled", "container app exceeded memory limit (exit 137)"),
					evEvent(ns, "Pod", p(1), model.SeverityCritical, "OOMKilled", "container app exceeded memory limit (exit 137)"),
					evObj(ns, "Pod", p(0), "Restarted", "container app restarted"),
				}, mutate: func(b *model.EvidenceBundle) {
					b.Pods[0].RestartCount = 4
					b.Pods[1].RestartCount = 3
					b.Pods[0].Containers = []model.ContainerStatus{{Name: "app", Image: b.Pods[0].Containers[0].Image, Ready: false, RestartCount: 4, State: "running", LastState: "terminated", LastReason: "OOMKilled", LastExitCode: 137, RequestsMem: "512Mi", LimitsMem: "512Mi"}}
				}},
				{at: 26 * time.Second, emit: []model.EvidenceEvent{
					evMetric(ns, target, "memory 2.4Gi · cpu 1200m"),
				}, mutate: func(b *model.EvidenceBundle) {
					b.Pods[2].RestartCount = 2
					b.Metrics.MemUsageMi = 2457
					b.Metrics.CPUUsageM = 1200
				}},
				{at: 30 * time.Second, emit: []model.EvidenceEvent{
					evEvent(ns, "Service", target, model.SeverityHigh, "EndpointsDropped", "endpoints dropped from 4 to 1"),
					evObj(ns, "Service", target, "EndpointsChanged", "endpoints 4 → 1"),
				}, mutate: func(b *model.EvidenceBundle) {
					b.Pods[2].Ready = false
					b.Services[0].ReadyEndpoints = 1
				}},
				{at: 34 * time.Second, emit: []model.EvidenceEvent{
					evLog(ns, target+"-a1", "app", model.SeverityCritical, "Panic", "panic: runtime out of memory"),
				}},
			}
		},
		tail: func(target string) []model.EvidenceEvent {
			return []model.EvidenceEvent{
				evEvent(ns, "Pod", target+"-a1", model.SeverityCritical, "OOMKilled", "container app exceeded memory limit (exit 137)"),
				evLog(ns, target+"-a2", "app", model.SeverityHigh, "Memory allocation", "ERROR Memory allocation failed"),
				evLog(ns, target+"-a3", "app", model.SeverityHigh, "Redis timeout", "WARN Redis timeout after 5000ms"),
				evMetric(ns, target, "memory 2.4Gi · cpu 1200m"),
			}
		},
		dataset: paymentDataset,
		recent: &model.RecentChange{
			RevisionFrom: "41", RevisionTo: "42", Image: "payment-api:v2.8.1",
			HelmRelease: "payment-stack", HelmRevision: "17"},
	}
}

// ── vault: controller configuration / mount failure ──────────────────────────

func vaultScenario() DemoScenario {
	ns := "vault"
	return DemoScenario{
		Query:      "vault",
		Namespace:  ns,
		Context:    "prod-eks",
		Conclusion: "Vault controller configuration issue",
		Targets: []DemoTarget{
			{Name: "vault-controller", Kind: "Deployment", Pods: 1, Extra: "dynamic secrets"},
			{Name: "vault-agent", Kind: "DaemonSet", Pods: 6, Extra: "sidecar injector"},
			{Name: "vault-webhook", Kind: "Deployment", Pods: 2, Extra: "admission webhook"},
		},
		base: func(target string) model.EvidenceBundle {
			img := "hashicorp/vault-k8s:1.3.1"
			pods := []model.PodSummary{demoPod(ns, target+"-0", "ip-10-0-3-9", true, 0, img)}
			return model.EvidenceBundle{
				CollectedAt: time.Now(),
				Workloads:   []model.WorkloadSummary{{Kind: "Deployment", Name: target, Namespace: ns, Replicas: 1, Ready: 1, Available: 1, Generation: 12, Selector: "app=" + target}},
				ReplicaSets: []model.ReplicaSetSummary{{Name: target + "-r12", Namespace: ns, Replicas: 1, Ready: 1, DeploymentOwner: target}},
				Pods:        pods,
				Services:    []model.ServiceSummary{{Name: target, Namespace: ns, Type: "ClusterIP", Selector: "app=" + target, ReadyEndpoints: 1, TotalEndpoints: 1, Ports: []string{"8200/TCP"}}},
				Nodes:       []model.NodeSummary{{Name: "ip-10-0-3-9", Ready: true}},
				ConfigRefs:  []model.ResourceRef{{Kind: "ConfigMap", Name: target + "-config", Namespace: ns}},
				SecretRefs:  []model.ResourceRef{{Kind: "Secret", Name: "vault-dynamic-creds", Namespace: ns}},
				Metrics:     model.MetricsSummary{Available: true, MemRequestMi: 256, MemLimitMi: 256, MemUsageMi: 120, CPURequestM: 200, CPULimitM: 500, CPUUsageM: 90},
			}
		},
		steps: func(target string) []demoStep {
			return []demoStep{
				{at: 0, emit: []model.EvidenceEvent{
					evSys(ns, "Investigation started: "+target),
					evObj(ns, "Deployment", target, "RevisionChanged", "rollout revision 11 → 12"),
				}},
				{at: 3 * time.Second, emit: []model.EvidenceEvent{
					evEvent(ns, "Pod", target+"-0", model.SeverityInfo, "Scheduled", "assigned to ip-10-0-3-9"),
					evEvent(ns, "Pod", target+"-0", model.SeverityInfo, "Started", "started container vault-k8s"),
					evLog(ns, target+"-0", "vault-k8s", model.SeverityInfo, "", "INFO starting controller"),
				}},
				{at: 6 * time.Second, emit: repeat(2, evEvent(ns, "Pod", target+"-0", model.SeverityHigh, "FailedMount", "MountVolume.SetUp failed for volume 'creds': secret vault-dynamic-creds not found"))},
				{at: 9 * time.Second, emit: repeat(2, evLog(ns, target+"-0", "vault-k8s", model.SeverityHigh, "Secret error", "ERROR permission denied reading secret/data/app"))},
				{at: 12 * time.Second, emit: repeat(2, evLog(ns, target+"-0", "vault-k8s", model.SeverityHigh, "Role missing", "ERROR vault: dynamic role 'app-role' missing"))},
				{at: 15 * time.Second, emit: []model.EvidenceEvent{
					evEvent(ns, "Pod", target+"-0", model.SeverityHigh, "BackOff", "back-off restarting failed container"),
					evObj(ns, "Pod", target+"-0", "Restarted", "controller container restarted"),
				}, mutate: func(b *model.EvidenceBundle) { b.Pods[0].Ready = false; b.Pods[0].RestartCount = 3 }},
				{at: 18 * time.Second, emit: append(repeat(2, evEvent(ns, "Pod", target+"-0", model.SeverityHigh, "FailedMount", "MountVolume.SetUp failed for volume 'creds': secret vault-dynamic-creds not found")),
					evLog(ns, target+"-0", "vault-k8s", model.SeverityHigh, "Role missing", "ERROR vault: dynamic role 'app-role' missing"))},
			}
		},
		tail: func(target string) []model.EvidenceEvent {
			return []model.EvidenceEvent{
				evEvent(ns, "Pod", target+"-0", model.SeverityHigh, "FailedMount", "MountVolume.SetUp failed for volume 'creds': secret not found"),
				evLog(ns, target+"-0", "vault-k8s", model.SeverityHigh, "Secret error", "ERROR permission denied reading secret/data/app"),
			}
		},
		dataset: vaultDataset,
		recent: &model.RecentChange{
			RevisionFrom: "11", RevisionTo: "12", Image: "vault-k8s:1.3.1"},
	}
}

// ── checkout: registry / image pull failure ──────────────────────────────────

func checkoutScenario() DemoScenario {
	ns := "prod"
	return DemoScenario{
		Query:      "checkout",
		Namespace:  ns,
		Context:    "prod-eks",
		Conclusion: "Registry failure",
		Targets: []DemoTarget{
			{Name: "checkout-api", Kind: "Deployment", Pods: 3, Extra: "Service: checkout-api"},
			{Name: "checkout-worker", Kind: "Deployment", Pods: 2, Extra: "order processor"},
		},
		base: func(target string) model.EvidenceBundle {
			img := "registry.internal/checkout:v9.0.0"
			pods := []model.PodSummary{
				demoPod(ns, target+"-a1", "ip-10-0-2-11", false, 0, img),
				demoPod(ns, target+"-a2", "ip-10-0-2-12", false, 0, img),
			}
			pods[0].Phase, pods[1].Phase = "Pending", "Pending"
			return model.EvidenceBundle{
				CollectedAt: time.Now(),
				Workloads:   []model.WorkloadSummary{{Kind: "Deployment", Name: target, Namespace: ns, Replicas: 3, Ready: 0, Available: 0, Generation: 9, Selector: "app=" + target}},
				ReplicaSets: []model.ReplicaSetSummary{{Name: target + "-r9", Namespace: ns, Replicas: 3, Ready: 0, DeploymentOwner: target}},
				Pods:        pods,
				Services:    []model.ServiceSummary{{Name: target, Namespace: ns, Type: "ClusterIP", Selector: "app=" + target, ReadyEndpoints: 0, TotalEndpoints: 3, Ports: []string{"80/TCP"}}},
				Nodes:       []model.NodeSummary{{Name: "ip-10-0-2-11", Ready: true}, {Name: "ip-10-0-2-12", Ready: true}},
				ConfigRefs:  []model.ResourceRef{{Kind: "ConfigMap", Name: target + "-config", Namespace: ns}},
				Metrics:     model.MetricsSummary{Available: true, MemRequestMi: 256, MemLimitMi: 512, MemUsageMi: 0, CPURequestM: 200, CPULimitM: 500, CPUUsageM: 0},
			}
		},
		steps: func(target string) []demoStep {
			return []demoStep{
				{at: 0, emit: []model.EvidenceEvent{
					evSys(ns, "Investigation started: "+target),
					evObj(ns, "Deployment", target, "RevisionChanged", "rollout revision 8 → 9"),
				}},
				{at: 3 * time.Second, emit: []model.EvidenceEvent{
					evEvent(ns, "Pod", target+"-a1", model.SeverityInfo, "Scheduled", "assigned to ip-10-0-2-11"),
				}},
				{at: 5 * time.Second, emit: []model.EvidenceEvent{
					evEvent(ns, "Pod", target+"-a1", model.SeverityInfo, "Pulling", "pulling image registry.internal/checkout:v9.0.0"),
				}},
				{at: 7 * time.Second, emit: repeat(2, evEvent(ns, "Pod", target+"-a1", model.SeverityHigh, "ErrImagePull", "Failed to pull image: registry timeout"))},
				{at: 10 * time.Second, emit: repeat(3, evEvent(ns, "Pod", target+"-a1", model.SeverityHigh, "ImagePullBackOff", "Back-off pulling image registry.internal/checkout:v9.0.0"))},
				{at: 13 * time.Second, emit: repeat(2, evLog(ns, target+"-a1", "app", model.SeverityHigh, "Registry timeout", "ERROR timeout contacting registry.internal"))},
				{at: 16 * time.Second, emit: repeat(3, evEvent(ns, "Pod", target+"-a2", model.SeverityHigh, "ImagePullBackOff", "Back-off pulling image registry.internal/checkout:v9.0.0"))},
			}
		},
		tail: func(target string) []model.EvidenceEvent {
			return []model.EvidenceEvent{
				evEvent(ns, "Pod", target+"-a1", model.SeverityHigh, "ImagePullBackOff", "Back-off pulling image"),
				evLog(ns, target+"-a1", "app", model.SeverityHigh, "Registry timeout", "ERROR timeout contacting registry.internal"),
			}
		},
		dataset: checkoutDataset,
		recent: &model.RecentChange{
			RevisionFrom: "8", RevisionTo: "9", Image: "checkout:v9.0.0",
			GitSHA: "a1b9f3c", SyncState: "OutOfSync"},
	}
}

func repeat(n int, e model.EvidenceEvent) []model.EvidenceEvent {
	out := make([]model.EvidenceEvent, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, e)
	}
	return out
}

// ── demo discovery datasets (searched + grouped + scope-expanded) ────────────

func ref(ns, kind, name string) investigation.Ref {
	return investigation.Ref{Kind: kind, Name: name, Namespace: ns}
}

func refp(r investigation.Ref) *investigation.Ref { return &r }

// demoWorkload builds a Deployment (or other controller) with its ReplicaSet,
// pods and (optionally) a matching Service.
func demoWorkload(ns, app, kind string, pods int, withService bool) []investigation.Object {
	lbl := map[string]string{"app": app}
	dep := investigation.Object{Ref: ref(ns, kind, app), Labels: lbl, Selector: lbl, ServiceAccount: app}
	objs := []investigation.Object{dep}

	podOwner := dep.Ref
	if kind == "Deployment" {
		rs := investigation.Object{Ref: ref(ns, "ReplicaSet", app+"-rs"), Labels: lbl, Owner: refp(dep.Ref)}
		objs = append(objs, rs)
		podOwner = rs.Ref
	}
	for i := 1; i <= pods; i++ {
		objs = append(objs, investigation.Object{
			Ref: ref(ns, "Pod", fmt.Sprintf("%s-a%d", app, i)), Labels: lbl, Owner: refp(podOwner),
			Node: fmt.Sprintf("ip-10-0-2-%d", 11+i%2), ServiceAccount: app})
	}
	if withService {
		objs = append(objs, investigation.Object{Ref: ref(ns, "Service", app), Selector: lbl})
	}
	return objs
}

func paymentDataset() investigation.Dataset {
	ns := "prod"
	lbl := map[string]string{"app": "payment-api"}
	var objs []investigation.Object

	// payment-api — the fully-referenced workload
	objs = append(objs,
		investigation.Object{Ref: ref(ns, "Deployment", "payment-api"), Labels: lbl, Selector: lbl,
			ConfigMaps: []string{"payment-api-config", "payment-api-features"},
			Secrets:    []string{"payment-api-secret"}, PVCs: []string{"payment-api-data"}, ServiceAccount: "payment-api"},
		investigation.Object{Ref: ref(ns, "ReplicaSet", "payment-api-r42"), Labels: lbl, Owner: refp(ref(ns, "Deployment", "payment-api"))},
	)
	for i := 1; i <= 4; i++ {
		objs = append(objs, investigation.Object{Ref: ref(ns, "Pod", fmt.Sprintf("payment-api-a%d", i)), Labels: lbl,
			Owner: refp(ref(ns, "ReplicaSet", "payment-api-r42")), Node: fmt.Sprintf("ip-10-0-2-%d", 11+i%2), ServiceAccount: "payment-api"})
	}
	objs = append(objs,
		investigation.Object{Ref: ref(ns, "Service", "payment-api"), Selector: lbl},
		investigation.Object{Ref: ref(ns, "EndpointSlice", "payment-api-abcde"), Owner: refp(ref(ns, "Service", "payment-api"))},
		investigation.Object{Ref: ref(ns, "Ingress", "payment"), Target: refp(ref(ns, "Service", "payment-api"))},
		investigation.Object{Ref: ref(ns, "HorizontalPodAutoscaler", "payment-api"), Target: refp(ref(ns, "Deployment", "payment-api"))},
		investigation.Object{Ref: ref(ns, "PodDisruptionBudget", "payment-api"), Selector: lbl},
		investigation.Object{Ref: ref(ns, "NetworkPolicy", "payment-api"), Selector: lbl},
		investigation.Object{Ref: ref(ns, "ServiceAccount", "payment-api")},
		investigation.Object{Ref: ref(ns, "Role", "payment-api-role")},
		investigation.Object{Ref: ref(ns, "RoleBinding", "payment-api"), Target: refp(ref(ns, "ServiceAccount", "payment-api")), Role: refp(ref(ns, "Role", "payment-api-role"))},
		investigation.Object{Ref: ref(ns, "ConfigMap", "payment-api-config")},
		investigation.Object{Ref: ref(ns, "ConfigMap", "payment-api-features")},
		investigation.Object{Ref: ref(ns, "Secret", "payment-api-secret")},
		investigation.Object{Ref: ref(ns, "PersistentVolumeClaim", "payment-api-data")},
		investigation.Object{Ref: ref(ns, "Event", "payment-api-a1.oom"), Involved: refp(ref(ns, "Pod", "payment-api-a1"))},
		investigation.Object{Ref: ref(ns, "Event", "payment-api.endpoints"), Involved: refp(ref(ns, "Service", "payment-api"))},
	)

	// unrelated payment workloads discovered by the same query
	objs = append(objs, demoWorkload(ns, "payment-worker", "Deployment", 2, true)...)
	objs = append(objs, demoWorkload(ns, "payment-admin", "Deployment", 3, false)...)
	objs = append(objs,
		investigation.Object{Ref: ref(ns, "CronJob", "payment-batch"), Labels: map[string]string{"app": "payment-batch"}},
		investigation.Object{Ref: ref(ns, "Job", "payment-batch-27500"), Labels: map[string]string{"app": "payment-batch"}, Owner: refp(ref(ns, "CronJob", "payment-batch"))},
		investigation.Object{Ref: ref(ns, "Pod", "payment-batch-27500-x"), Labels: map[string]string{"app": "payment-batch"}, Owner: refp(ref(ns, "Job", "payment-batch-27500")), Node: "ip-10-0-2-12"},
	)

	return investigation.Dataset{Namespace: ns, Objects: objs, Extensions: []investigation.ExtensionData{
		{Name: "Istio", CRDs: []investigation.Ref{ref(ns, "VirtualService", "payment-api"), ref(ns, "DestinationRule", "payment-api")}},
		{Name: "cert-manager", CRDs: []investigation.Ref{ref(ns, "Certificate", "payment-api-tls")}},
		{Name: "External Secrets", CRDs: []investigation.Ref{ref(ns, "ExternalSecret", "payment-api-secret")}},
	}}
}

func vaultDataset() investigation.Dataset {
	ns := "vault"
	lbl := map[string]string{"app": "vault-controller"}
	var objs []investigation.Object
	objs = append(objs,
		investigation.Object{Ref: ref(ns, "Deployment", "vault-controller"), Labels: lbl, Selector: lbl,
			ConfigMaps: []string{"vault-controller-config"}, Secrets: []string{"vault-dynamic-creds"}, ServiceAccount: "vault-controller"},
		investigation.Object{Ref: ref(ns, "ReplicaSet", "vault-controller-rs"), Labels: lbl, Owner: refp(ref(ns, "Deployment", "vault-controller"))},
		investigation.Object{Ref: ref(ns, "Pod", "vault-controller-a1"), Labels: lbl, Owner: refp(ref(ns, "ReplicaSet", "vault-controller-rs")), Node: "ip-10-0-3-9", ServiceAccount: "vault-controller"},
		investigation.Object{Ref: ref(ns, "Service", "vault-controller"), Selector: lbl},
		investigation.Object{Ref: ref(ns, "EndpointSlice", "vault-controller-slice"), Owner: refp(ref(ns, "Service", "vault-controller"))},
		investigation.Object{Ref: ref(ns, "ServiceAccount", "vault-controller")},
		investigation.Object{Ref: ref(ns, "Role", "vault-controller")},
		investigation.Object{Ref: ref(ns, "RoleBinding", "vault-controller"), Target: refp(ref(ns, "ServiceAccount", "vault-controller")), Role: refp(ref(ns, "Role", "vault-controller"))},
		investigation.Object{Ref: ref(ns, "ConfigMap", "vault-controller-config")},
		investigation.Object{Ref: ref(ns, "Secret", "vault-dynamic-creds")},
		investigation.Object{Ref: ref(ns, "Event", "vault-controller.mount"), Involved: refp(ref(ns, "Pod", "vault-controller-a1"))},
	)
	objs = append(objs, demoWorkload(ns, "vault-agent", "DaemonSet", 3, false)...)
	objs = append(objs, demoWorkload(ns, "vault-webhook", "Deployment", 2, true)...)

	return investigation.Dataset{Namespace: ns, Objects: objs, Extensions: []investigation.ExtensionData{
		{Name: "cert-manager", CRDs: []investigation.Ref{ref(ns, "Certificate", "vault-controller-tls"), ref(ns, "Issuer", "vault-controller-issuer")}},
		{Name: "External Secrets", CRDs: []investigation.Ref{ref(ns, "SecretStore", "vault-controller-store")}},
	}}
}

func checkoutDataset() investigation.Dataset {
	ns := "prod"
	lbl := map[string]string{"app": "checkout-api"}
	var objs []investigation.Object
	objs = append(objs,
		investigation.Object{Ref: ref(ns, "Deployment", "checkout-api"), Labels: lbl, Selector: lbl,
			ConfigMaps: []string{"checkout-api-config"}, ServiceAccount: "checkout-api"},
		investigation.Object{Ref: ref(ns, "ReplicaSet", "checkout-api-r9"), Labels: lbl, Owner: refp(ref(ns, "Deployment", "checkout-api"))},
	)
	for i := 1; i <= 3; i++ {
		objs = append(objs, investigation.Object{Ref: ref(ns, "Pod", fmt.Sprintf("checkout-api-a%d", i)), Labels: lbl,
			Owner: refp(ref(ns, "ReplicaSet", "checkout-api-r9")), Node: "ip-10-0-2-11", ServiceAccount: "checkout-api"})
	}
	objs = append(objs,
		investigation.Object{Ref: ref(ns, "Service", "checkout-api"), Selector: lbl},
		investigation.Object{Ref: ref(ns, "EndpointSlice", "checkout-api-slice"), Owner: refp(ref(ns, "Service", "checkout-api"))},
		investigation.Object{Ref: ref(ns, "Ingress", "checkout"), Target: refp(ref(ns, "Service", "checkout-api"))},
		investigation.Object{Ref: ref(ns, "ConfigMap", "checkout-api-config")},
		investigation.Object{Ref: ref(ns, "Event", "checkout-api.pull"), Involved: refp(ref(ns, "Pod", "checkout-api-a1"))},
	)
	objs = append(objs, demoWorkload(ns, "checkout-worker", "Deployment", 2, false)...)

	return investigation.Dataset{Namespace: ns, Objects: objs, Extensions: []investigation.ExtensionData{
		{Name: "Istio", CRDs: []investigation.Ref{ref(ns, "VirtualService", "checkout-api")}},
	}}
}
