package kube

import "sort"

// Built-in generic collectors. Each lists one resource type for the rules in
// the engine. Collectors for APIs a cluster may not have (Gateway API, OLM,
// dynamic resource allocation) and for cluster-scoped kinds a namespaced user
// often cannot list are optional: they skip quietly, and the investigation
// notes the permission gaps once.
func resourceCollectors() []EvidenceCollector {
	specs := []ResourceCollectorSpec{
		// Workload objects the pod-centric collectors do not read
		{Name: "cronjobs", Group: "batch", Version: "v1", Resource: "cronjobs", Optional: true},
		{Name: "statefulsets", Group: "apps", Version: "v1", Resource: "statefulsets", Optional: true},
		{Name: "daemonsets", Group: "apps", Version: "v1", Resource: "daemonsets", Optional: true},
		// Configuration: names and keys only, never values
		{Name: "configmaps", Version: "v1", Resource: "configmaps", Optional: true, Limit: MaxResourceLimit, Transform: keysOnly},
		// Ingress classes and admission webhooks
		{Name: "ingressclasses", Group: "networking.k8s.io", Version: "v1", Resource: "ingressclasses", ClusterScoped: true, Optional: true},
		{Name: "mutatingwebhookconfigurations", Group: "admissionregistration.k8s.io", Version: "v1", Resource: "mutatingwebhookconfigurations", ClusterScoped: true, Optional: true},
		{Name: "validatingwebhookconfigurations", Group: "admissionregistration.k8s.io", Version: "v1", Resource: "validatingwebhookconfigurations", ClusterScoped: true, Optional: true},
		// Storage
		{Name: "persistentvolumeclaims", Version: "v1", Resource: "persistentvolumeclaims"},
		{Name: "storageclasses", Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses", ClusterScoped: true, Optional: true},
		{Name: "persistentvolumes", Version: "v1", Resource: "persistentvolumes", ClusterScoped: true, Optional: true, Limit: MaxResourceLimit, Keep: claimedFromNamespace},
		// Availability and traffic
		{Name: "poddisruptionbudgets", Group: "policy", Version: "v1", Resource: "poddisruptionbudgets", Optional: true},
		{Name: "networkpolicies", Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies", Optional: true},
		// Security
		{Name: "serviceaccounts", Version: "v1", Resource: "serviceaccounts", Optional: true},
		{Name: "roles", Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles", Optional: true},
		{Name: "rolebindings", Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings", Optional: true},
		// Gateway API
		{Name: "gateways", Group: "gateway.networking.k8s.io", Resource: "gateways", Optional: true},
		{Name: "gatewayclasses", Group: "gateway.networking.k8s.io", Resource: "gatewayclasses", ClusterScoped: true, Optional: true},
		{Name: "httproutes", Group: "gateway.networking.k8s.io", Resource: "httproutes", Optional: true},
		// Admission policies
		{Name: "validatingadmissionpolicies", Group: "admissionregistration.k8s.io", Resource: "validatingadmissionpolicies", ClusterScoped: true, Optional: true},
		{Name: "validatingadmissionpolicybindings", Group: "admissionregistration.k8s.io", Resource: "validatingadmissionpolicybindings", ClusterScoped: true, Optional: true},
		// Dynamic resource allocation
		{Name: "resourceclaims", Group: "resource.k8s.io", Resource: "resourceclaims", Optional: true},
		// Operator Lifecycle Manager v0
		{Name: "clusterserviceversions", Group: "operators.coreos.com", Resource: "clusterserviceversions", Optional: true},
		{Name: "subscriptions", Group: "operators.coreos.com", Resource: "subscriptions", Optional: true},
		{Name: "installplans", Group: "operators.coreos.com", Resource: "installplans", Optional: true},
		{Name: "catalogsources", Group: "operators.coreos.com", Resource: "catalogsources", Optional: true},
		{Name: "operatorgroups", Group: "operators.coreos.com", Resource: "operatorgroups", Optional: true},
		// Operator Lifecycle Manager v1
		{Name: "clusterextensions", Group: "olm.operatorframework.io", Resource: "clusterextensions", ClusterScoped: true, Optional: true, Keep: installsIntoNamespace},
		{Name: "clustercatalogs", Group: "olm.operatorframework.io", Resource: "clustercatalogs", ClusterScoped: true, Optional: true},
	}
	out := make([]EvidenceCollector, 0, len(specs))
	for _, s := range specs {
		c, err := NewResourceCollector(s)
		if err != nil {
			panic(err) // built-in specs are static; a bad one is a programming error
		}
		out = append(out, c)
	}
	return out
}

// claimedFromNamespace keeps PersistentVolumes bound to a claim in the
// investigation namespace.
func claimedFromNamespace(env *CollectEnv, obj map[string]any) bool {
	return nestedString(obj, "spec", "claimRef", "namespace") == env.Namespace
}

// installsIntoNamespace keeps ClusterExtensions that install into the
// investigation namespace.
func installsIntoNamespace(env *CollectEnv, obj map[string]any) bool {
	return nestedString(obj, "spec", "namespace") == env.Namespace
}

func nestedString(obj map[string]any, path ...string) string {
	var cur any = obj
	for _, p := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = m[p]
	}
	s, _ := cur.(string)
	return s
}

// keysOnly replaces a ConfigMap's data with the sorted list of its keys, so
// rules can check that a key exists without ever seeing a value.
func keysOnly(obj map[string]any) {
	var keys []string
	for _, field := range []string{"data", "binaryData"} {
		if m, ok := obj[field].(map[string]any); ok {
			for k := range m {
				keys = append(keys, k)
			}
		}
		delete(obj, field)
	}
	sort.Strings(keys)
	obj["keys"] = keys
}
