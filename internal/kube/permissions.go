package kube

import (
	"context"
	"fmt"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/glnreddy421/klew/internal/model"
)

// requiredPermissions are probed before collection.
var requiredPermissions = []struct {
	Resource  string
	Verb      string
	Namespace bool
}{
	{"deployments", "get", true},
	{"deployments", "list", true},
	{"deployments", "watch", true},
	{"replicasets", "list", true},
	{"replicasets", "watch", true},
	{"pods", "list", true},
	{"pods", "get", true},
	{"pods", "watch", true},
	{"pods/log", "get", true},
	{"services", "list", true},
	{"services", "watch", true},
	{"endpointslices", "list", true},
	{"events", "list", true},
	{"events", "watch", true},
	{"ingresses", "list", true},
	{"horizontalpodautoscalers", "list", true},
	{"nodes", "list", false},
	{"configmaps", "list", true},
	{"secrets", "list", true},
	{"persistentvolumeclaims", "list", true},
}

// CheckPermissions runs SelfSubjectAccessReview for read operations.
func CheckPermissions(ctx context.Context, c *Client, namespace string) ([]model.PermissionCheck, []string) {
	var checks []model.PermissionCheck
	var warnings []string
	auth := c.Clientset.AuthorizationV1()

	for _, p := range requiredPermissions {
		ns := namespace
		if !p.Namespace {
			ns = ""
		}
		sar := &authorizationv1.SelfSubjectAccessReview{
			Spec: authorizationv1.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &authorizationv1.ResourceAttributes{
					Namespace: ns,
					Verb:      p.Verb,
					Resource:  p.Resource,
				},
			},
		}
		result, err := auth.SelfSubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
		allowed := false
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("permission check failed for %s/%s: %v", p.Resource, p.Verb, err))
		} else if result.Status.Allowed {
			allowed = true
		} else {
			warnings = append(warnings, fmt.Sprintf("Missing permission: %s %s (namespace=%s)", p.Verb, p.Resource, ns))
		}
		checks = append(checks, model.PermissionCheck{
			Resource:  p.Resource,
			Verb:      p.Verb,
			Namespace: ns,
			Allowed:   allowed,
		})
	}
	return checks, warnings
}

func allowed(checks []model.PermissionCheck, resource, verb string) bool {
	for _, c := range checks {
		if c.Resource == resource && c.Verb == verb {
			return c.Allowed
		}
	}
	return false
}
