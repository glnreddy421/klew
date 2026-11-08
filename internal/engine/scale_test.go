package engine

import (
	"fmt"
	"testing"
	"time"

	"github.com/glnreddy421/klew/v2/internal/model"
)

// bigBundle is a namespace with n crash-looping pods across n/2 Deployments,
// one Service per Deployment and an event per pod.
func bigBundle(n int) model.EvidenceBundle {
	var b model.EvidenceBundle
	b.Permissions = endpointsReadable
	for d := 0; d < n/2; d++ {
		rs := fmt.Sprintf("app%d-abc", d)
		b.ReplicaSets = append(b.ReplicaSets, model.ReplicaSetSummary{Name: rs, DeploymentOwner: fmt.Sprintf("app%d", d)})
		b.Services = append(b.Services, model.ServiceSummary{Name: fmt.Sprintf("app%d", d), Selector: fmt.Sprintf("app=app%d", d), TotalEndpoints: 2})
		for i := 0; i < 2; i++ {
			name := fmt.Sprintf("%s-%d", rs, i)
			b.Pods = append(b.Pods, model.PodSummary{
				Name: name, Phase: "Running", RestartCount: 5, Labels: map[string]string{"app": fmt.Sprintf("app%d", d)},
				OwnerRefs: []model.ObjectRef{{Kind: "ReplicaSet", Name: rs}},
				Containers: []model.ContainerStatus{{Name: "c", State: "waiting", Reason: "CrashLoopBackOff", RestartCount: 5,
					LastReason: "Error", LastExitCode: 1, LivenessProbe: "tcp :80"}},
			})
			b.Events = append(b.Events, model.EventRecord{Reason: "Unhealthy", Message: "Liveness probe failed: refused",
				InvolvedObject: model.ObjectRef{Kind: "Pod", Name: name}})
		}
	}
	return b
}

func TestRulesScaleToLargeNamespaces(t *testing.T) {
	b := bigBundle(5000)
	start := time.Now()
	res := DefaultRegistry().Evaluate(b)
	problems := BuildProblems(b, res.Findings)
	elapsed := time.Since(start)
	t.Logf("5000 pods: %d findings, %d problems in %s", len(res.Findings), len(problems), elapsed)
	if len(problems) != 5000 { // 2500 Deployments + 2500 Services
		t.Fatalf("problems = %d", len(problems))
	}
	if elapsed > 2*time.Second && !raceEnabled {
		t.Fatalf("rules took %s for 5000 pods", elapsed)
	}
}

func TestRulesScaleWithSelectorsPoliciesAndCEL(t *testing.T) {
	b := bigBundle(5000)
	for i := range b.Services {
		if i%2 == 0 {
			b.Services[i].TotalEndpoints = 0
			b.Services[i].Selector += ",tier=gone"
		}
	}
	var policies []map[string]any
	for i := 0; i < 500; i++ {
		policies = append(policies, with(with(meta("NetworkPolicy", fmt.Sprintf("np%d", i)),
			"spec.podSelector", obj{"matchLabels": obj{"app": fmt.Sprintf("app%d", i)}}), "spec.policyTypes", []any{"Ingress"}))
	}
	b.Resources = map[string][]map[string]any{"networkpolicies": policies}
	rule, err := NewCELRule("restarts", RuleSpec{Resource: "pods", State: "Restarting", When: "object.restartCount > 3"})
	if err != nil {
		t.Fatal(err)
	}
	reg := &Registry{Rules: append(DefaultRules(), rule)}
	start := time.Now()
	res := reg.Evaluate(b)
	BuildProblems(b, res.Findings)
	elapsed := time.Since(start)
	t.Logf("5000 pods, 1250 empty Services, 500 policies, 1 CEL rule: %d findings in %s", len(res.Findings), elapsed)
	if elapsed > 5*time.Second && !raceEnabled {
		t.Fatalf("rules took %s", elapsed)
	}
}
