package kube

import (
	"context"
	"fmt"
	"strings"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/glnreddy421/klew/internal/model"
)

// ParseQuery parses "deploy/payment" or free-text "payment".
func ParseQuery(q string) (kind, name string, freeText string) {
	q = strings.TrimSpace(q)
	if q == "" {
		return "", "", ""
	}
	parts := strings.SplitN(q, "/", 2)
	if len(parts) == 2 {
		if k := normalizeKindAlias(parts[0]); k != "" {
			return k, parts[1], ""
		}
		return parts[0], parts[1], ""
	}
	return "", "", q
}

func normalizeKindAlias(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "po", "pod", "pods":
		return "Pod"
	case "deploy", "deployment", "deployments":
		return "Deployment"
	case "sts", "statefulset", "statefulsets":
		return "StatefulSet"
	case "ds", "daemonset", "daemonsets":
		return "DaemonSet"
	case "rs", "replicaset", "replicasets":
		return "ReplicaSet"
	case "job", "jobs":
		return "Job"
	case "cj", "cronjob", "cronjobs":
		return "CronJob"
	case "svc", "service", "services":
		return "Service"
	case "ing", "ingress", "ingresses":
		return "Ingress"
	case "cm", "configmap", "configmaps":
		return "ConfigMap"
	case "secret", "secrets":
		return "Secret"
	case "pvc", "persistentvolumeclaim", "persistentvolumeclaims":
		return "PersistentVolumeClaim"
	case "hpa", "horizontalpodautoscaler", "horizontalpodautoscalers":
		return "HorizontalPodAutoscaler"
	case "pdb", "poddisruptionbudget", "poddisruptionbudgets":
		return "PodDisruptionBudget"
	case "sa", "serviceaccount", "serviceaccounts":
		return "ServiceAccount"
	case "netpol", "networkpolicy", "networkpolicies":
		return "NetworkPolicy"
	case "role", "roles":
		return "Role"
	case "rb", "rolebinding", "rolebindings":
		return "RoleBinding"
	case "limits", "limitrange", "limitranges":
		return "LimitRange"
	case "quota", "resourcequota", "resourcequotas":
		return "ResourceQuota"
	default:
		return ""
	}
}

