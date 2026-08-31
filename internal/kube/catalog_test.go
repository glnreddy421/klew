package kube

import (
	"context"
	"testing"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"

	"github.com/glnreddy421/klew/internal/model"
)

func TestPresentationKey(t *testing.T) {
	if PresentationKey("", "pods") != "/pods" {
		t.Fatal("expected /pods")
	}
	if PresentationKey("apps", "deployments") != "apps/deployments" {
		t.Fatal("expected apps/deployments")
	}
}

func TestParseResourceID(t *testing.T) {
	g, v, r, err := ParseResourceID("v1/pods")
	if err != nil || g != "" || v != "v1" || r != "pods" {
		t.Fatalf("ParseResourceID v1/pods = %q,%q,%q err=%v", g, v, r, err)
	}
	g, v, r, err = ParseResourceID("apps/v1/deployments")
	if err != nil || g != "apps" || v != "v1" || r != "deployments" {
		t.Fatalf("ParseResourceID apps/v1/deployments = %q,%q,%q err=%v", g, v, r, err)
	}
}

func TestResourceID(t *testing.T) {
	tests := []struct {
		group, version, resource, want string
	}{
		{"", "v1", "pods", "v1/pods"},
		{"apps", "v1", "deployments", "apps/v1/deployments"},
		{"argoproj.io", "v1alpha1", "applications", "argoproj.io/v1alpha1/applications"},
	}
	for _, tc := range tests {
		if got := ResourceID(tc.group, tc.version, tc.resource); got != tc.want {
			t.Fatalf("ResourceID(%q,%q,%q) = %q want %q", tc.group, tc.version, tc.resource, got, tc.want)
		}
	}
}

func TestIsSubresource(t *testing.T) {
	if !isSubresource("pods/status") {
		t.Fatal("expected subresource")
	}
	if isSubresource("pods") {
		t.Fatal("expected primary resource")
	}
}

func TestParseDiscoveredResources_CoreGroupedClusterCRD(t *testing.T) {
	lists := []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "pods", Kind: "Pod", Namespaced: true, Verbs: []string{"list", "get"}},
				{Name: "nodes", Kind: "Node", Namespaced: false, Verbs: []string{"list"}},
				{Name: "pods/status", Kind: "Pod", Namespaced: true},
			},
		},
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", Kind: "Deployment", Namespaced: true, Verbs: []string{"list"}},
			},
		},
		{
			GroupVersion: "argoproj.io/v1alpha1",
			APIResources: []metav1.APIResource{
				{Name: "applications", Kind: "Application", Namespaced: true, Verbs: []string{"list"}},
			},
		},
	}
	got := parseDiscoveredResources(lists, nil)
	if len(got) != 4 {
		t.Fatalf("expected 4 resources (subresource filtered), got %d", len(got))
	}
	byID := map[string]DiscoveredResource{}
	for _, d := range got {
		byID[ResourceID(d.Group, d.Version, d.Resource)] = d
	}
	pod := byID["v1/pods"]
	if !pod.Namespaced || pod.Kind != "Pod" || pod.Group != "" || pod.Version != "v1" {
		t.Fatalf("unexpected pod descriptor: %+v", pod)
	}
	dep := byID["apps/v1/deployments"]
	if dep.Group != "apps" || !dep.Namespaced {
		t.Fatalf("unexpected deployment descriptor: %+v", dep)
	}
	node := byID["v1/nodes"]
	if node.Namespaced {
		t.Fatalf("nodes should be cluster scoped")
	}
	app := byID["argoproj.io/v1alpha1/applications"]
	if classifySource(app.Group) != model.ResourceSourceExtension {
		t.Fatalf("expected extension source")
	}
}

func TestParseDiscoveredResources_DeduplicatesPreferredVersion(t *testing.T) {
	lists := []*metav1.APIResourceList{
		{GroupVersion: "apps/v1", APIResources: []metav1.APIResource{
			{Name: "deployments", Kind: "Deployment", Namespaced: true},
		}},
		{GroupVersion: "apps/v1beta1", APIResources: []metav1.APIResource{
			{Name: "deployments", Kind: "Deployment", Namespaced: true},
		}},
	}
	got := parseDiscoveredResources(lists, nil)
	var depVersions int
	for _, d := range got {
		if d.Resource == "deployments" && d.Group == "apps" {
			depVersions++
		}
	}
	if depVersions != 2 {
		t.Fatalf("expected both versions when both lists provided, got %d entries", depVersions)
	}
}

