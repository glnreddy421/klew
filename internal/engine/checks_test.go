package engine

import (
	"strings"
	"testing"

	"github.com/glnreddy421/klew/internal/model"
)

func TestCrashLoopNextChecksAreActionable(t *testing.T) {
	b := model.EvidenceBundle{
		Namespace: "klew-lab",
		Workloads: []model.WorkloadSummary{{
			Kind: "Deployment", Name: "payment-api", Selector: "app=payment-api",
		}},
		Pods: []model.PodSummary{{
			Name: "payment-api-abc12",
			Containers: []model.ContainerStatus{{
				Name: "app", Reason: "CrashLoopBackOff", LastReason: "Error",
				RestartCount: 7, Ready: false,
			}},
		}},
	}
	signals := ScoreSignals(b)
	corr := CorrelationEngine{}.Correlate(b, signals, nil)
	if corr.HypothesisLabel != "Container crash loop" {
		t.Fatalf("hypothesis=%q want Container crash loop", corr.HypothesisLabel)
	}
	if len(corr.NextChecks) == 0 {
		t.Fatal("expected next steps for crashloop")
	}
	for _, c := range corr.NextChecks {
		if strings.HasPrefix(c, "kubectl") {
			t.Fatalf("next steps should not be kubectl commands: %q", c)
		}
	}
	foundTermination := false
	foundLogs := false
	for _, c := range corr.NextChecks {
		if strings.Contains(c, "payment-api-abc12") && strings.Contains(c, "termination") {
			foundTermination = true
		}
		if strings.Contains(c, "payment-api-abc12") && strings.Contains(c, "logs") {
			foundLogs = true
		}
	}
	if !foundTermination {
		t.Fatalf("expected termination step referencing failing pod, got %v", corr.NextChecks)
	}
	if !foundLogs {
		t.Fatalf("expected log review step referencing failing pod, got %v", corr.NextChecks)
	}
	for _, c := range corr.FixActions {
		if strings.Contains(c, "rollout undo") {
			t.Fatalf("rollout undo suggested with no previous revision: %v", corr.FixActions)
		}
	}
}

func shopBundle() model.EvidenceBundle {
	owned := func(name, rs, hash string, c model.ContainerStatus) model.PodSummary {
		return model.PodSummary{
			Name: name, Phase: "Running", Labels: map[string]string{"pod-template-hash": hash},
			OwnerRefs: []model.ObjectRef{{Kind: "ReplicaSet", Name: rs}}, Containers: []model.ContainerStatus{c},
		}
	}
	return model.EvidenceBundle{
		Namespace: "shop",
		Workloads: []model.WorkloadSummary{{Kind: "Deployment", Name: "web"}, {Kind: "Deployment", Name: "api"}, {Kind: "Deployment", Name: "checkout"}},
		ReplicaSets: []model.ReplicaSetSummary{
			{Name: "checkout-aaa", DeploymentOwner: "checkout"},
			{Name: "checkout-bbb", DeploymentOwner: "checkout"},
		},
		Pods: []model.PodSummary{
			owned("web-1", "web-ccc", "ccc", model.ContainerStatus{Name: "nginx", Ready: false, State: "running"}),
			owned("api-1", "api-ddd", "ddd", model.ContainerStatus{Name: "api", LastReason: "OOMKilled", LastExitCode: 137, LimitsMem: "64Mi", RestartCount: 9}),
			owned("checkout-1", "checkout-bbb", "bbb", model.ContainerStatus{Name: "checkout", Reason: "CrashLoopBackOff", RestartCount: 5}),
		},
	}
}

func TestFixActionsNameFailingWorkloadAndContainer(t *testing.T) {
	b := shopBundle()
	b.Pods = b.Pods[:2] // web and api only
	got := strings.Join(fixActionsFor("OOMKilled", b), "\n")
	want := "kubectl -n shop set resources deployment/api -c api --limits=memory=128Mi"
	if !strings.Contains(got, want) {
		t.Fatalf("fix actions = %q, want %q", got, want)
	}
	if strings.Contains(got, "rollout undo") {
		t.Fatalf("api has one revision, undo suggested: %q", got)
	}
	if strings.Contains(got, "klew-labs") || strings.Contains(got, "-c app") {
		t.Fatalf("lab-specific or guessed container in fix actions: %q", got)
	}
}

func TestFixActionsUndoOnlyWithPreviousRevision(t *testing.T) {
	b := shopBundle()
	b.Pods = append(b.Pods[:1], b.Pods[2]) // web and checkout
	got := fixActionsFor("CrashLoopBackOff", b)
	if len(got) != 1 || got[0] != "kubectl -n shop rollout undo deployment/checkout" {
		t.Fatalf("fix actions = %v, want rollout undo of checkout", got)
	}
	for _, c := range fixActionsFor("CrashLoopBackOff", b) {
		if strings.Contains(c, "patch") {
			t.Fatalf("crash loop fix patches the spec: %q", c)
		}
	}
}

func TestFixActionsWithoutKnownWorkload(t *testing.T) {
	b := shopBundle()
	b.Pods[1].OwnerRefs = nil
	b.Pods = b.Pods[1:2]
	if got := fixActionsFor("OOMKilled", b); len(got) != 0 {
		t.Fatalf("fix actions for an unowned pod among several workloads = %v, want none", got)
	}
}

func TestOOMNextChecksWithoutMetrics(t *testing.T) {
	b := model.EvidenceBundle{
		Namespace: "klew-lab",
		Metrics:   model.MetricsSummary{Available: false},
		Pods: []model.PodSummary{{
			Name: "payment-api-xyz",
			Containers: []model.ContainerStatus{{
				Name: "app", LastReason: "OOMKilled", LastExitCode: 137,
				LimitsMem: "128Mi", RestartCount: 3,
			}},
		}},
	}
	steps := investigationStepsFor("OOMKilled", b)
	if len(steps) == 0 {
		t.Fatal("expected OOM investigation steps")
	}
	found := false
	for _, s := range steps {
		if strings.Contains(s, "128Mi") && strings.Contains(s, "OOMKilled") {
			found = true
		}
		if strings.HasPrefix(s, "kubectl") {
			t.Fatalf("unexpected kubectl in steps: %q", s)
		}
	}
	if !found {
		t.Fatalf("expected limit-aware OOM step, got %v", steps)
	}
}

func TestFailingPodPrefersCrashLoop(t *testing.T) {
	b := model.EvidenceBundle{
		Pods: []model.PodSummary{
			{Name: "healthy-a1", Containers: []model.ContainerStatus{{Name: "app", Ready: true}}},
			{Name: "broken-b2", Containers: []model.ContainerStatus{{
				Name: "app", Reason: "CrashLoopBackOff", RestartCount: 4,
			}}},
		},
	}
	pod, _ := failingPod(b)
	if pod != "broken-b2" {
		t.Fatalf("failingPod=%q want broken-b2", pod)
	}
}
