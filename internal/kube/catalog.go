package kube

import (
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"

	"github.com/glnreddy421/klew/internal/model"
)

const catalogDiscoveryTTL = 5 * time.Minute

// DiscoveredResource holds normalized Kubernetes API discovery metadata.
type DiscoveredResource struct {
	Group      string
	Version    string
	APIVersion string
	Resource   string
	Kind       string
	Namespaced bool
	Verbs      []string
	ShortNames []string
	Categories []string
}

// ResourceID returns a stable GVR identity string.
func ResourceID(group, version, resource string) string {
	if group == "" {
		return version + "/" + resource
	}
	return group + "/" + version + "/" + resource
}

// PresentationKey returns group/resource identity for UI classification (no version).
func PresentationKey(group, resource string) string {
	if group == "" {
		return "/" + resource
	}
	return group + "/" + resource
}

// ParseResourceID splits a GVR identity string into group, version, resource.
func ParseResourceID(id string) (group, version, resource string, err error) {
	parts := strings.Split(id, "/")
	switch len(parts) {
	case 2:
		return "", parts[0], parts[1], nil
	case 3:
		return parts[0], parts[1], parts[2], nil
	default:
		return "", "", "", fmt.Errorf("invalid resource id %q", id)
	}
}

func apiVersion(group, version string) string {
	if group == "" {
		return version
	}
	return group + "/" + version
}

func isSubresource(name string) bool {
	return strings.Contains(name, "/")
}

func classifySource(group string) model.ResourceSource {
	if group == "" {
		return model.ResourceSourceBuiltin
	}
	if strings.HasSuffix(group, ".k8s.io") {
		return model.ResourceSourceBuiltin
	}
	return model.ResourceSourceExtension
}

func parseDiscoveredResources(lists []*metav1.APIResourceList, failedGroups []string) []DiscoveredResource {
	var out []DiscoveredResource
	seen := map[string]bool{}
	for _, list := range lists {
		if list == nil {
			continue
		}
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}
		for _, r := range list.APIResources {
			if isSubresource(r.Name) {
				continue
			}
			if r.Kind == "" || r.Name == "" {
				continue
			}
			id := ResourceID(gv.Group, gv.Version, r.Name)
			if seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, DiscoveredResource{
				Group:      gv.Group,
				Version:    gv.Version,
				APIVersion: apiVersion(gv.Group, gv.Version),
				Resource:   r.Name,
				Kind:       r.Kind,
				Namespaced: r.Namespaced,
				Verbs:      append([]string(nil), r.Verbs...),
				ShortNames: append([]string(nil), r.ShortNames...),
				Categories: append([]string(nil), r.Categories...),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Resource < out[j].Resource
	})
	_ = failedGroups
	return out
}

func toDescriptor(d DiscoveredResource, perms model.ResourcePermissions, access model.ResourceAccessState, count *model.ResourceCount) model.KubernetesResourceDescriptor {
	source := classifySource(d.Group)
	section := source
	if section == model.ResourceSourceExtension && !d.Namespaced {
		// cluster-scoped extensions stay extension
	}
	return model.KubernetesResourceDescriptor{
		ID:             ResourceID(d.Group, d.Version, d.Resource),
		Group:          d.Group,
		Version:        d.Version,
		APIVersion:     d.APIVersion,
		Resource:       d.Resource,
		Kind:           d.Kind,
		Namespaced:     d.Namespaced,
		ShortNames:     d.ShortNames,
		SupportedVerbs: d.Verbs,
		Permissions:    perms,
		Source:         source,
		AccessState:    access,
		Discovered:     true,
		Count:          count,
	}
}

func partitionCatalog(descriptors []model.KubernetesResourceDescriptor) (namespaced, extensions, cluster []model.KubernetesResourceDescriptor) {
	for _, d := range descriptors {
		if !d.Namespaced {
			cluster = append(cluster, d)
			continue
		}
		if d.Source == model.ResourceSourceExtension {
			extensions = append(extensions, d)
			continue
		}
		namespaced = append(namespaced, d)
	}
	sortDescriptors := func(list []model.KubernetesResourceDescriptor) {
		sort.Slice(list, func(i, j int) bool {
			return list[i].Kind < list[j].Kind
		})
	}
	sortDescriptors(namespaced)
	sortDescriptors(extensions)
	sortDescriptors(cluster)
	return namespaced, extensions, cluster
}

type discoveryCache struct {
	mu    sync.RWMutex
	items map[string]cachedDiscovery
}

type cachedDiscovery struct {
	resources    []DiscoveredResource
	failedGroups []string
	fetchedAt    time.Time
}

func newDiscoveryCache() *discoveryCache {
	return &discoveryCache{items: map[string]cachedDiscovery{}}
}

func (c *discoveryCache) key(client *Client) string {
	return client.Context + "|" + client.Cluster
}

func (c *discoveryCache) get(client *Client) ([]DiscoveredResource, []string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.items[c.key(client)]
	if !ok || time.Since(entry.fetchedAt) > catalogDiscoveryTTL {
		return nil, nil, false
	}
	return entry.resources, entry.failedGroups, true
}

func (c *discoveryCache) set(client *Client, resources []DiscoveredResource, failed []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[c.key(client)] = cachedDiscovery{
		resources:    resources,
		failedGroups: failed,
		fetchedAt:    time.Now(),
	}
}

func (c *discoveryCache) invalidate(client *Client) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, c.key(client))
}

var globalDiscoveryCache = newDiscoveryCache()

// InvalidateCatalogDiscovery drops cached discovery for a cluster context.
func InvalidateCatalogDiscovery(client *Client) {
	if client == nil {
		return
	}
	globalDiscoveryCache.invalidate(client)
}

func discoverAPIResources(client *Client) ([]DiscoveredResource, []string, error) {
	if cached, failed, ok := globalDiscoveryCache.get(client); ok {
		return cached, failed, nil
	}
	disco := client.Clientset.Discovery()
	lists, failed, err := fetchPreferredResourceLists(disco)
	if err != nil {
		return nil, failed, err
	}
	resources := parseDiscoveredResources(lists, failed)
	globalDiscoveryCache.set(client, resources, failed)
	slog.Debug("kubernetes discovery",
		"context", client.Context,
		"resources", len(resources),
		"failedGroups", len(failed),
	)
	return resources, failed, nil
}

type preferredDiscovery interface {
	ServerPreferredResources() ([]*metav1.APIResourceList, error)
}

func fetchPreferredResourceLists(disco preferredDiscovery) ([]*metav1.APIResourceList, []string, error) {
	lists, err := disco.ServerPreferredResources()
	var failed []string
	if err != nil {
		var groupErr *discovery.ErrGroupDiscoveryFailed
		if errors.As(err, &groupErr) {
			for gv := range groupErr.Groups {
				failed = append(failed, gv.String())
			}
		}
		if len(lists) == 0 {
			return nil, failed, err
		}
	}
	sort.Strings(failed)
	return lists, failed, nil
}
