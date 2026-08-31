package kube

import (
	"context"
	"fmt"
	"sync"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/glnreddy421/klew/internal/model"
)

const (
	catalogListConcurrency = 8
	catalogCountLimit      = 500
)

type rulesIndex struct {
	rules []authorizationv1.ResourceRule
}

func newRulesIndex(rules []authorizationv1.ResourceRule) *rulesIndex {
	return &rulesIndex{rules: rules}
}

func (r *rulesIndex) allows(group, resource, verb string) bool {
	for _, rule := range r.rules {
		if !verbAllowed(rule.Verbs, verb) {
			continue
		}
		if !groupAllowed(rule.APIGroups, group) {
			continue
		}
		if !resourceAllowed(rule.Resources, resource) {
			continue
		}
		return true
	}
	return false
}

func verbAllowed(verbs []string, verb string) bool {
	for _, v := range verbs {
		if v == "*" || v == verb {
			return true
		}
	}
	return false
}

func groupAllowed(groups []string, group string) bool {
	for _, g := range groups {
		if g == "*" || g == group {
			return true
		}
	}
	return false
}

func resourceAllowed(resources []string, resource string) bool {
	for _, r := range resources {
		if r == "*" || r == resource {
			return true
		}
	}
	return false
}

func permissionsFromRules(rules *rulesIndex, group, resource string) model.ResourcePermissions {
	if rules == nil {
		return model.ResourcePermissions{}
	}
	get := rules.allows(group, resource, "get")
	list := rules.allows(group, resource, "list")
	watch := rules.allows(group, resource, "watch")
	return model.ResourcePermissions{
		Get:   &get,
		List:  &list,
		Watch: &watch,
	}
}

func accessStateFromPermissions(perms model.ResourcePermissions) model.ResourceAccessState {
	if perms.List != nil && *perms.List {
		return model.ResourceAccessAllowed
	}
	if perms.Get != nil && *perms.Get {
		return model.ResourceAccessAllowed
	}
	if perms.List != nil && !*perms.List && perms.Get != nil && !*perms.Get {
		return model.ResourceAccessForbidden
	}
	return model.ResourceAccessUnknown
}

func fetchRulesReview(ctx context.Context, client *Client, namespace string) (*rulesIndex, error) {
	auth := client.Clientset.AuthorizationV1()
	review := &authorizationv1.SelfSubjectRulesReview{
		Spec: authorizationv1.SelfSubjectRulesReviewSpec{
			Namespace: namespace,
		},
	}
	result, err := auth.SelfSubjectRulesReviews().Create(ctx, review, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return newRulesIndex(result.Status.ResourceRules), nil
}

type authCache struct {
	mu    sync.RWMutex
	items map[string]cachedAuth
}

type cachedAuth struct {
	namespaced *rulesIndex
	cluster    *rulesIndex
	fetchedAt  time.Time
}

func newAuthCache() *authCache {
	return &authCache{items: map[string]cachedAuth{}}
}

func (c *authCache) key(client *Client, namespace string) string {
	return client.Context + "|" + client.Cluster + "|" + namespace
}

func (c *authCache) get(client *Client, namespace string) (*rulesIndex, *rulesIndex, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.items[c.key(client, namespace)]
	if !ok || time.Since(entry.fetchedAt) > 2*time.Minute {
		return nil, nil, false
	}
	return entry.namespaced, entry.cluster, true
}

func (c *authCache) set(client *Client, namespace string, namespaced, cluster *rulesIndex) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[c.key(client, namespace)] = cachedAuth{
		namespaced: namespaced,
		cluster:    cluster,
		fetchedAt:  time.Now(),
	}
}

func (c *authCache) invalidate(client *Client) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.items {
		prefix := client.Context + "|" + client.Cluster + "|"
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(c.items, k)
		}
	}
}

var globalAuthCache = newAuthCache()

// InvalidateCatalogAuth drops cached authorization for a cluster context.
func InvalidateCatalogAuth(client *Client) {
	if client == nil {
		return
	}
	globalAuthCache.invalidate(client)
}

func resolveAuthRules(ctx context.Context, client *Client, namespace string) (namespaced, cluster *rulesIndex, err error) {
	if nsRules, clRules, ok := globalAuthCache.get(client, namespace); ok {
		return nsRules, clRules, nil
	}
	nsRules, err := fetchRulesReview(ctx, client, namespace)
	if err != nil {
		return nil, nil, err
	}
	clRules, err := fetchRulesReview(ctx, client, "")
	if err != nil {
		return nil, nil, err
	}
	globalAuthCache.set(client, namespace, nsRules, clRules)
	return nsRules, clRules, nil
}

