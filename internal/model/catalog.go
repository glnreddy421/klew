package model

import "time"

// ResourceAccessState describes whether the current identity can use a resource type.
type ResourceAccessState string

const (
	ResourceAccessUnknown     ResourceAccessState = "unknown"
	ResourceAccessAllowed     ResourceAccessState = "allowed"
	ResourceAccessForbidden   ResourceAccessState = "forbidden"
	ResourceAccessUnavailable ResourceAccessState = "unavailable"
	ResourceAccessError       ResourceAccessState = "error"
)

// ResourcePermissions captures read verbs relevant to investigation.
type ResourcePermissions struct {
	Get   *bool `json:"get,omitempty"`
	List  *bool `json:"list,omitempty"`
	Watch *bool `json:"watch,omitempty"`
}

// ResourceCount represents entity enumeration state for a resource kind.
type ResourceCount struct {
	State string `json:"state"` // loaded, loading, forbidden, unavailable, error
	Count int    `json:"count,omitempty"`
	Error string `json:"error,omitempty"`
}

// ResourceSource classifies built-in vs extension APIs when reliable.
type ResourceSource string

const (
	ResourceSourceBuiltin   ResourceSource = "builtin"
	ResourceSourceExtension ResourceSource = "extension"
	ResourceSourceUnknown   ResourceSource = "unknown"
)

// GroupVersionResource is a stable Kubernetes API identity.
type GroupVersionResource struct {
	Group    string `json:"group"`
	Version  string `json:"version"`
	Resource string `json:"resource"`
}

// KubernetesResourceDescriptor is a normalized catalog entry.
type KubernetesResourceDescriptor struct {
	ID             string              `json:"id"`
	Group          string              `json:"group"`
	Version        string              `json:"version"`
	APIVersion     string              `json:"apiVersion"`
	Resource       string              `json:"resource"`
	Kind           string              `json:"kind"`
	Namespaced     bool                `json:"namespaced"`
	ShortNames     []string            `json:"shortNames"`
	SupportedVerbs []string            `json:"supportedVerbs"`
	Permissions    ResourcePermissions `json:"permissions"`
	Source         ResourceSource      `json:"source"`
	AccessState    ResourceAccessState `json:"accessState"`
	Discovered     bool                `json:"discovered"`
	Count          *ResourceCount      `json:"count,omitempty"`
}

// CatalogEntity is a lightweight object reference for resource browsing.
type CatalogEntity struct {
	ResourceID        string `json:"resourceId"`
	Name              string `json:"name"`
	Namespace         string `json:"namespace,omitempty"`
	UID               string `json:"uid,omitempty"`
	ResourceVersion   string `json:"resourceVersion,omitempty"`
	Kind              string `json:"kind"`
	APIVersion        string `json:"apiVersion"`
	CreationTimestamp string `json:"creationTimestamp,omitempty"`
	StatusHint        string `json:"statusHint,omitempty"`
}

// CatalogEntityList is the result of listing entities for a resource GVR.
type CatalogEntityList struct {
	Entities    []CatalogEntity     `json:"entities"`
	AccessState ResourceAccessState `json:"accessState"`
	Error       string              `json:"error,omitempty"`
}

// ResourceCatalog is a discovery-driven, RBAC-aware resource index for a scope.
type ResourceCatalog struct {
	Context             string                         `json:"context"`
	Cluster             string                         `json:"cluster"`
	Namespace           string                         `json:"namespace"`
	GeneratedAt         time.Time                      `json:"generatedAt"`
	DiscoveryDurationMs int64                          `json:"discoveryDurationMs"`
	AuthDurationMs      int64                          `json:"authDurationMs"`
	Resources           []KubernetesResourceDescriptor `json:"resources"`
	Namespaced          []KubernetesResourceDescriptor `json:"namespaced"`
	Extensions          []KubernetesResourceDescriptor `json:"extensions"`
	ClusterScoped       []KubernetesResourceDescriptor `json:"clusterScoped"`
	FailedGroups        []string                       `json:"failedGroups,omitempty"`
}
