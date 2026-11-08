package kube

import (
	"context"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	discoveryfake "k8s.io/client-go/discovery/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/glnreddy421/klew/v2/internal/model"
)

func certificate(ns, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "cert-manager.io/v1", "kind": "Certificate",
		"metadata": map[string]any{
			"name": name, "namespace": ns,
			"managedFields": []any{map[string]any{"manager": "kubectl"}},
			"annotations":   map[string]any{"kubectl.kubernetes.io/last-applied-configuration": "{}", "team": "edge"},
		},
	}}
}

func TestResourceCollectorListsOnlyTheNamespaceAndTrims(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "cert-manager.io", Version: "v1", Resource: "certificates"}
	fake := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{gvr: "CertificateList"},
		certificate("shop", "web-tls"), certificate("other", "api-tls"))
	c, err := NewResourceCollector(ResourceCollectorSpec{Name: "certificates", Group: gvr.Group, Version: gvr.Version, Resource: "Certificates"})
	if err != nil {
		t.Fatal(err)
	}
	c.(*resourceCollector).newDynamic = func(*Client) (dynamic.Interface, error) { return fake, nil }

	var b model.EvidenceBundle
	c.Collect(context.Background(), &CollectEnv{Namespace: "shop"}, &b)
	got := b.Resources["certificates"]
	if len(got) != 1 {
		t.Fatalf("collected %d objects, want 1 from namespace shop: %v", len(got), got)
	}
	meta := got[0]["metadata"].(map[string]any)
	if meta["name"] != "web-tls" || meta["managedFields"] != nil {
		t.Fatalf("metadata = %v", meta)
	}
	if ann := meta["annotations"].(map[string]any); ann["team"] != "edge" || len(ann) != 1 {
		t.Fatalf("annotations = %v", ann)
	}
	for _, a := range fake.Actions() {
		if a.GetVerb() != "list" {
			t.Fatalf("collector issued %s; only list is allowed", a.GetVerb())
		}
	}
}

func TestResourceCollectorWarnsOnListError(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "x.io", Version: "v1", Resource: "things"}
	fake := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{gvr: "ThingList"})
	fake.PrependReactor("list", "things", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, context.DeadlineExceeded
	})
	c, _ := NewResourceCollector(ResourceCollectorSpec{Name: "things", Group: gvr.Group, Version: gvr.Version, Resource: gvr.Resource})
	c.(*resourceCollector).newDynamic = func(*Client) (dynamic.Interface, error) { return fake, nil }
	var b model.EvidenceBundle
	c.Collect(context.Background(), &CollectEnv{Namespace: "ns"}, &b)
	if len(b.Warnings) != 1 || b.Resources != nil {
		t.Fatalf("warnings = %v, resources = %v", b.Warnings, b.Resources)
	}
}

func TestResourceCollectorRefusesSecrets(t *testing.T) {
	for _, spec := range []ResourceCollectorSpec{
		{Name: "s", Version: "v1", Resource: "secrets"},
		{Name: "s", Version: "v1", Resource: " Secrets "},
		{Name: "l", Version: "v1", Resource: "pods/log"},
	} {
		if _, err := NewResourceCollector(spec); err == nil {
			t.Errorf("NewResourceCollector(%+v) succeeded", spec)
		}
	}
}

func fakeEnv(groups ...metav1.APIGroup) *CollectEnv {
	cs := kubefake.NewSimpleClientset()
	disco := cs.Discovery().(*discoveryfake.FakeDiscovery)
	for _, g := range groups {
		disco.Resources = append(disco.Resources, &metav1.APIResourceList{GroupVersion: g.PreferredVersion.GroupVersion})
	}
	return &CollectEnv{Namespace: "ns", Client: &Client{Clientset: cs}}
}

func TestOptionalCollectorSkipsQuietlyWhenAPIMissing(t *testing.T) {
	c, _ := NewResourceCollector(ResourceCollectorSpec{Name: "gateways", Group: "gateway.networking.k8s.io", Resource: "gateways", Optional: true})
	c.(*resourceCollector).newDynamic = func(*Client) (dynamic.Interface, error) {
		t.Fatal("listed an API the cluster does not serve")
		return nil, nil
	}
	var b model.EvidenceBundle
	c.Collect(context.Background(), fakeEnv(), &b)
	if len(b.Warnings) != 0 || b.Resources != nil {
		t.Fatalf("warnings = %v, resources = %v", b.Warnings, b.Resources)
	}
}

func TestOptionalCollectorUsesPreferredVersion(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gateways"}
	gw := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "gateway.networking.k8s.io/v1", "kind": "Gateway",
		"metadata": map[string]any{"name": "public", "namespace": "ns"}}}
	fake := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{gvr: "GatewayList"})
	// The fake guesses "gatewaies" from the kind, so add the object under its real resource.
	if err := fake.Tracker().Create(gvr, gw, "ns"); err != nil {
		t.Fatal(err)
	}
	c, _ := NewResourceCollector(ResourceCollectorSpec{Name: "gateways", Group: gvr.Group, Resource: gvr.Resource, Optional: true})
	c.(*resourceCollector).newDynamic = func(*Client) (dynamic.Interface, error) { return fake, nil }
	env := fakeEnv(metav1.APIGroup{Name: gvr.Group, PreferredVersion: metav1.GroupVersionForDiscovery{GroupVersion: "gateway.networking.k8s.io/v1", Version: "v1"}})
	var b model.EvidenceBundle
	c.Collect(context.Background(), env, &b)
	if len(b.Resources["gateways"]) != 1 {
		t.Fatalf("resources = %v, warnings = %v", b.Resources, b.Warnings)
	}
}

