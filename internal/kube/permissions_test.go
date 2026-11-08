package kube

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

// reviewer answers access reviews: denied resources are refused, broken ones
// fail, and every review's attributes are recorded.
func reviewer(t *testing.T, denied, broken map[string]bool) (*fake.Clientset, func() []authorizationv1.ResourceAttributes) {
	t.Helper()
	cs := fake.NewSimpleClientset()
	var mu sync.Mutex
	var seen []authorizationv1.ResourceAttributes
	cs.PrependReactor("create", "selfsubjectaccessreviews", func(a k8stesting.Action) (bool, runtime.Object, error) {
		review := a.(k8stesting.CreateAction).GetObject().(*authorizationv1.SelfSubjectAccessReview)
		attrs := *review.Spec.ResourceAttributes
		mu.Lock()
		seen = append(seen, attrs)
		mu.Unlock()
		if broken[attrs.Resource] {
			return true, nil, fmt.Errorf("boom")
		}
		review.Status.Allowed = !denied[attrs.Resource]
		return true, review, nil
	})
	return cs, func() []authorizationv1.ResourceAttributes {
		mu.Lock()
		defer mu.Unlock()
		return append([]authorizationv1.ResourceAttributes(nil), seen...)
	}
}

func TestCheckPermissionsSendsGroupsAndReportsDenied(t *testing.T) {
	cs, seen := reviewer(t, map[string]bool{"persistentvolumeclaims": true}, map[string]bool{"configmaps": true})
	checks, warnings := CheckPermissions(context.Background(), &Client{Clientset: cs, Context: t.Name()}, "shop")
	if len(checks) != len(scopeAccess) {
		t.Fatalf("checks = %d", len(checks))
	}
	for i, a := range scopeAccess {
		c := checks[i]
		if c.Group != a.Group || c.Verb != a.Verb {
			t.Fatalf("checks[%d] = %+v, want %s", i, c, a)
		}
		wantNS := "shop"
		if a.ClusterScoped {
			wantNS = ""
		}
		if c.Namespace != wantNS {
			t.Fatalf("%s namespace = %q", a, c.Namespace)
		}
	}
	for _, attrs := range seen() {
		if attrs.Resource == "deployments" && attrs.Group != "apps" {
			t.Fatalf("deployments reviewed in group %q; RBAC for apps/deployments would not match", attrs.Group)
		}
		if attrs.Resource == "pods" && attrs.Verb == "get" && attrs.Subresource == "" {
			continue
		}
		if attrs.Subresource == "log" && attrs.Resource != "pods" {
			t.Fatalf("log review = %+v", attrs)
		}
	}
	joined := strings.Join(warnings, "\n")
	if !strings.Contains(joined, "Missing permission: list persistentvolumeclaims") || !strings.Contains(joined, "permission check failed for list configmaps") {
		t.Fatalf("warnings = %v", warnings)
	}
}

func TestAllowedTreatsUnreviewableAsTry(t *testing.T) {
	cs, _ := reviewer(t, map[string]bool{"nodes": true}, map[string]bool{"events": true})
	checks, _ := CheckPermissions(context.Background(), &Client{Clientset: cs, Context: t.Name()}, "shop")
	if allowed(checks, "nodes", "list") {
		t.Error("denied nodes list reported as allowed")
	}
	if !allowed(checks, "events", "list") {
		t.Error("an access review that failed to run must not hide events: try them")
	}
	if !allowed(checks, "pods/log", "get") {
		t.Error("pods/log get not found")
	}
}

func TestAccessReviewsAreCached(t *testing.T) {
	cs, seen := reviewer(t, nil, nil)
	c := &Client{Clientset: cs, Context: t.Name()}
	CheckPermissions(context.Background(), c, "shop")
	first := len(seen())
	CheckPermissions(context.Background(), c, "shop")
	if len(seen()) != first {
		t.Fatalf("second check made %d more reviews, want 0 (cached)", len(seen())-first)
	}
}
