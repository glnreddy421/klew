package kube

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/glnreddy421/klew/internal/model"
)

func TestInvestigableTypesComeFromCollectors(t *testing.T) {
	custom, err := NewResourceCollector(ResourceCollectorSpec{Name: "certificates", Group: "cert-manager.io", Resource: "certificates"})
	if err != nil {
		t.Fatal(err)
	}
	types := InvestigableTypes([]EvidenceCollector{custom})
	got := map[string]model.InvestigableType{}
	for _, it := range types {
		got[it.Group+"/"+it.Resource] = it
	}
	for _, want := range []string{"apps/deployments", "apps/replicasets", "/pods", "/services", "/configmaps", "gateway.networking.k8s.io/httproutes"} {
		if _, ok := got[want]; !ok {
			t.Errorf("missing investigable type %s", want)
		}
	}
	if got["apps/deployments"].Kind != "Deployment" {
		t.Errorf("typed lister kind = %q", got["apps/deployments"].Kind)
	}
	if c := got["cert-manager.io/certificates"]; !c.Custom || c.Collector != "certificates" {
		t.Errorf("custom collector = %+v", c)
	}
	for _, never := range []string{"/secrets", "coordination.k8s.io/leases", "storage.k8s.io/storageclasses", "/nodes"} {
		if _, ok := got[never]; ok {
			t.Errorf("%s should not be investigable", never)
		}
	}
	if types[len(types)-1].Group != "cert-manager.io" {
		t.Errorf("custom types should come last, got %+v", types[len(types)-1])
	}
}

func TestTargetWarningsSayWhenNoCollectorReadsAKind(t *testing.T) {
	cs := fake.NewSimpleClientset()
	cs.Resources = []*metav1.APIResourceList{
		{GroupVersion: "v1", APIResources: []metav1.APIResource{{Name: "configmaps", Kind: "ConfigMap", Namespaced: true}}},
		{GroupVersion: "networking.k8s.io/v1", APIResources: []metav1.APIResource{{Name: "networkpolicies", Kind: "NetworkPolicy", Namespaced: true}}},
	}
	env := &CollectEnv{Client: &Client{Clientset: cs}, Namespace: "shop"}
	target := &model.ScopeTarget{Found: true, Missing: []string{"Lease/leader", "Deployment/nope", "NetworkPolicy/gone"}}
	got := strings.Join(targetWarnings(env, DefaultCollectors(), target, "shop"), "\n")
	for _, want := range []string{
		"Not investigated: Lease/leader. Klew has no collector for Lease",
		"Not found in shop: Deployment/nope, NetworkPolicy/gone",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("warnings %q lack %q", got, want)
		}
	}
}

func TestTargetWarningsNeverClaimUnsupportedWithoutDiscovery(t *testing.T) {
	target := &model.ScopeTarget{Ref: model.ObjectRef{Kind: "Lease", Name: "leader"}}
	got := targetWarnings(&CollectEnv{}, DefaultCollectors(), target, "shop")
	if len(got) != 1 || got[0] != "Lease/leader was not found in shop" {
		t.Errorf("warnings = %q", got)
	}
}