func TestCollectorOutcomesAreRecorded(t *testing.T) {
	forbidden := func(res string) EvidenceCollector {
		gvr := schema.GroupVersionResource{Version: "v1", Resource: res}
		fake := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{gvr: "List"})
		fake.PrependReactor("list", res, func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: res}, "", nil)
		})
		c, _ := NewResourceCollector(ResourceCollectorSpec{Name: res, Version: "v1", Resource: res, ClusterScoped: true, Optional: true})
		c.(*resourceCollector).newDynamic = func(*Client) (dynamic.Interface, error) { return fake, nil }
		return c
	}
	missing, _ := NewResourceCollector(ResourceCollectorSpec{Name: "gateways", Group: "gateway.networking.k8s.io", Resource: "gateways", Optional: true})
	env := fakeEnv()
	// storageclasses is denied by review, so it must not even be listed.
	env.access = map[string]model.PermissionCheck{
		(Access{Group: "", Resource: "storageclasses", Verb: "list", ClusterScoped: true}).key("ns"): {Resource: "storageclasses", Verb: "list"},
	}
	denied := NewCollectorWithAccess("storageclasses", []Access{{Resource: "storageclasses", Verb: "list", ClusterScoped: true}},
		func(context.Context, *CollectEnv, *model.EvidenceBundle) {
			t.Fatal("ran a collector the review denied")
		})
	var b model.EvidenceBundle
	parallelCollector{name: "p", children: []EvidenceCollector{forbidden("persistentvolumes"), missing, denied}}.Collect(context.Background(), env, &b)
	got := map[string]string{}
	for _, s := range env.coverageList() {
		got[s.Collector] = s.Status
	}
	want := map[string]string{"persistentvolumes": StatusNoPermission, "gateways": StatusNotInstalled, "storageclasses": StatusNoPermission}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q (all: %v)", k, got[k], v, got)
		}
	}
	warn := strings.Join(coverageWarning(env.coverageList()), "\n")
	if !strings.Contains(warn, "Not checked, no permission: persistentvolumes") || !strings.Contains(warn, "storageclasses (needs list storageclasses)") || strings.Contains(warn, "gateways") {
		t.Errorf("warning = %q", warn)
	}
}

func TestPersistentVolumesKeptOnlyForTheNamespace(t *testing.T) {
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "persistentvolumes"}
	pv := func(name, claimNS string) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "PersistentVolume",
			"metadata": map[string]any{"name": name}, "spec": map[string]any{"claimRef": map[string]any{"namespace": claimNS}}}}
	}
	fake := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{gvr: "PersistentVolumeList"}, pv("mine", "ns"), pv("theirs", "other"))
	var spec ResourceCollectorSpec
	for _, c := range resourceCollectors() {
		if c.Name() == "persistentvolumes" {
			spec = c.(*resourceCollector).spec
		}
	}
	c, _ := NewResourceCollector(spec)
	c.(*resourceCollector).newDynamic = func(*Client) (dynamic.Interface, error) { return fake, nil }
	var b model.EvidenceBundle
	c.Collect(context.Background(), fakeEnv(), &b)
	if got := b.Resources["persistentvolumes"]; len(got) != 1 || got[0]["metadata"].(map[string]any)["name"] != "mine" {
		t.Fatalf("kept %v", got)
	}
}

func TestDefaultCollectorNamesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, n := range DefaultCollectorNames() {
		if seen[n] {
			t.Fatalf("duplicate collector name %q", n)
		}
		seen[n] = true
	}
	if !seen["persistentvolumeclaims"] || !seen["clustercatalogs"] || !seen["pods"] {
		t.Fatalf("names = %v", DefaultCollectorNames())
	}
}

func TestPanickingCollectorBecomesAWarning(t *testing.T) {
	boom := NewCollector("boom", func(context.Context, *CollectEnv, *model.EvidenceBundle) { panic("nil map") })
	ok := NewCollector("ok", func(_ context.Context, _ *CollectEnv, b *model.EvidenceBundle) {
		b.Resources = map[string][]map[string]any{"ok": {}}
	})
	var b model.EvidenceBundle
	parallelCollector{name: "p", children: []EvidenceCollector{boom, ok}}.Collect(context.Background(), fakeEnv(), &b)
	if len(b.Warnings) != 1 || b.Warnings[0] != "collector boom failed and was skipped: nil map" || b.Resources["ok"] == nil {
		t.Fatalf("warnings = %v, resources = %v", b.Warnings, b.Resources)
	}
}

func TestConfigMapsKeepKeysNotValues(t *testing.T) {
	cm := map[string]any{"metadata": map[string]any{"name": "app"},
		"data": map[string]any{"DB_HOST": "db", "API_TOKEN": "s3cr3t"}, "binaryData": map[string]any{"cert": "AAAA"}}
	keysOnly(cm)
	if cm["data"] != nil || cm["binaryData"] != nil {
		t.Fatalf("values kept: %v", cm)
	}
	if got := cm["keys"].([]string); len(got) != 3 || got[0] != "API_TOKEN" || got[2] != "cert" {
		t.Fatalf("keys = %v", got)
	}
}

func TestUserCollectorsCannotReadConfigMapValues(t *testing.T) {
	if _, err := NewResourceCollector(ResourceCollectorSpec{Name: "cm", Version: "v1", Resource: "configmaps"}); err == nil {
		t.Fatal("user collector for configmaps accepted")
	}
}
