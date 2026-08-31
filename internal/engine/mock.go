package engine

import (
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

type mockEmit struct {
	sourceType model.SourceType
	kind, name string
	pod, ctr   string
	severity   model.Severity
	reason     string
	message    string
}

// mockBaseState returns a minimal live investigation state for engine tests.
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
		Permissions: fixturePermissions(),
	}

	st := model.NewInvestigationState("payment-gateway", model.ModeLive)
	st.KubeContext = bundle.KubeContext
	st.NamespaceScope = model.NamespaceScope{Primary: "prod"}
	st.Snapshot = bundle
	st.Window = model.DurationMS(15 * time.Minute)
	st.TailLines = 200
	st.WorkloadGraph = BuildGraph(bundle)
	st.ActiveWatches = fixtureWatches(model.TimestampFrom(time.Now()))
	st.ExpectedWatches = 8
	return st
}

func fixturePermissions() []model.PermissionCheck {
	allow := func(res, verb string, ok bool) model.PermissionCheck {
		return model.PermissionCheck{Resource: res, Verb: verb, Namespace: "prod", Allowed: ok}
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

func fixtureWatches(start model.Timestamp) []model.ActiveWatch {
	names := []string{"pods", "deployments", "replicasets", "services", "endpointslices", "events", "logs:payment-gateway", "logs:payment-gateway"}
	var out []model.ActiveWatch
	for _, n := range names {
		out = append(out, model.ActiveWatch{Name: n, Resource: n, Namespace: "prod", StartedAt: start})
	}
	return out
}
