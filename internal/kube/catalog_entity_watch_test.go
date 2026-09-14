package kube

import "testing"

func TestCatalogEntityWatchSupports(t *testing.T) {
	if CatalogEntityWatchSupports(VirtualHelmReleasesResourceID, CatalogEntityScope{Namespace: "default"}) {
		t.Fatal("virtual resources should not watch")
	}
	if !CatalogEntityWatchSupports("apps/v1/deployments", CatalogEntityScope{Namespace: "prod"}) {
		t.Fatal("single namespace should watch")
	}
	if CatalogEntityWatchSupports("apps/v1/deployments", CatalogEntityScope{AllNamespaces: true}) {
		t.Fatal("all namespaces should poll")
	}
	if CatalogEntityWatchSupports("apps/v1/deployments", CatalogEntityScope{Namespaces: []string{"a", "b"}}) {
		t.Fatal("multi namespace should poll")
	}
	if !CatalogEntityWatchSupports("storage.k8s.io/v1/storageclasses", CatalogEntityScope{ClusterScoped: true}) {
		t.Fatal("cluster scoped should watch")
	}
}
