package kube

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"
)

func TestListTargetsGroupsBuiltinAndCustomKinds(t *testing.T) {
	cs := fake.NewSimpleClientset(
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "shop"}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "shop"}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "elsewhere", Namespace: "other"}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "shop"}},
	)
	gvr := schema.GroupVersionResource{Group: "cert-manager.io", Version: "v1", Resource: "certificates"}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{gvr: "CertificateList"},
		&unstructured.Unstructured{Object: map[string]any{"apiVersion": "cert-manager.io/v1", "kind": "Certificate",
			"metadata": map[string]any{"name": "web-tls", "namespace": "shop"}}})
	certs, err := NewResourceCollector(ResourceCollectorSpec{Name: "certificates", Group: gvr.Group, Version: gvr.Version, Resource: gvr.Resource})
	if err != nil {
		t.Fatal(err)
	}
	certs.(*resourceCollector).newDynamic = func(*Client) (dynamic.Interface, error) { return dyn, nil }

	allowAll(cs)
	got := ListTargets(context.Background(), &Client{Clientset: cs, Config: &rest.Config{Host: "http://127.0.0.1:1"}, Context: t.Name()}, "shop", []EvidenceCollector{certs}).Groups
	byKind := map[string][]string{}
	var order []string
	custom := map[string]bool{}
	for _, g := range got {
		byKind[g.Kind] = g.Names
		order = append(order, g.Kind)
		custom[g.Kind] = g.Custom
	}
	if len(byKind["Deployment"]) != 2 || byKind["Deployment"][0] != "api" {
		t.Errorf("deployments = %v", byKind["Deployment"])
	}
	if len(byKind["Service"]) != 1 {
		t.Errorf("services = %v", byKind["Service"])
	}
	if len(byKind["Certificate"]) != 1 || !custom["Certificate"] || custom["Deployment"] {
		t.Errorf("certificates = %v (custom %v)", byKind["Certificate"], custom)
	}
	if order[0] != "Deployment" || order[len(order)-1] != "Certificate" {
		t.Errorf("order = %v, want workloads first and custom kinds last", order)
	}
}

func allowAll(cs *fake.Clientset, deny ...string) {
	denied := map[string]bool{}
	for _, d := range deny {
		denied[d] = true
	}
	cs.PrependReactor("create", "selfsubjectaccessreviews", func(a k8stesting.Action) (bool, runtime.Object, error) {
		review := a.(k8stesting.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)
		review.Status.Allowed = !denied[review.Spec.ResourceAttributes.Resource]
		return true, review, nil
	})
}

func TestListTargetsChecksAccessFirstAndSaysWhy(t *testing.T) {
	cs := fake.NewSimpleClientset(&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "shop"}})
	allowAll(cs, "deployments", "jobs")
	listed := map[string]bool{}
	cs.PrependReactor("list", "*", func(a k8stesting.Action) (bool, runtime.Object, error) {
		listed[a.GetResource().Resource] = true
		return false, nil, nil
	})
	got := ListTargets(context.Background(), &Client{Clientset: cs, Config: &rest.Config{Host: "http://127.0.0.1:1"}, Context: t.Name()}, "shop", nil)
	if listed["deployments"] || listed["jobs"] {
		t.Fatalf("listed kinds the review denied: %v", listed)
	}
	reasons := map[string]string{}
	for _, u := range got.Unavailable {
		reasons[u.Collector] = u.Status + " " + u.Detail
	}
	if reasons["workloads"] != "no-permission needs list deployments.apps" || reasons["jobs"] != "no-permission needs list jobs.batch" {
		t.Fatalf("unavailable = %v", got.Unavailable)
	}
	if len(got.Groups) == 0 || got.Groups[0].Kind != "Service" {
		t.Fatalf("groups = %+v", got.Groups)
	}
}
