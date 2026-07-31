package engine

import (
	"context"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

// StartMock builds a live session backed by a scripted evidence producer. It is
// used by `klew demo` (and web mode) to exercise the Incident Overview with
// live data that changes over time, including a leading-signal change.
func StartMock(ctx context.Context) *LiveSession {
	ctx, cancel := context.WithCancel(ctx)
	state := mockBaseState()
	store := NewStore(&state)
	bus := NewBus(512)

	session := &LiveSession{Reducer: store, Bus: bus, cancel: cancel}
	go bus.RunConsumer(ctx, store.ApplyEvent)

	session.wg.Add(1)
	go func() {
		defer session.wg.Done()
		runMockProducer(ctx, bus)
	}()
	return session
}

func mockBaseState() model.InvestigationState {
	labels := map[string]string{"app": "payment-gateway"}
	pod := func(name string, ready bool, restarts int32, oom bool) model.PodSummary {
		c := model.ContainerStatus{Name: "app", Image: "repo/payment-gateway:v2.3.1", Ready: ready, RestartCount: restarts, State: "running",
			RequestsCPU: "250m", RequestsMem: "384Mi", LimitsCPU: "500m", LimitsMem: "384Mi"}
		if oom {
			c.LastState, c.LastReason, c.LastExitCode = "terminated", "OOMKilled", 137
		}
		return model.PodSummary{Name: name, Namespace: "prod", Node: "ip-10-0-2-11", Phase: "Running", Ready: ready, RestartCount: restarts, Labels: labels,
			Containers: []model.ContainerStatus{c}}
	}
	bundle := model.EvidenceBundle{
		CollectedAt: model.TimestampFrom(time.Now()),
		Namespace:   "prod",
		Query:       "payment-gateway",
		KubeContext: model.KubeContext{Context: "prod-eks", Cluster: "prod-eks", User: "sre@corp", Namespace: "prod"},
		Workloads: []model.WorkloadSummary{{Kind: "Deployment", Name: "payment-gateway", Namespace: "prod",
			Replicas: 4, Ready: 1, Available: 1, Updated: 3, Generation: 47, Selector: "app=payment-gateway"}},
		ReplicaSets: []model.ReplicaSetSummary{
			{Name: "payment-gateway-77c9", Namespace: "prod", Replicas: 3, Ready: 0, DeploymentOwner: "payment-gateway"},
			{Name: "payment-gateway-52aa", Namespace: "prod", Replicas: 1, Ready: 1, DeploymentOwner: "payment-gateway"},
		},
		Pods: []model.PodSummary{
			pod("payment-gateway-a1", false, 4, false),
			pod("payment-gateway-a2", false, 3, false),
			pod("payment-gateway-a3", false, 2, false),
			pod("payment-gateway-a4", true, 0, false),
		},
		Services: []model.ServiceSummary{{Name: "payment-gateway", Namespace: "prod", Type: "ClusterIP",
			Selector: "app=payment-gateway", ReadyEndpoints: 1, TotalEndpoints: 4, Ports: []string{"80/TCP"}}},
		Nodes:       []model.NodeSummary{{Name: "ip-10-0-2-11", Ready: true}},
		ConfigRefs:  []model.ResourceRef{{Kind: "ConfigMap", Name: "payment-gateway-config", Namespace: "prod"}},
		Permissions: demoPermissions(),
	}

	st := model.NewInvestigationState("payment-gateway", model.ModeLive)
	st.KubeContext = bundle.KubeContext
	st.NamespaceScope = model.NamespaceScope{Primary: "prod"}
	st.Snapshot = bundle
	st.Window = model.DurationMS(15 * time.Minute)
	st.TailLines = 200
	st.WorkloadGraph = BuildGraph(bundle)
	st.ActiveWatches = demoWatches(model.TimestampFrom(time.Now()))
	st.ExpectedWatches = 8
	return st
}

type mockEmit struct {
	sourceType model.SourceType
	kind, name string
	pod, ctr   string
	severity   model.Severity
	reason     string
	message    string
}

// runMockProducer emits scripted evidence: first redis/readiness signals make
// "Dependency failure" lead, then OOM + memory-log signals flip the leading
// signal to OOMKilled, which the store records as a hypothesis change.
func runMockProducer(ctx context.Context, bus *Bus) {
	script := []mockEmit{
		{model.SourceLog, "Pod", "payment-gateway-a1", "payment-gateway-a1", "app", model.SeverityWarning, "Redis timeout", "redis connection timeout after 5000ms"},
		{model.SourceLog, "Pod", "payment-gateway-a2", "payment-gateway-a2", "app", model.SeverityWarning, "Redis timeout", "redis connection timeout after 5000ms"},
		{model.SourceK8sEvent, "Pod", "payment-gateway-a1", "payment-gateway-a1", "", model.SeverityHigh, "Readiness failed", "readiness probe failed: connection refused"},
		{model.SourceLog, "Pod", "payment-gateway-a3", "payment-gateway-a3", "app", model.SeverityWarning, "Redis timeout", "redis connection timeout after 5000ms"},
		{model.SourceK8sEvent, "Pod", "payment-gateway-a2", "payment-gateway-a2", "", model.SeverityHigh, "Readiness failed", "readiness probe failed: connection refused"},
		// leading flip begins here
		{model.SourceK8sEvent, "Pod", "payment-gateway-a1", "payment-gateway-a1", "", model.SeverityCritical, "OOMKilled", "container app exceeded memory limit (exit 137)"},
		{model.SourceLog, "Pod", "payment-gateway-a1", "payment-gateway-a1", "app", model.SeverityCritical, "Memory pressure", "fatal: out of memory allocating 64MB"},
		{model.SourceK8sEvent, "Pod", "payment-gateway-a2", "payment-gateway-a2", "", model.SeverityCritical, "OOMKilled", "container app exceeded memory limit (exit 137)"},
		{model.SourceK8sEvent, "Service", "payment-gateway", "", "", model.SeverityHigh, "EndpointsDropped", "endpoints dropped from 4 to 1"},
		{model.SourceLog, "Pod", "payment-gateway-a2", "payment-gateway-a2", "app", model.SeverityCritical, "Memory pressure", "fatal: out of memory allocating 64MB"},
		{model.SourceK8sEvent, "Pod", "payment-gateway-a3", "payment-gateway-a3", "", model.SeverityHigh, "BackOff", "back-off restarting failed container"},
	}
	// steady-state loop keeps OOM leading and the stream alive
	tail := []mockEmit{
		{model.SourceK8sEvent, "Pod", "payment-gateway-a1", "payment-gateway-a1", "", model.SeverityCritical, "OOMKilled", "container app exceeded memory limit (exit 137)"},
		{model.SourceLog, "Pod", "payment-gateway-a3", "payment-gateway-a3", "app", model.SeverityWarning, "Redis timeout", "redis connection timeout after 5000ms"},
		{model.SourceLog, "Pod", "payment-gateway-a1", "payment-gateway-a1", "app", model.SeverityInfo, "", "GET /healthz 200 3ms"},
		{model.SourceK8sEvent, "Pod", "payment-gateway-a2", "payment-gateway-a2", "", model.SeverityHigh, "Readiness failed", "readiness probe failed: connection refused"},
	}

	ticker := time.NewTicker(800 * time.Millisecond)
	defer ticker.Stop()
	i := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var e mockEmit
			if i < len(script) {
				e = script[i]
			} else {
				e = tail[(i-len(script))%len(tail)]
			}
			i++
			bus.Publish(model.EvidenceEvent{
				Timestamp: model.TimestampFrom(time.Now()), SourceType: e.sourceType, SourceKind: e.kind, SourceName: e.name,
				Namespace: "prod", Pod: e.pod, Container: e.ctr, Severity: e.severity,
				Reason: e.reason, Message: e.message, Raw: e.message, Confidence: 0.8,
			})
		}
	}
}
