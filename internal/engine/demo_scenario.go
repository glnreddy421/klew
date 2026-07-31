package engine

import (
	"context"
	"strings"
	"time"

	"github.com/glnreddy421/klew/internal/investigation"
	"github.com/glnreddy421/klew/internal/model"
)

// DemoTarget is one discovered investigation target shown in the picker.
type DemoTarget struct {
	Name  string
	Kind  string
	Pods  int
	Extra string
}

// demoStep is one deterministic point on the incident timeline.
type demoStep struct {
	at     time.Duration
	emit   []model.EvidenceEvent
	mutate func(*model.EvidenceBundle)
}

// DemoScenario is a fully-simulated, deterministic incident that behaves like a
// live investigation without a Kubernetes cluster.
type DemoScenario struct {
	Query      string
	Namespace  string
	Context    string
	Conclusion string
	Targets    []DemoTarget

	base    func(target string) model.EvidenceBundle
	steps   func(target string) []demoStep
	tail    func(target string) []model.EvidenceEvent
	dataset func() investigation.Dataset
	recent  *model.RecentChange
}

// DiscoverDemo routes a business-concept query to a scenario. Deterministic.
func DiscoverDemo(query string) DemoScenario {
	q := strings.ToLower(strings.TrimSpace(query))
	switch {
	case strings.Contains(q, "vault"):
		return vaultScenario()
	case strings.Contains(q, "checkout"):
		return checkoutScenario()
	default:
		return paymentScenario()
	}
}

// StartDemoFor discovers and starts a demo for a query using the primary target.
func StartDemoFor(ctx context.Context, query string) *LiveSession {
	sc := DiscoverDemo(query)
	return StartDemo(ctx, sc, sc.Targets[0].Name)
}

// StartDemo starts a live session driven by the scenario's deterministic timeline.
func StartDemo(ctx context.Context, sc DemoScenario, target string) *LiveSession {
	ctx, cancel := context.WithCancel(ctx)
	st := demoState(sc, target)
	store := NewStore(&st)
	bus := NewBus(512)

	session := &LiveSession{Reducer: store, Bus: bus, cancel: cancel}
	go bus.RunConsumer(ctx, store.ApplyEvent)

	session.wg.Add(1)
	go func() {
		defer session.wg.Done()
		runScenario(ctx, store, bus, st.Snapshot, sc.steps(target), sc.tail(target))
	}()
	return session
}

func demoState(sc DemoScenario, target string) model.InvestigationState {
	bundle := sc.base(target)
	bundle.KubeContext = model.KubeContext{Context: sc.Context, Cluster: sc.Context, User: "sre@corp", Namespace: sc.Namespace}
	bundle.Namespace = sc.Namespace
	bundle.Query = target
	bundle.Permissions = demoPermissions()

	st := model.NewInvestigationState(target, model.ModeLive)
	st.KubeContext = bundle.KubeContext
	st.NamespaceScope = model.NamespaceScope{Primary: sc.Namespace}
	st.Snapshot = bundle
	st.Window = 15 * time.Minute
	st.TailLines = 200
	st.ActiveWatches = demoWatchers(sc.Namespace)
	st.ExpectedWatches = 10
	st.Permissions = bundle.Permissions
	st.WorkloadGraph = BuildGraph(bundle)
	if scope := DemoScope(sc, target); scope != nil {
		st.Scope = scope
		st.ExpectedWatches = 10
	}
	if sc.recent != nil {
		rc := *sc.recent
		rc.DeployedAt = time.Now().Add(-3 * time.Minute)
		st.RecentChange = &rc
	}
	return st
}

// DemoScope discovers and builds the InvestigationScope for a demo target by
// running the real discovery + scope-builder pipeline over the scenario dataset.
func DemoScope(sc DemoScenario, target string) *investigation.InvestigationScope {
	if sc.dataset == nil {
		return nil
	}
	ds := sc.dataset()
	root := investigation.Ref{Kind: "Deployment", Name: target, Namespace: sc.Namespace}
	for _, o := range ds.Objects {
		if o.Ref.Name == target && investigation.WorkloadRootKinds[o.Ref.Kind] {
			root = o.Ref
			break
		}
	}
	scope := investigation.BuildScope(ds, root)
	return &scope
}