// Core namespaced kinds for name matching. Not every CRD — full API scans
// cause client-side throttling (see kyverno/policy GETs) and noisy match lists.
var discoverTargets = []struct {
	gvr  schema.GroupVersionResource
	kind string
}{
	{schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}, "Deployment"},
	{schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}, "StatefulSet"},
	{schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}, "DaemonSet"},
	{schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "replicasets"}, "ReplicaSet"},
	{schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}, "Job"},
	{schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"}, "CronJob"},
	{schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}, "Pod"},
	{schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}, "Service"},
	{schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}, "ConfigMap"},
	{schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}, "Secret"},
	{schema.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumeclaims"}, "PersistentVolumeClaim"},
	{schema.GroupVersionResource{Group: "", Version: "v1", Resource: "serviceaccounts"}, "ServiceAccount"},
	{schema.GroupVersionResource{Group: "", Version: "v1", Resource: "limitranges"}, "LimitRange"},
	{schema.GroupVersionResource{Group: "", Version: "v1", Resource: "resourcequotas"}, "ResourceQuota"},
	{schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"}, "Ingress"},
	{schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"}, "NetworkPolicy"},
	{schema.GroupVersionResource{Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers"}, "HorizontalPodAutoscaler"},
	{schema.GroupVersionResource{Group: "policy", Version: "v1", Resource: "poddisruptionbudgets"}, "PodDisruptionBudget"},
	{schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"}, "Role"},
	{schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"}, "RoleBinding"},
}

// DiscoverMatches finds objects matching the query in a namespace by name
// substring across core Kubernetes kinds (workloads, networking, config, RBAC).
func DiscoverMatches(ctx context.Context, c *Client, namespace, query string) ([]model.MatchedObject, error) {
	if c == nil || c.Clientset == nil {
		return nil, fmt.Errorf("kubernetes client is required")
	}
	kind, name, freeText := ParseQuery(query)
	needle := strings.ToLower(freeText)
	if name != "" {
		needle = strings.ToLower(name)
	}

	dyn, err := dynamicClient(c)
	if err != nil {
		return nil, err
	}

	targets := discoverTargets
	if kind != "" {
		var filtered []struct {
			gvr  schema.GroupVersionResource
			kind string
		}
		for _, t := range discoverTargets {
			if resourceMatchesKind(kind, t.kind, t.gvr.Resource, nil) {
				filtered = append(filtered, t)
			}
		}
		targets = filtered
	}

	var (
		mu      sync.Mutex
		matches []model.MatchedObject
		wg      sync.WaitGroup
		sem     = make(chan struct{}, 3)
	)

	for _, t := range targets {
		t := t
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			ul, err := dyn.Resource(t.gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				return
			}
			var local []model.MatchedObject
			for _, item := range ul.Items {
				n := item.GetName()
				if needle != "" && !strings.Contains(strings.ToLower(n), needle) {
					continue
				}
				k := item.GetKind()
				if k == "" {
					k = t.kind
				}
				matchBy := "name"
				score := scoreNameMatch(n, needle)
				if needle == "" {
					matchBy = "namespace"
				}
				if name != "" && strings.EqualFold(n, name) {
					matchBy = "exact"
					score = 1.0
				}
				local = append(local, model.MatchedObject{
					Ref: model.ObjectRef{
						Kind:      k,
						Name:      n,
						Namespace: item.GetNamespace(),
						UID:       string(item.GetUID()),
					},
					MatchBy: matchBy,
					Score:   score,
				})
			}
			if len(local) == 0 {
				return
			}
			mu.Lock()
			matches = append(matches, local...)
			mu.Unlock()
		}()
	}
	wg.Wait()

	return dedupeMatches(matches), nil
}

func dynamicClient(c *Client) (dynamic.Interface, error) {
	if c.Config == nil {
		return nil, fmt.Errorf("rest config is required for discovery")
	}
	return dynamic.NewForConfig(c.Config)
}

func resourceMatchesKind(wantKind, apiKind, resource string, shortNames []string) bool {
	if strings.EqualFold(wantKind, apiKind) {
		return true
	}
	if strings.EqualFold(wantKind, resource) {
		return true
	}
	for _, s := range shortNames {
		if strings.EqualFold(wantKind, s) {
			return true
		}
	}
	if alias := normalizeKindAlias(wantKind); alias != "" && strings.EqualFold(alias, apiKind) {
		return true
	}
	return false
}

func scoreNameMatch(haystack, needle string) float64 {
	if needle == "" {
		return 0.5
	}
	if strings.EqualFold(haystack, needle) {
		return 1.0
	}
	if strings.Contains(strings.ToLower(haystack), needle) {
		return 0.8
	}
	return 0.3
}

func dedupeMatches(in []model.MatchedObject) []model.MatchedObject {
	seen := map[string]bool{}
	var out []model.MatchedObject
	for _, m := range in {
		key := m.Ref.Kind + "/" + m.Ref.Name
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, m)
	}
	return out
}

// DetectCRDs lists API resources that look like CRDs (non-core groups).
func DetectCRDs(ctx context.Context, c *Client) ([]string, error) {
	_ = ctx
	disco := c.Clientset.Discovery()
	groups, err := disco.ServerGroups()
	if err != nil {
		return nil, err
	}
	var kinds []string
	for _, g := range groups.Groups {
		if g.Name == "" || strings.HasPrefix(g.Name, "kube") {
			continue
		}
		for _, v := range g.Versions {
			rl, err := disco.ServerResourcesForGroupVersion(g.Name + "/" + v.Version)
			if err != nil {
				continue
			}
			for _, r := range rl.APIResources {
				if !strings.Contains(r.Name, "/") {
					kinds = append(kinds, schema.GroupVersionResource{Group: g.Name, Version: v.Version, Resource: r.Name}.String())
				}
			}
		}
	}
	return kinds, nil
}