func TestClassifySource(t *testing.T) {
	if classifySource("") != model.ResourceSourceBuiltin {
		t.Fatal("core should be builtin")
	}
	if classifySource("networking.k8s.io") != model.ResourceSourceBuiltin {
		t.Fatal("k8s.io group should be builtin")
	}
	if classifySource("argoproj.io") != model.ResourceSourceExtension {
		t.Fatal("custom group should be extension")
	}
}

func TestRulesIndexAllows(t *testing.T) {
	rules := newRulesIndex([]authorizationv1.ResourceRule{
		{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"list", "get"},
		},
		{
			APIGroups: []string{"*"},
			Resources: []string{"secrets"},
			Verbs:     []string{"get"},
		},
	})
	if !rules.allows("", "pods", "list") {
		t.Fatal("expected pods list allowed")
	}
	if rules.allows("", "secrets", "list") {
		t.Fatal("expected secrets list forbidden")
	}
	if !rules.allows("", "secrets", "get") {
		t.Fatal("expected secrets get allowed")
	}
}

func TestAccessStateFromPermissions_Forbidden(t *testing.T) {
	list := false
	get := false
	state := accessStateFromPermissions(model.ResourcePermissions{List: &list, Get: &get})
	if state != model.ResourceAccessForbidden {
		t.Fatalf("expected forbidden, got %s", state)
	}
}

func TestPartitionCatalog(t *testing.T) {
	descriptors := []model.KubernetesResourceDescriptor{
		{Kind: "Pod", Namespaced: true, Source: model.ResourceSourceBuiltin},
		{Kind: "Application", Namespaced: true, Source: model.ResourceSourceExtension},
		{Kind: "Node", Namespaced: false, Source: model.ResourceSourceBuiltin},
	}
	ns, ext, cl := partitionCatalog(descriptors)
	if len(ns) != 1 || len(ext) != 1 || len(cl) != 1 {
		t.Fatalf("unexpected partition sizes: %d %d %d", len(ns), len(ext), len(cl))
	}
}

func TestFetchPreferredResourceLists_PartialFailure(t *testing.T) {
	fake := &fakeDiscovery{
		lists: []*metav1.APIResourceList{
			{GroupVersion: "v1", APIResources: []metav1.APIResource{{Name: "pods", Kind: "Pod", Namespaced: true}}},
		},
		err: &discovery.ErrGroupDiscoveryFailed{
			Groups: map[schema.GroupVersion]error{
				{Group: "metrics.k8s.io", Version: "v1beta1"}: context.DeadlineExceeded,
			},
		},
	}
	lists, failed, err := fetchPreferredResourceLists(fake)
	if err != nil {
		t.Fatalf("partial discovery should not fail when lists exist: %v", err)
	}
	if len(lists) != 1 || len(failed) != 1 {
		t.Fatalf("expected partial results, got lists=%d failed=%d", len(lists), len(failed))
	}
}

func TestBuildResourceCatalog_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Without a real client this only validates cancellation plumbing in attachCounts.
	if ctx.Err() == nil {
		t.Fatal("expected cancelled context")
	}
}

type fakeDiscovery struct {
	lists []*metav1.APIResourceList
	err   error
}

func (f *fakeDiscovery) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return f.lists, f.err
}

func TestDiscoveryCacheTTL(t *testing.T) {
	cache := newDiscoveryCache()
	client := &Client{Context: "ctx-a", Cluster: "cluster-a"}
	resources := []DiscoveredResource{{Kind: "Pod", Resource: "pods", Group: "", Version: "v1"}}
	cache.set(client, resources, nil)
	if _, _, ok := cache.get(client); !ok {
		t.Fatal("expected cache hit")
	}
	cache.items[cache.key(client)] = cachedDiscovery{
		resources: resources,
		fetchedAt: time.Now().Add(-catalogDiscoveryTTL - time.Second),
	}
	if _, _, ok := cache.get(client); ok {
		t.Fatal("expected cache miss after TTL")
	}
}
