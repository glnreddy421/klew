package engine

import (
	"strings"
	"testing"

	"github.com/glnreddy421/klew/internal/investigation"
	"github.com/glnreddy421/klew/internal/model"
)

// playScenario applies a scenario's timeline synchronously (no timers) so the
// deterministic conclusion can be asserted.
func playScenario(sc DemoScenario, target string) model.InvestigationState {
	st := demoState(sc, target)
	store := NewStore(&st)

	b := st.Snapshot
	b.Pods = append([]model.PodSummary(nil), st.Snapshot.Pods...)
	b.Services = append([]model.ServiceSummary(nil), st.Snapshot.Services...)
	b.ReplicaSets = append([]model.ReplicaSetSummary(nil), st.Snapshot.ReplicaSets...)

	for _, step := range sc.steps(target) {
		for _, e := range step.emit {
			store.ApplyEvent(e)
		}
		if step.mutate != nil {
			step.mutate(&b)
			store.ApplySnapshot(b, BuildGraph(b), nil, model.Verdict{})
		}
	}
	return store.State()
}

func TestDemoPaymentConcludesOOM(t *testing.T) {
	st := playScenario(paymentScenario(), "payment-api")
	if st.Verdict.LeadingSignal != "OOMKilled" {
		t.Fatalf("payment: expected leading OOMKilled, got %q", st.Verdict.LeadingSignal)
	}
	if !strings.Contains(st.HypothesisLabel, "rollout") {
		t.Fatalf("payment: expected rollout-correlated OOM hypothesis, got %q", st.HypothesisLabel)
	}
	if st.Verdict.Status != model.VerdictCritical {
		t.Fatalf("payment: expected critical, got %q", st.Verdict.Status)
	}
	if st.HypothesisChanges == 0 {
		t.Fatalf("payment: expected leading signal to evolve")
	}
	if st.Verdict.Confidence >= 1.0 {
		t.Fatalf("payment: confidence must never be 100%%, got %v", st.Verdict.Confidence)
	}
}

func TestDemoVaultConcludesConfig(t *testing.T) {
	st := playScenario(vaultScenario(), "vault-controller")
	if st.HypothesisLabel != "Configuration issue" {
		t.Fatalf("vault: expected configuration issue, got %q (leading %q)", st.HypothesisLabel, st.Verdict.LeadingSignal)
	}
	if st.Verdict.LeadingSignal == "OOMKilled" {
		t.Fatalf("vault: must not conclude OOM")
	}
}

func TestDemoCheckoutConcludesRegistry(t *testing.T) {
	st := playScenario(checkoutScenario(), "checkout-api")
	if st.HypothesisLabel != "Registry issue" {
		t.Fatalf("checkout: expected registry issue, got %q (leading %q)", st.HypothesisLabel, st.Verdict.LeadingSignal)
	}
}

func TestDiscoverDemoRouting(t *testing.T) {
	cases := map[string]string{
		"payment": "payment", "pay": "payment", "worker": "payment",
		"vault": "vault", "checkout": "checkout", "orders": "payment",
	}
	for q, want := range cases {
		if got := DiscoverDemo(q).Query; got != want {
			t.Errorf("DiscoverDemo(%q).Query = %q, want %q", q, got, want)
		}
	}
	if len(DiscoverDemo("payment").Targets) != 4 {
		t.Errorf("payment should discover 4 targets")
	}
	if len(DiscoverDemo("vault").Targets) != 3 {
		t.Errorf("vault should discover 3 targets")
	}
}

func TestDemoDiscoveryGroupsPayment(t *testing.T) {
	sc := paymentScenario()
	groups := investigation.Discover(sc.dataset(), sc.Query)
	if len(groups) != 4 {
		t.Fatalf("expected 4 payment workload groups, got %d", len(groups))
	}
	// payment-api should expose 4 pods and its Service
	var api *investigation.WorkloadGroup
	for i := range groups {
		if groups[i].Root.Name == "payment-api" {
			api = &groups[i]
		}
	}
	if api == nil || api.Pods != 4 || api.Service != "payment-api" {
		t.Fatalf("payment-api group wrong: %+v", api)
	}
}

func TestDemoScopeAttachedAndScoped(t *testing.T) {
	st := demoState(paymentScenario(), "payment-api")
	if st.Scope == nil {
		t.Fatal("expected scope to be attached to state")
	}
	stats := st.Scope.Stats()
	if stats.Resources < 15 {
		t.Errorf("expected a rich scope, got %d resources", stats.Resources)
	}
	if len(st.Scope.Extensions) == 0 {
		t.Errorf("expected detected extensions")
	}
	// unrelated payment workloads must not leak into the payment-api scope
	for _, d := range st.Scope.Deployments {
		if d.Name == "payment-worker" || d.Name == "payment-admin" {
			t.Fatalf("unrelated workload %s leaked into scope", d.Name)
		}
	}
}

