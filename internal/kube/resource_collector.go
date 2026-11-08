package kube

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/glnreddy421/klew/v2/internal/model"
)

const (
	defaultResourceLimit = 200
	// MaxResourceLimit caps the objects one generic collector keeps.
	MaxResourceLimit = 1000
)

// ResourceCollectorSpec defines a generic collector: list one resource type
// and keep the objects for rules. It can only list.
type ResourceCollectorSpec struct {
	// Name keys the objects in EvidenceBundle.Resources; rules refer to it.
	Name  string `json:"name"`
	Group string `json:"group,omitempty"`
	// Version is the API version; empty uses the version the cluster prefers.
	Version  string `json:"version,omitempty"`
	Resource string `json:"resource"`
	// ClusterScoped lists across the cluster instead of the investigation namespace.
	ClusterScoped bool `json:"clusterScoped,omitempty"`
	// Limit caps the objects kept (default 200, max 1000).
	Limit int `json:"limit,omitempty"`
	// Optional collectors skip quietly when the API is not installed or the
	// user may not list it; the investigation notes the skip once.
	Optional bool `json:"-"`
	// Keep, when set, drops objects it returns false for.
	Keep func(env *CollectEnv, obj map[string]any) bool `json:"-"`
	// Transform, when set, rewrites each kept object, e.g. to drop values
	// rules must not see.
	Transform func(obj map[string]any) `json:"-"`
}

type resourceCollector struct {
	spec       ResourceCollectorSpec
	newDynamic func(*Client) (dynamic.Interface, error)
}

// NewResourceCollector validates spec. Secrets are refused: Klew never reads
// Secret values. Subresources (pods/log, …) are refused too.
func NewResourceCollector(spec ResourceCollectorSpec) (EvidenceCollector, error) {
	spec.Resource = strings.ToLower(strings.TrimSpace(spec.Resource))
	switch {
	case spec.Name == "":
		return nil, fmt.Errorf("collector needs a name")
	case spec.Resource == "":
		return nil, fmt.Errorf("collector %s needs a resource", spec.Name)
	case strings.Contains(spec.Resource, "/"):
		return nil, fmt.Errorf("collector %s: subresources such as %s cannot be collected", spec.Name, spec.Resource)
	case spec.Group == "" && spec.Resource == "secrets":
		return nil, fmt.Errorf("collector %s: Secrets cannot be collected", spec.Name)
	case spec.Group == "" && spec.Resource == "configmaps" && spec.Transform == nil:
		return nil, fmt.Errorf("collector %s: ConfigMaps are collected by Klew already, without their values; use resource configmaps in rules", spec.Name)
	}
	if spec.Limit <= 0 {
		spec.Limit = defaultResourceLimit
	}
	spec.Limit = min(spec.Limit, MaxResourceLimit)
	return &resourceCollector{spec: spec, newDynamic: dynamicClient}, nil
}

func (c *resourceCollector) Name() string { return c.spec.Name }

// Access is the list permission the collector needs.
func (c *resourceCollector) Access() []Access {
	return []Access{{Group: c.spec.Group, Resource: c.spec.Resource, Verb: "list", ClusterScoped: c.spec.ClusterScoped}}
}

func (c *resourceCollector) Collect(ctx context.Context, env *CollectEnv, b *model.EvidenceBundle) {
	objects, err := c.list(ctx, env)
	switch {
	case err == nil:
		if b.Resources == nil {
			b.Resources = map[string][]map[string]any{}
		}
		b.Resources[c.spec.Name] = objects
	case errNotServed(err):
		env.record(c.spec.Name, StatusNotInstalled, "the cluster does not serve "+c.groupResource())
		if !c.spec.Optional {
			b.Warnings = append(b.Warnings, fmt.Sprintf("collector %s: %s is not installed in this cluster", c.spec.Name, c.groupResource()))
		}
	case apierrors.IsForbidden(err):
		// The review allowed it but the list was refused (e.g. a webhook authorizer).
		env.record(c.spec.Name, StatusNoPermission, "list "+c.groupResource()+" was refused")
	default:
		env.record(c.spec.Name, StatusFailed, err.Error())
		b.Warnings = append(b.Warnings, fmt.Sprintf("collector %s: %v", c.spec.Name, err))
	}
}