// runScenario plays the deterministic timeline, then keeps the stream alive.
func runScenario(ctx context.Context, store *StateStore, bus *Bus, bundle model.EvidenceBundle, steps []demoStep, tail []model.EvidenceEvent) {
	b := bundle
	b.Pods = append([]model.PodSummary(nil), bundle.Pods...)
	b.Services = append([]model.ServiceSummary(nil), bundle.Services...)
	b.ReplicaSets = append([]model.ReplicaSetSummary(nil), bundle.ReplicaSets...)

	fired := make([]bool, len(steps))
	start := time.Now()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	ticks, tailIdx := 0, 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ticks++
			elapsed := time.Since(start)
			allDone := true
			for i := range steps {
				if fired[i] {
					continue
				}
				if steps[i].at > elapsed {
					allDone = false
					continue
				}
				fired[i] = true
				for _, e := range steps[i].emit {
					bus.Publish(e)
				}
				if steps[i].mutate != nil {
					steps[i].mutate(&b)
					store.ApplySnapshot(b, BuildGraph(b), nil, model.Verdict{})
				}
			}
			// keep the heartbeat alive after the scripted incident completes
			if allDone && len(tail) > 0 && ticks%8 == 0 {
				bus.Publish(tail[tailIdx%len(tail)])
				tailIdx++
			}
		}
	}
}

// ── evidence helpers ─────────────────────────────────────────────────────────

func evLog(ns, pod, ctr string, sev model.Severity, reason, msg string) model.EvidenceEvent {
	return model.EvidenceEvent{SourceType: model.SourceLog, SourceKind: "Pod", SourceName: pod, Namespace: ns,
		Pod: pod, Container: ctr, Severity: sev, Reason: reason, Message: msg, Raw: msg, Confidence: 0.8}
}

func evEvent(ns, kind, name string, sev model.Severity, reason, msg string) model.EvidenceEvent {
	return model.EvidenceEvent{SourceType: model.SourceK8sEvent, SourceKind: kind, SourceName: name, Namespace: ns,
		Pod: podIfKind(kind, name), Severity: sev, Reason: reason, Message: msg, Confidence: 0.85}
}

func evObj(ns, kind, name, reason, msg string) model.EvidenceEvent {
	return model.EvidenceEvent{SourceType: model.SourceObjectChange, SourceKind: kind, SourceName: name, Namespace: ns,
		Severity: model.SeverityInfo, Reason: reason, Message: msg, Confidence: 0.7}
}

func evMetric(ns, name, msg string) model.EvidenceEvent {
	return model.EvidenceEvent{SourceType: model.SourceMetric, SourceKind: "Metric", SourceName: name, Namespace: ns,
		Severity: model.SeverityInfo, Reason: "Metric", Message: msg, Confidence: 0.6}
}

func evSys(ns, msg string) model.EvidenceEvent {
	return model.EvidenceEvent{SourceType: model.SourceSystem, Namespace: ns, Severity: model.SeverityInfo,
		Reason: "System", Message: msg, Confidence: 1}
}

func podIfKind(kind, name string) string {
	if kind == "Pod" {
		return name
	}
	return ""
}

func demoWatchers(ns string) []model.ActiveWatch {
	names := []string{"deployments", "replicasets", "pods", "services", "endpointslices", "events", "logs", "configmaps", "secrets", "metrics"}
	out := make([]model.ActiveWatch, 0, len(names))
	for _, n := range names {
		out = append(out, model.ActiveWatch{Name: n, Resource: n, Namespace: ns, StartedAt: time.Now()})
	}
	return out
}

func demoPod(ns, name, node string, ready bool, restarts int32, img string) model.PodSummary {
	return model.PodSummary{Name: name, Namespace: ns, Node: node, Phase: "Running", Ready: ready, RestartCount: restarts,
		Labels: map[string]string{"app": name}, Containers: []model.ContainerStatus{{
			Name: "app", Image: img, Ready: ready, RestartCount: restarts, State: "running",
			RequestsCPU: "300m", RequestsMem: "512Mi", LimitsCPU: "500m", LimitsMem: "512Mi"}}}
}
