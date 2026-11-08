package kube

import (
	"context"
	"fmt"

	"github.com/glnreddy421/klew/v2/internal/model"
)

// scopeAccess is what resolving the scope and the pod-centric collectors read.
// Each names its API group: a review without one asks about the core group,
// which a Role granting apps/deployments does not cover.
var scopeAccess = []Access{
	{Group: "apps", Resource: "deployments", Verb: "get"},
	{Group: "apps", Resource: "deployments", Verb: "list"},
	{Group: "apps", Resource: "deployments", Verb: "watch"},
	{Group: "apps", Resource: "replicasets", Verb: "list"},
	{Group: "apps", Resource: "replicasets", Verb: "watch"},
	{Resource: "pods", Verb: "list"},
	{Resource: "pods", Verb: "get"},
	{Resource: "pods", Verb: "watch"},
	{Resource: "pods", Subresource: "log", Verb: "get"},
	{Resource: "services", Verb: "list"},
	{Resource: "services", Verb: "watch"},
	{Group: "discovery.k8s.io", Resource: "endpointslices", Verb: "list"},
	{Resource: "events", Verb: "list"},
	{Resource: "events", Verb: "watch"},
	{Group: "networking.k8s.io", Resource: "ingresses", Verb: "list"},
	{Group: "autoscaling", Resource: "horizontalpodautoscalers", Verb: "list"},
	{Group: "batch", Resource: "jobs", Verb: "list"},
	{Resource: "nodes", Verb: "list", ClusterScoped: true},
	{Resource: "configmaps", Verb: "list"},
	{Resource: "persistentvolumeclaims", Verb: "list"},
}

// CheckPermissions reviews scopeAccess in namespace, in order, and describes
// each denied or unreviewable permission.
func CheckPermissions(ctx context.Context, c *Client, namespace string) ([]model.PermissionCheck, []string) {
	results := checkAccess(ctx, c, namespace, scopeAccess)
	checks := make([]model.PermissionCheck, 0, len(scopeAccess))
	var notes []string
	for _, a := range scopeAccess {
		check := results[a.key(namespace)]
		checks = append(checks, check)
		switch {
		case check.Error != "":
			notes = append(notes, fmt.Sprintf("permission check failed for %s: %s", a, check.Error))
		case !check.Allowed:
			notes = append(notes, fmt.Sprintf("Missing permission: %s (namespace=%s)", a, check.Namespace))
		}
	}
	return checks, notes
}

// allowed reports whether verb on resource may be tried: allowed, or unknown
// because the review itself failed.
func allowed(checks []model.PermissionCheck, resource, verb string) bool {
	for _, c := range checks {
		if c.Resource == resource && c.Verb == verb {
			return c.Allowed || c.Error != ""
		}
	}
	return false
}
