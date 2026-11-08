package kube

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/glnreddy421/klew/v2/internal/model"
)

// Access is one permission a collector needs, checked with a
// SelfSubjectAccessReview before the collector runs.
type Access struct {
	Group         string
	Resource      string
	Subresource   string
	Verb          string
	ClusterScoped bool
}

func (a Access) String() string {
	r := a.Resource
	if a.Subresource != "" {
		r += "/" + a.Subresource
	}
	if a.Group != "" {
		r += "." + a.Group
	}
	return a.Verb + " " + r
}

func (a Access) key(ns string) string {
	if a.ClusterScoped {
		ns = ""
	}
	return strings.Join([]string{ns, a.Group, a.Resource, a.Subresource, a.Verb}, "|")
}

// AccessDeclarer is a collector that says what it reads. Collectors that do
// not declare access run unconditionally and check access themselves.
type AccessDeclarer interface {
	Access() []Access
}

// Collector statuses recorded in EvidenceBundle.Coverage.
const (
	StatusCollected    = "collected"
	StatusNoPermission = "no-permission"
	StatusNotInstalled = "not-installed"
	StatusFailed       = "failed"
	StatusOff          = "off"
)

const (
	maxConcurrentAccessChecks = 10
	accessCacheTTL            = 2 * time.Minute
)

type accessResult struct {
	allowed bool
	err     string
	at      time.Time
}

// accessCache remembers review results per cluster and user so the periodic
// snapshot refresh does not repeat dozens of reviews.
var accessCache = struct {
	sync.Mutex
	m map[string]accessResult
}{m: map[string]accessResult{}}

// checkAccess reviews each access in ns. A review that fails to run is
// returned with Error set: the access is unknown, not denied.
func checkAccess(ctx context.Context, c *Client, ns string, list []Access) map[string]model.PermissionCheck {
	out := map[string]model.PermissionCheck{}
	if c == nil || c.Clientset == nil {
		return out
	}
	prefix := clientIdentity(c)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentAccessChecks)
	for _, a := range list {
		key := a.key(ns)
		mu.Lock()
		_, seen := out[key]
		if !seen {
			out[key] = model.PermissionCheck{}
		}
		mu.Unlock()
		if seen {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			res := reviewCached(ctx, c, prefix+key, ns, a)
			check := model.PermissionCheck{
				Group: a.Group, Resource: a.Resource, Verb: a.Verb, Allowed: res.allowed, Error: res.err,
			}
			if a.Subresource != "" {
				check.Resource += "/" + a.Subresource
			}
			if !a.ClusterScoped {
				check.Namespace = ns
			}
			mu.Lock()
			out[key] = check
			mu.Unlock()
		}()
	}
	wg.Wait()
	return out
}

func clientIdentity(c *Client) string {
	host := ""
	if c.Config != nil {
		host = c.Config.Host
	}
	return c.Context + "|" + c.User + "|" + host + "|"
}

func reviewCached(ctx context.Context, c *Client, cacheKey, ns string, a Access) accessResult {
	accessCache.Lock()
	if r, ok := accessCache.m[cacheKey]; ok && time.Since(r.at) < accessCacheTTL {
		accessCache.Unlock()
		return r
	}
	accessCache.Unlock()

	attrs := &authorizationv1.ResourceAttributes{
		Group: a.Group, Resource: a.Resource, Subresource: a.Subresource, Verb: a.Verb,
	}
	if !a.ClusterScoped {
		attrs.Namespace = ns
	}
	review, err := c.Clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx,
		&authorizationv1.SelfSubjectAccessReview{Spec: authorizationv1.SelfSubjectAccessReviewSpec{ResourceAttributes: attrs}},
		metav1.CreateOptions{})
	r := accessResult{at: time.Now()}
	if err != nil {
		r.err = err.Error()
		return r // not cached: a transient failure should be retried
	}
	r.allowed = review.Status.Allowed
	accessCache.Lock()
	accessCache.m[cacheKey] = r
	accessCache.Unlock()
	return r
}

// gate decides whether a collector may run: every declared access must be
// allowed or unknown. It returns the denied accesses.
func (e *CollectEnv) gate(c EvidenceCollector) []Access {
	d, ok := c.(AccessDeclarer)
	if !ok {
		return nil
	}
	var denied []Access
	for _, a := range d.Access() {
		check, known := e.access[a.key(e.Namespace)]
		if known && !check.Allowed && check.Error == "" {
			denied = append(denied, a)
		}
	}
	return denied
}

// record notes a collector's outcome once; the first status wins.
func (e *CollectEnv) record(name, status, detail string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.coverage == nil {
		e.coverage = map[string]model.CollectorStatus{}
	}
	if _, done := e.coverage[name]; done {
		return
	}
	e.coverage[name] = model.CollectorStatus{Collector: name, Status: status, Detail: detail}
}

// runGated runs c unless access denies it, recording the outcome.
func (e *CollectEnv) runGated(ctx context.Context, c EvidenceCollector, b *model.EvidenceBundle) {
	if denied := e.gate(c); len(denied) > 0 {
		parts := make([]string, len(denied))
		for i, a := range denied {
			parts[i] = a.String()
		}
		e.record(c.Name(), StatusNoPermission, "needs "+strings.Join(parts, ", "))
		return
	}
	before := len(b.Warnings)
	safeCollect(ctx, c, e, b)
	if len(b.Warnings) > before && strings.HasPrefix(b.Warnings[len(b.Warnings)-1], "collector "+c.Name()+" failed") {
		e.record(c.Name(), StatusFailed, b.Warnings[len(b.Warnings)-1])
		return
	}
	e.record(c.Name(), StatusCollected, "")
}

// coverageList is the recorded outcomes, sorted by collector name.
func (e *CollectEnv) coverageList() []model.CollectorStatus {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]model.CollectorStatus, 0, len(e.coverage))
	for _, s := range e.coverage {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Collector < out[j].Collector })
	return out
}

// declaredAccess lists what the scope resolution and the collectors need.
func declaredAccess(collectors []EvidenceCollector) []Access {
	out := append([]Access(nil), scopeAccess...)
	var walk func(cs []EvidenceCollector)
	walk = func(cs []EvidenceCollector) {
		for _, c := range cs {
			if p, ok := c.(parallelCollector); ok {
				walk(p.children)
				continue
			}
			if d, ok := c.(AccessDeclarer); ok {
				out = append(out, d.Access()...)
			}
		}
	}
	walk(collectors)
	return out
}

// coverageWarning sums up, honestly, what this investigation could not read.
func coverageWarning(list []model.CollectorStatus) []string {
	var denied, failed []string
	for _, s := range list {
		switch s.Status {
		case StatusNoPermission:
			denied = append(denied, fmt.Sprintf("%s (%s)", s.Collector, s.Detail))
		case StatusFailed:
			failed = append(failed, s.Collector)
		}
	}
	var out []string
	if len(denied) > 0 {
		out = append(out, "Not checked, no permission: "+strings.Join(denied, "; "))
	}
	if len(failed) > 0 {
		out = append(out, "Not checked, collection failed: "+strings.Join(failed, ", "))
	}
	return out
}

func sortPermissions(p []model.PermissionCheck) {
	sort.Slice(p, func(i, j int) bool {
		a, b := p[i], p[j]
		if a.Group+a.Resource != b.Group+b.Resource {
			return a.Group+a.Resource < b.Group+b.Resource
		}
		return a.Verb < b.Verb
	})
}

func appendUnique(list []string, s string) []string {
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}

func errFromPanic(p any) error { return fmt.Errorf("lister failed: %v", p) }