func countResource(ctx context.Context, dyn dynamic.Interface, d DiscoveredResource, namespace string) *model.ResourceCount {
	gvr := schema.GroupVersionResource{Group: d.Group, Version: d.Version, Resource: d.Resource}
	var res dynamic.ResourceInterface
	if d.Namespaced {
		res = dyn.Resource(gvr).Namespace(namespace)
	} else {
		res = dyn.Resource(gvr)
	}
	ul, err := res.List(ctx, metav1.ListOptions{Limit: catalogCountLimit})
	if err != nil {
		if apierrors.IsForbidden(err) {
			return &model.ResourceCount{State: "forbidden"}
		}
		if apierrors.IsNotFound(err) {
			return &model.ResourceCount{State: "unavailable"}
		}
		if ctx.Err() != nil {
			return &model.ResourceCount{State: "unavailable"}
		}
		return &model.ResourceCount{State: "error", Error: err.Error()}
	}
	count := len(ul.Items)
	if ul.GetContinue() != "" {
		count = catalogCountLimit
	}
	return &model.ResourceCount{State: "loaded", Count: count}
}

func attachCounts(ctx context.Context, client *Client, namespace string, descriptors []model.KubernetesResourceDescriptor) []model.KubernetesResourceDescriptor {
	dyn, err := dynamicClient(client)
	if err != nil {
		return descriptors
	}
	type job struct {
		idx int
	}
	jobs := make(chan job)
	var wg sync.WaitGroup
	out := append([]model.KubernetesResourceDescriptor(nil), descriptors...)

	for w := 0; w < catalogListConcurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				if ctx.Err() != nil {
					return
				}
				d := descriptors[j.idx]
				if d.AccessState != model.ResourceAccessAllowed {
					continue
				}
				if d.Permissions.List != nil && !*d.Permissions.List {
					continue
				}
				disc := DiscoveredResource{
					Group: d.Group, Version: d.Version, Resource: d.Resource, Kind: d.Kind, Namespaced: d.Namespaced,
				}
				ns := namespace
				if !d.Namespaced {
					ns = ""
				}
				out[j.idx].Count = countResource(ctx, dyn, disc, ns)
				if out[j.idx].Count != nil && out[j.idx].Count.State == "forbidden" {
					out[j.idx].AccessState = model.ResourceAccessForbidden
				}
			}
		}()
	}

	for i, d := range descriptors {
		if d.AccessState == model.ResourceAccessAllowed && d.Permissions.List != nil && *d.Permissions.List {
			jobs <- job{idx: i}
		}
	}
	close(jobs)
	wg.Wait()
	return out
}

// BuildResourceCatalog discovers API resources and evaluates RBAC for the namespace scope.
func BuildResourceCatalog(ctx context.Context, client *Client, namespace string, includeCounts bool) (model.ResourceCatalog, error) {
	if client == nil || client.Clientset == nil {
		return model.ResourceCatalog{}, fmt.Errorf("kubernetes client is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	start := time.Now()
	discovered, failedGroups, err := discoverAPIResources(client)
	if err != nil {
		return model.ResourceCatalog{}, err
	}
	discoveryMs := time.Since(start).Milliseconds()

	authStart := time.Now()
	nsRules, clRules, authErr := resolveAuthRules(ctx, client, namespace)
	authMs := time.Since(authStart).Milliseconds()

	var descriptors []model.KubernetesResourceDescriptor
	for _, d := range discovered {
		if ctx.Err() != nil {
			break
		}
		var rules *rulesIndex
		if d.Namespaced {
			rules = nsRules
		} else {
			rules = clRules
		}
		perms := permissionsFromRules(rules, d.Group, d.Resource)
		access := accessStateFromPermissions(perms)
		descriptors = append(descriptors, toDescriptor(d, perms, access, nil))
	}

	if includeCounts && authErr == nil {
		descriptors = attachCounts(ctx, client, namespace, descriptors)
	}

	namespaced, extensions, cluster := partitionCatalog(descriptors)
	catalog := model.ResourceCatalog{
		Context:             client.Context,
		Cluster:             client.Cluster,
		Namespace:           namespace,
		GeneratedAt:         time.Now(),
		DiscoveryDurationMs: discoveryMs,
		AuthDurationMs:      authMs,
		Resources:           descriptors,
		Namespaced:          namespaced,
		Extensions:          extensions,
		ClusterScoped:       cluster,
		FailedGroups:        failedGroups,
	}
	return catalog, nil
}

// RefreshResourceCatalog invalidates caches and rebuilds the catalog.
func RefreshResourceCatalog(ctx context.Context, client *Client, namespace string, includeCounts bool) (model.ResourceCatalog, error) {
	InvalidateCatalogDiscovery(client)
	InvalidateCatalogAuth(client)
	return BuildResourceCatalog(ctx, client, namespace, includeCounts)
}