func (c *resourceCollector) groupResource() string {
	if c.spec.Group == "" {
		return c.spec.Resource
	}
	return c.spec.Resource + "." + c.spec.Group
}

var errAPINotServed = errors.New("API not served by this cluster")

func errNotServed(err error) bool {
	return errors.Is(err, errAPINotServed) || apierrors.IsNotFound(err)
}

func (c *resourceCollector) list(ctx context.Context, env *CollectEnv) ([]map[string]any, error) {
	version := c.spec.Version
	if version == "" {
		version = env.preferredVersion(c.spec.Group)
		if version == "" {
			return nil, errAPINotServed
		}
	}
	dyn, err := c.newDynamic(env.Client)
	if err != nil {
		return nil, err
	}
	gvr := schema.GroupVersionResource{Group: c.spec.Group, Version: version, Resource: c.spec.Resource}
	res := dyn.Resource(gvr)
	opts := metav1.ListOptions{Limit: int64(c.spec.Limit)}
	var list interface{ UnstructuredContent() map[string]any }
	if c.spec.ClusterScoped {
		list, err = res.List(ctx, opts)
	} else {
		list, err = res.Namespace(env.Namespace).List(ctx, opts)
	}
	if err != nil {
		if errNotServed(err) {
			return nil, err
		}
		return nil, fmt.Errorf("list %s: %w", gvr.String(), err)
	}
	items, _ := list.UnstructuredContent()["items"].([]any)
	objects := make([]map[string]any, 0, len(items))
	for _, it := range items {
		obj, ok := it.(map[string]any)
		if !ok || (c.spec.Keep != nil && !c.spec.Keep(env, obj)) {
			continue
		}
		trimObject(obj)
		if c.spec.Transform != nil {
			c.spec.Transform(obj)
		}
		objects = append(objects, obj)
	}
	return objects, nil
}

// trimObject drops metadata that is large and never useful to a rule.
func trimObject(obj map[string]any) {
	meta, ok := obj["metadata"].(map[string]any)
	if !ok {
		return
	}
	delete(meta, "managedFields")
	if ann, ok := meta["annotations"].(map[string]any); ok {
		delete(ann, "kubectl.kubernetes.io/last-applied-configuration")
	}
}

// parallelCollector runs independent collectors concurrently, each on its own
// scratch bundle, and merges their Resources and Warnings in order.
type parallelCollector struct {
	name     string
	children []EvidenceCollector
}

func (p parallelCollector) Name() string { return p.name }

func (p parallelCollector) Collect(ctx context.Context, env *CollectEnv, b *model.EvidenceBundle) {
	env.serverGroups() // resolve discovery once, before the goroutines race for it
	scratch := make([]model.EvidenceBundle, len(p.children))
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentLogFetches)
	for i, c := range p.children {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			env.runGated(ctx, c, &scratch[i])
		}()
	}
	wg.Wait()
	for _, s := range scratch {
		for name, objs := range s.Resources {
			if b.Resources == nil {
				b.Resources = map[string][]map[string]any{}
			}
			b.Resources[name] = objs
		}
		b.Warnings = append(b.Warnings, s.Warnings...)
	}
}

// serverGroups lists the cluster's API groups once per collection.
func (e *CollectEnv) serverGroups() *metav1.APIGroupList {
	e.groupsOnce.Do(func() {
		if e.Client == nil || e.Client.Clientset == nil {
			return
		}
		e.groups, _ = e.Client.Clientset.Discovery().ServerGroups()
	})
	return e.groups
}

// preferredVersion is the version the cluster prefers for group, or "" when
// the group is not served.
func (e *CollectEnv) preferredVersion(group string) string {
	groups := e.serverGroups()
	if groups == nil {
		return ""
	}
	for _, g := range groups.Groups {
		if g.Name == group {
			return g.PreferredVersion.Version
		}
	}
	return ""
}
