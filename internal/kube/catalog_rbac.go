package kube

import (
	"context"
	"errors"
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
	catalogListConcurrency  = 2
	catalogCountLimit       = 500
	catalogRequestTimeout   = 12 * time.Second
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

func countResource(ctx context.Context, dyn dynamic.Interface, d DiscoveredResource, namespace string, allNamespaces bool) *model.ResourceCount {
	reqCtx, cancel := context.WithTimeout(ctx, catalogRequestTimeout)
	defer cancel()

	gvr := schema.GroupVersionResource{Group: d.Group, Version: d.Version, Resource: d.Resource}
	var res dynamic.ResourceInterface
	if d.Namespaced {
		if allNamespaces {
			res = dyn.Resource(gvr)
		} else {
			res = dyn.Resource(gvr).Namespace(namespace)
		}
	} else {
		res = dyn.Resource(gvr)
	}
	ul, err := res.List(reqCtx, metav1.ListOptions{Limit: catalogCountLimit})
	if err != nil {
		if apierrors.IsForbidden(err) {
			return &model.ResourceCount{State: "forbidden"}
		}
		if apierrors.IsNotFound(err) {
			return &model.ResourceCount{State: "unavailable"}
		}
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || reqCtx.Err() != nil {
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

func countResourceMulti(ctx context.Context, dyn dynamic.Interface, d DiscoveredResource, namespaces []string) *model.ResourceCount {
	total := 0
	allowed := 0
	for _, ns := range namespaces {
		if ctx.Err() != nil {
			break
		}
		c := countResource(ctx, dyn, d, ns, false)
		if c == nil {
			continue
		}
		switch c.State {
		case "loaded":
			allowed++
			total += c.Count
		case "forbidden":
			continue
		default:
			continue
		}
	}
	if allowed == 0 {
		return &model.ResourceCount{State: "forbidden"}
	}
	return &model.ResourceCount{State: "loaded", Count: total}
}

func shouldAttemptCatalogCount(d model.KubernetesResourceDescriptor) bool {
	if !IsBrowsableCatalogResource(d.Resource) {
		return false
	}
	if !descriptorSupportsList(d) {
		return false
	}
	if d.AccessState == model.ResourceAccessForbidden {
		return false
	}
	if d.Permissions.List != nil && !*d.Permissions.List {
		return false
	}
	return true
}

func attachCounts(ctx context.Context, client *Client, namespace string, descriptors []model.KubernetesResourceDescriptor, allNamespaces bool, namespaces []string) []model.KubernetesResourceDescriptor {
	if client == nil {
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
				if !shouldAttemptCatalogCount(d) {
					continue
				}
				scope := catalogEntityScopeForDescriptor(d, namespace, allNamespaces, namespaces)
				out[j.idx].Count = CountCatalogEntities(ctx, client, d.ID, scope)
				if out[j.idx].Count != nil && out[j.idx].Count.State == "forbidden" {
					out[j.idx].AccessState = model.ResourceAccessForbidden
				}
			}
		}()
	}

	for i, d := range descriptors {
		if shouldAttemptCatalogCount(d) {
			jobs <- job{idx: i}
		}
	}
	close(jobs)
	wg.Wait()
	return out
}

// BuildResourceCatalog discovers API resources and evaluates RBAC for the namespace scope.
func BuildResourceCatalog(ctx context.Context, client *Client, namespace string, includeCounts bool, allNamespaces bool, namespaces []string) (model.ResourceCatalog, error) {
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
	authNS := namespace
	if allNamespaces || len(namespaces) > 1 {
		authNS = ""
	}
	nsRules, clRules, authErr := resolveAuthRules(ctx, client, authNS)
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
		descriptors = attachCounts(ctx, client, namespace, descriptors, allNamespaces, namespaces)
	}

	namespaced, extensions, cluster := partitionCatalog(descriptors)
	catalogNS := namespace
	if allNamespaces {
		catalogNS = "*"
	} else if len(namespaces) > 1 {
		catalogNS = ""
	}
	catalog := model.ResourceCatalog{
		Context:             client.Context,
		Cluster:             client.Cluster,
		Namespace:           catalogNS,
		AllNamespaces:       allNamespaces,
		Namespaces:          append([]string(nil), namespaces...),
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
func RefreshResourceCatalog(ctx context.Context, client *Client, namespace string, includeCounts bool, allNamespaces bool, namespaces []string) (model.ResourceCatalog, error) {
	InvalidateCatalogDiscovery(client)
	InvalidateCatalogAuth(client)
	return BuildResourceCatalog(ctx, client, namespace, includeCounts, allNamespaces, namespaces)
}
