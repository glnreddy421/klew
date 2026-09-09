package kube

import (
	"testing"

	"github.com/glnreddy421/klew/internal/model"
)

func TestIsBrowsableCatalogResource(t *testing.T) {
	if IsBrowsableCatalogResource("subjectaccessreviews") {
		t.Fatal("expected subjectaccessreviews to be non-browsable")
	}
	if !IsBrowsableCatalogResource("servicecidrs") {
		t.Fatal("expected servicecidrs to be browsable")
	}
}

func TestCatalogEntityScopeForDescriptor(t *testing.T) {
	desc := model.KubernetesResourceDescriptor{
		ID:         "authorization.k8s.io/v1/subjectaccessreviews",
		Group:      "authorization.k8s.io",
		Version:    "v1",
		Resource:   "subjectaccessreviews",
		Kind:       "SubjectAccessReview",
		Namespaced: true,
	}
	scope := catalogEntityScopeForDescriptor(desc, "klew-lab", true, nil)
	if !scope.AllNamespaces || scope.ClusterScoped {
		t.Fatalf("expected all-namespaces namespaced scope, got %+v", scope)
	}

	clusterDesc := model.KubernetesResourceDescriptor{
		ID:         "networking.k8s.io/v1/servicecidrs",
		Namespaced: false,
	}
	clusterScope := catalogEntityScopeForDescriptor(clusterDesc, "klew-lab", true, nil)
	if !clusterScope.ClusterScoped {
		t.Fatalf("expected cluster scoped scope, got %+v", clusterScope)
	}
}

func TestDescriptorSupportsList(t *testing.T) {
	list := true
	if !descriptorSupportsList(model.KubernetesResourceDescriptor{
		SupportedVerbs: []string{"create"},
		Permissions:    model.ResourcePermissions{List: &list},
	}) {
		t.Fatal("expected permissions.list to allow counting")
	}
	if descriptorSupportsList(model.KubernetesResourceDescriptor{
		SupportedVerbs: []string{"create"},
	}) {
		t.Fatal("expected missing list verb to skip counting")
	}
}
