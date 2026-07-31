package engine

import (
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

// DemoState returns a rich, hand-crafted InvestigationState so the TUI can be
// explored without a live cluster. It reproduces a payment-api OOM incident.
func DemoState() model.InvestigationState {
	now := time.Now()
	at := func(hh, mm, ss int) time.Time {
		return time.Date(now.Year(), now.Month(), now.Day(), hh, mm, ss, 0, time.Local)
	}
	tp := func(t time.Time) *time.Time { return &t }

	labels := map[string]string{"app": "payment-api"}
	oomContainer := func(image string, restarts int32, started, finished time.Time) model.ContainerStatus {
		return model.ContainerStatus{
			Name: "app", Image: image, Ready: false, RestartCount: restarts,
			State: "running", LastState: "terminated", LastReason: "OOMKilled", LastExitCode: 137,
			StartedAt: tp(started), FinishedAt: tp(finished),
			RequestsCPU: "300m", RequestsMem: "512Mi", LimitsCPU: "500m", LimitsMem: "512Mi",
		}
	}
	healthyContainer := func(image string) model.ContainerStatus {
		return model.ContainerStatus{
			Name: "app", Image: image, Ready: true, RestartCount: 0, State: "running",
			RequestsCPU: "300m", RequestsMem: "512Mi", LimitsCPU: "500m", LimitsMem: "512Mi",
		}
	}

	pods := []model.PodSummary{
		{Name: "payment-77c9-a1", Namespace: "payments", Node: "ip-10-0-4-12", Phase: "Running", Ready: false, RestartCount: 4, Labels: labels,
			Containers: []model.ContainerStatus{oomContainer("repo/payment-api:v2.1.4", 4, at(9, 39, 2), at(9, 40, 22))}},
		{Name: "payment-77c9-b2", Namespace: "payments", Node: "ip-10-0-4-13", Phase: "Running", Ready: false, RestartCount: 5, Labels: labels,
			Containers: []model.ContainerStatus{oomContainer("repo/payment-api:v2.1.4", 5, at(9, 39, 8), at(9, 41, 2))}},
		{Name: "payment-77c9-c3", Namespace: "payments", Node: "ip-10-0-4-12", Phase: "Running", Ready: false, RestartCount: 2, Labels: labels,
			Containers: []model.ContainerStatus{oomContainer("repo/payment-api:v2.1.4", 2, at(9, 39, 15), at(9, 40, 55))}},
		{Name: "payment-52aa-x1", Namespace: "payments", Node: "ip-10-0-4-12", Phase: "Running", Ready: true, RestartCount: 0, Labels: labels,
			Containers: []model.ContainerStatus{healthyContainer("repo/payment-api:v2.1.3")}},
		{Name: "payment-52aa-x2", Namespace: "payments", Node: "ip-10-0-4-13", Phase: "Running", Ready: true, RestartCount: 0, Labels: labels,
			Containers: []model.ContainerStatus{healthyContainer("repo/payment-api:v2.1.3")}},
	}

	bundle := model.EvidenceBundle{
		CollectedAt: at(9, 38, 0),
		Namespace:   "payments",
		Query:       "payment-api",
		KubeContext: model.KubeContext{Context: "prod-eks", Cluster: "prod-eks", User: "sre@corp", Namespace: "payments"},
		Workloads: []model.WorkloadSummary{{
			Kind: "Deployment", Name: "payment-api", Namespace: "payments",
			Replicas: 5, Ready: 2, Available: 2, Updated: 3, Generation: 42, ObservedGen: 42,
			Selector: "app=payment-api",
		}},
		ReplicaSets: []model.ReplicaSetSummary{
			{Name: "payment-api-77c9", Namespace: "payments", Replicas: 3, Ready: 0, DeploymentOwner: "payment-api", CreatedAt: at(9, 38, 31)},
			{Name: "payment-api-52aa", Namespace: "payments", Replicas: 2, Ready: 2, DeploymentOwner: "payment-api", CreatedAt: at(9, 10, 0)},
		},
		Pods: pods,
		Services: []model.ServiceSummary{{
			Name: "payment-api", Namespace: "payments", Type: "ClusterIP", ClusterIP: "10.100.4.20",
			Selector: "app=payment-api", ReadyEndpoints: 2, TotalEndpoints: 5, Ports: []string{"80/TCP"},
		}},
		Ingresses: []model.IngressSummary{{Name: "payment-ingress", Namespace: "payments", Hosts: []string{"pay.example.com"}, Backends: []string{"payment-api"}}},
		HPAs: []model.HPASummary{{
			Name: "payment-api", Namespace: "payments", TargetKind: "Deployment", TargetName: "payment-api",
			MinReplicas: 2, MaxReplicas: 5, CurrentReplicas: 5, DesiredReplicas: 5, AtMax: true,
		}},
		Nodes: []model.NodeSummary{
			{Name: "ip-10-0-4-12", Ready: true},
			{Name: "ip-10-0-4-13", Ready: true},
		},
		ConfigRefs: []model.ResourceRef{{Kind: "ConfigMap", Name: "payment-config", Namespace: "payments", UsedBy: "payment-api"}},
		SecretRefs: []model.ResourceRef{{Kind: "Secret", Name: "payment-secret", Namespace: "payments", UsedBy: "payment-api"}},
		Events:     demoEvents(at),
		Metrics: model.MetricsSummary{
			Available: true, CPURequestM: 1500, CPULimitM: 2500, CPUUsageM: 900,
			MemRequestMi: 2560, MemLimitMi: 2560, MemUsageMi: 2410,
		},
		Permissions: demoPermissions(),
		Warnings:    []string{"nodes get: permission denied — node pressure details limited"},
	}

	st := model.NewInvestigationState("payment-api", model.ModeLive)
	st.CollectedAt = at(9, 38, 0)
	st.LastUpdatedAt = at(9, 42, 11)
	st.KubeContext = bundle.KubeContext
	st.NamespaceScope = model.NamespaceScope{Primary: "payments"}
	st.Snapshot = bundle
	st.MatchedObjects = []model.MatchedObject{{Ref: model.ObjectRef{Kind: "Deployment", Name: "payment-api", Namespace: "payments"}, MatchBy: "name", Score: 1}}
	st.Permissions = bundle.Permissions
	st.Warnings = bundle.Warnings
	st.Timeline = demoTimeline(at)
	st.LiveEvidence = demoStream(at)
	st.WorkloadGraph = model.WorkloadGraph{
		Health: "critical",
		PropagationPath: []string{
			"Deployment/payment-api → ReplicaSet/payment-api-77c9 (rollout revision 42)",
			"ReplicaSet/payment-api-77c9 → 3 new pods on v2.1.4",
			"Pods → OOMKilled (memory limit 512Mi exceeded)",
			"Service/payment-api → endpoints dropped 5 → 2",
		},
	}
	st.Verdict = demoVerdict()
	st.HypothesisLabel = "Memory regression after rollout"
	st.HypothesisReasons = []string{"OOMKilled ×12", "Memory allocation failures ×18", "New ReplicaSet"}
	st.HypothesisStatus = "Confirmed"
	st.ConfidenceTrend = "up"
	st.HypothesisChanges = 2
	st.LastTransition = &model.HypothesisTransition{From: "Readiness failed", To: "OOMKilled", ConfDelta: 0.13}
	st.HypothesisAlts = []model.Hypothesis{{Label: "Dependency issue", Category: "Dependency failure", Confidence: 0.45}}
	st.CausalChain = buildCausalChain(st.Timeline)
	st.NextChecks = nextChecksFor("OOMKilled", bundle)
	st.Correlation = []string{"✓ Rollout preceded failures", "✓ Memory logs precede OOM", "✓ Endpoint loss followed OOM"}
	st.Counters = model.StreamCounters{EventsIngested: 27, LogsIngested: 143, ObjectChanges: 19, MetricSamples: 8, LastEventAt: at(9, 42, 10)}
	st.ActiveWatches = demoWatches(at(9, 38, 0))
	return st
}

func demoEvents(at func(int, int, int) time.Time) []model.EventRecord {
	obj := func(name string) model.ObjectRef { return model.ObjectRef{Kind: "Pod", Name: name, Namespace: "payments"} }
	return []model.EventRecord{
		{Timestamp: at(9, 40, 2), Type: "Normal", Reason: "Scheduled", Count: 3, Message: "assigned to ip-10-0-4-13", InvolvedObject: obj("payment-77c9-c3")},
		{Timestamp: at(9, 40, 22), Type: "Warning", Reason: "OOMKilled", Count: 12, Message: "container app exceeded memory limit", InvolvedObject: obj("payment-77c9-a1")},
		{Timestamp: at(9, 41, 21), Type: "Warning", Reason: "Unhealthy", Count: 5, Message: "readiness probe failed: connection refused", InvolvedObject: obj("payment-77c9-a1")},
		{Timestamp: at(9, 41, 55), Type: "Warning", Reason: "BackOff", Count: 8, Message: "back-off restarting failed container", InvolvedObject: obj("payment-77c9-b2")},
		{Timestamp: at(9, 42, 10), Type: "Warning", Reason: "OOMKilled", Count: 12, Message: "container app exceeded memory limit", InvolvedObject: obj("payment-77c9-b2")},
	}
}

func demoTimeline(at func(int, int, int) time.Time) []model.TimelineEvent {
	return []model.TimelineEvent{
		{Timestamp: at(9, 38, 22), Type: "deploy", Severity: model.SeverityInfo, SourceKind: "Deployment", SourceName: "deployment/payment-api", Message: "revision 42 started"},
		{Timestamp: at(9, 38, 31), Type: "rs", Severity: model.SeverityInfo, SourceKind: "ReplicaSet", SourceName: "replicaset/payment-77c9", Message: "created"},
		{Timestamp: at(9, 38, 46), Type: "pod", Severity: model.SeverityInfo, SourceKind: "Pod", SourceName: "payment-77c9-a1", Message: "scheduled on ip-10-0-4-12"},
		{Timestamp: at(9, 39, 2), Type: "pod", Severity: model.SeverityInfo, SourceKind: "Pod", SourceName: "payment-77c9-a1", Message: "container started"},
		{Timestamp: at(9, 39, 44), Type: "event", Severity: model.SeverityWarning, SourceKind: "Pod", SourceName: "payment-77c9-a1", Reason: "Unhealthy", Message: "readiness probe failed"},
		{Timestamp: at(9, 40, 10), Type: "log", Severity: model.SeverityWarning, SourceKind: "Pod", SourceName: "payment-77c9-a1/app", Message: "redis timeout spike detected"},
		{Timestamp: at(9, 40, 22), Type: "event", Severity: model.SeverityCritical, SourceKind: "Pod", SourceName: "payment-77c9-a1", Reason: "OOMKilled", Message: "container exceeded memory limit", Confidence: 0.95},
		{Timestamp: at(9, 40, 31), Type: "event", Severity: model.SeverityHigh, SourceKind: "Service", SourceName: "payment-api service", Message: "endpoints dropped from 5 to 2"},
		{Timestamp: at(9, 41, 5), Type: "event", Severity: model.SeverityHigh, SourceKind: "Pod", SourceName: "payment-77c9-a1", Reason: "BackOff", Message: "back-off restarting container"},
		{Timestamp: at(9, 42, 0), Type: "verdict", Severity: model.SeverityCritical, SourceKind: "Klew", SourceName: "Klew", Message: "leading signal changed to OOMKilled"},
	}
}

func demoStream(at func(int, int, int) time.Time) []model.EvidenceEvent {
	return []model.EvidenceEvent{
		{Timestamp: at(9, 42, 10), SourceType: model.SourceK8sEvent, SourceKind: "Pod", SourceName: "payment-77c9", Pod: "payment-77c9", Severity: model.SeverityCritical, Reason: "OOMKilled", Message: "container exceeded memory limit"},
		{Timestamp: at(9, 42, 9), SourceType: model.SourceLog, SourceKind: "Pod", SourceName: "payment-77c9", Pod: "payment-77c9", Container: "app", Severity: model.SeverityCritical, Reason: "ERROR", Message: "redis connection timeout", Raw: "ERROR redis connection timeout"},
		{Timestamp: at(9, 42, 8), SourceType: model.SourceK8sEvent, SourceKind: "Service", SourceName: "payment-api", Severity: model.SeverityHigh, Reason: "no ready endpoints", Message: "service has 2/5 ready endpoints"},
		{Timestamp: at(9, 42, 7), SourceType: model.SourceLog, SourceKind: "Pod", SourceName: "payment-52aa", Pod: "payment-52aa", Container: "app", Severity: model.SeverityWarning, Reason: "WARN", Message: "readiness probe failed", Raw: "WARN readiness probe failed"},
		{Timestamp: at(9, 41, 55), SourceType: model.SourceK8sEvent, SourceKind: "Pod", SourceName: "payment-77c9-b2", Pod: "payment-77c9-b2", Severity: model.SeverityHigh, Reason: "BackOff", Message: "back-off restarting failed container"},
		{Timestamp: at(9, 41, 22), SourceType: model.SourceLog, SourceKind: "Pod", SourceName: "payment-77c9-c3", Pod: "payment-77c9-c3", Container: "app", Severity: model.SeverityWarning, Reason: "WARN", Message: "retrying redis connection", Raw: "WARN retrying redis connection"},
		{Timestamp: at(9, 40, 22), SourceType: model.SourceK8sEvent, SourceKind: "Pod", SourceName: "payment-77c9-a1", Pod: "payment-77c9-a1", Severity: model.SeverityCritical, Reason: "OOMKilled", Message: "container app exited 137"},
	}
}

func demoVerdict() model.Verdict {
	return model.Verdict{
		Status:        model.VerdictCritical,
		LeadingSignal: "OOMKilled",
		LikelyTrigger: "memory limit (512Mi) exceeded after rollout revision 42",
		Confidence:    0.9,
		Summary: "Payment API is unhealthy. The strongest evidence is repeated OOMKilled events on newly rolled out " +
			"pods (revision 42, image v2.1.4), followed by readiness probe failures and loss of service endpoints (5 to 2).",
		StrongSignals: []model.Signal{
			{ID: "oom", Label: "OOMKilled", Severity: model.SeverityCritical, Strength: "strong", Score: 95, Evidence: "3 pods OOMKilled, exit 137"},
			{ID: "endpoints", Label: "Zero ready endpoints", Severity: model.SeverityCritical, Strength: "strong", Score: 85, Evidence: "service payment-api 2/5 ready"},
		},
		MediumSignals: []model.Signal{
			{ID: "readiness", Label: "Readiness failures", Severity: model.SeverityHigh, Strength: "medium", Score: 70, Evidence: "readiness probe failed x5"},
		},
		WeakSignals: []model.Signal{
			{ID: "redis", Label: "Redis timeouts", Severity: model.SeverityWarning, Strength: "weak", Score: 45, Evidence: "redis timeout spike in logs"},
		},
		AffectedPods:     []string{"payments/payment-77c9-a1", "payments/payment-77c9-b2", "payments/payment-77c9-c3"},
		AffectedServices: []string{"payments/payment-api"},
		RecommendedNextChecks: []string{
			"Inspect container memory limit (512Mi) vs live usage (2410Mi across pods)",
			"Compare revision 42 vs 41 resource settings",
			"Verify readiness probe thresholds and dependencies (redis)",
		},
		MissingDataWarnings: []string{"node pressure limited (nodes get denied)"},
	}
}

func demoPermissions() []model.PermissionCheck {
	allow := func(res, verb string, ok bool) model.PermissionCheck {
		return model.PermissionCheck{Resource: res, Verb: verb, Namespace: "payments", Allowed: ok}
	}
	var out []model.PermissionCheck
	for _, res := range []string{"pods", "deployments", "replicasets", "services", "endpointslices", "events"} {
		for _, verb := range []string{"get", "list", "watch"} {
			out = append(out, allow(res, verb, true))
		}
	}
	out = append(out, allow("pods/log", "get", true))
	out = append(out, allow("nodes", "get", false))
	return out
}

func demoWatches(start time.Time) []model.ActiveWatch {
	names := []string{"pods", "deployments", "replicasets", "services", "endpointslices", "events", "logs:payment-77c9", "logs:payment-52aa"}
	var out []model.ActiveWatch
	for _, n := range names {
		out = append(out, model.ActiveWatch{Name: n, Resource: n, Namespace: "payments", StartedAt: start})
	}
	return out
}
