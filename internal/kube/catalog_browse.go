package kube

import (
	"context"

	"github.com/glnreddy421/klew/internal/model"
)

// Ephemeral auth review APIs are create-only machinery — not useful in the resource browser.
var nonBrowsableCatalogResources = map[string]struct{}{
	"subjectaccessreviews":       {},
	"selfsubjectaccessreviews":   {},
	"selfsubjectrulesreviews":    {},
	"localsubjectaccessreviews":  {},
	"tokenreviews":               {},
	"subjectrulesreviews":        {},
	"selfsubjectreviews":         {},
}

// IsBrowsableCatalogResource reports whether a discovered API resource belongs in the browse catalog.
func IsBrowsableCatalogResource(resource string) bool {
	if resource == "" {
		return false
	}
	_, skip := nonBrowsableCatalogResources[resource]
	return !skip
}

func catalogEntityScopeForDescriptor(d model.KubernetesResourceDescriptor, namespace string, allNamespaces bool, namespaces []string) CatalogEntityScope {
	if !d.Namespaced {
		return CatalogEntityScope{ClusterScoped: true}
	}
	if allNamespaces {
		return CatalogEntityScope{AllNamespaces: true}
	}
	if len(namespaces) > 1 {
		return CatalogEntityScope{Namespaces: append([]string(nil), namespaces...)}
	}
	ns := namespace
	if ns == "" && len(namespaces) == 1 {
		ns = namespaces[0]
	}
	return CatalogEntityScope{Namespace: ns}
}

// CountCatalogEntities lists entities once and derives an accurate browse count.
func CountCatalogEntities(ctx context.Context, client *Client, resourceID string, scope CatalogEntityScope) *model.ResourceCount {
	list, err := ListCatalogEntities(ctx, client, resourceID, scope)
	if err != nil {
		return &model.ResourceCount{State: "error", Error: err.Error()}
	}
	switch list.AccessState {
	case model.ResourceAccessForbidden:
		return &model.ResourceCount{State: "forbidden"}
	case model.ResourceAccessUnavailable:
		return &model.ResourceCount{State: "unavailable"}
	case model.ResourceAccessError:
		if list.Error != "" {
			return &model.ResourceCount{State: "error", Error: list.Error}
		}
		return &model.ResourceCount{State: "unavailable"}
	case model.ResourceAccessAllowed:
		return &model.ResourceCount{State: "loaded", Count: len(list.Entities)}
	default:
		return &model.ResourceCount{State: "unknown"}
	}
}

func descriptorSupportsList(d model.KubernetesResourceDescriptor) bool {
	for _, verb := range d.SupportedVerbs {
		if verb == "list" || verb == "*" {
			return true
		}
	}
	if d.Permissions.List != nil && *d.Permissions.List {
		return true
	}
	return false
}
