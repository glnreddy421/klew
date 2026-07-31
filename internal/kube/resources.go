package kube

import (
	"context"
	"fmt"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NamespaceResource is a selectable workload entry inside a namespace.
type NamespaceResource struct {
	Kind string
	Name string
}

// ResourceGroup groups selectable resources by Kubernetes kind.
type ResourceGroup struct {
	Kind  string
	Items []string
}

// ListNamespaceResources returns namespace resources grouped by kind, optionally
// filtered by a fuzzy search string.
func ListNamespaceResources(ctx context.Context, c *Client, namespace, filter string) ([]ResourceGroup, error) {
	groups := map[string][]string{}

	if deps, err := c.Clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{}); err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	} else {
		for _, d := range deps.Items {
			if filter == "" || FuzzyContains(d.Name, filter) {
				groups["Deployments"] = append(groups["Deployments"], d.Name)
			}
		}
	}

	if sts, err := c.Clientset.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{}); err != nil {
		return nil, fmt.Errorf("list statefulsets: %w", err)
	} else {
		for _, s := range sts.Items {
			if filter == "" || FuzzyContains(s.Name, filter) {
				groups["StatefulSets"] = append(groups["StatefulSets"], s.Name)
			}
		}
	}

	if dss, err := c.Clientset.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{}); err == nil {
		for _, d := range dss.Items {
			if filter == "" || FuzzyContains(d.Name, filter) {
				groups["DaemonSets"] = append(groups["DaemonSets"], d.Name)
			}
		}
	}

	if svcs, err := c.Clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{}); err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	} else {
		for _, s := range svcs.Items {
			if filter == "" || FuzzyContains(s.Name, filter) {
				groups["Services"] = append(groups["Services"], s.Name)
			}
		}
	}

	order := []string{"Deployments", "StatefulSets", "DaemonSets", "Services"}
	var out []ResourceGroup
	for _, kind := range order {
		names := groups[kind]
		if len(names) == 0 {
			continue
		}
		sort.Strings(names)
		out = append(out, ResourceGroup{Kind: kind, Items: names})
	}
	return out, nil
}

// FlattenResourceGroups converts grouped resources into a flat selectable list.
func FlattenResourceGroups(groups []ResourceGroup) []NamespaceResource {
	var out []NamespaceResource
	for _, g := range groups {
		for _, name := range g.Items {
			out = append(out, NamespaceResource{Kind: g.Kind, Name: name})
		}
	}
	return out
}
