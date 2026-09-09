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
	ResourceID        string   `json:"resourceId"`
	Name              string   `json:"name"`
	Namespace         string   `json:"namespace,omitempty"`
	UID               string   `json:"uid,omitempty"`
	ResourceVersion   string   `json:"resourceVersion,omitempty"`
	Kind              string   `json:"kind"`
	APIVersion        string   `json:"apiVersion"`
	CreationTimestamp string   `json:"creationTimestamp,omitempty"`
	StatusHint        string   `json:"statusHint,omitempty"`
	NodeName          string   `json:"nodeName,omitempty"`
	ContainerNames    []string `json:"containerNames,omitempty"`
	RestartCount      *int32   `json:"restartCount,omitempty"`
	OwnerKind         string   `json:"ownerKind,omitempty"`
	OwnerName         string   `json:"ownerName,omitempty"`
	QOSClass          string                   `json:"qosClass,omitempty"`
	Containers        []CatalogContainerStatus `json:"containers,omitempty"`
	DesiredReplicas   *int32                   `json:"desiredReplicas,omitempty"`
	CurrentReplicas   *int32                   `json:"currentReplicas,omitempty"`
	ReadyReplicas     *int32                   `json:"readyReplicas,omitempty"`
	AvailableReplicas   *int32             `json:"availableReplicas,omitempty"`
	UpdatedReplicas     *int32             `json:"updatedReplicas,omitempty"`
	UnavailableReplicas *int32             `json:"unavailableReplicas,omitempty"`
	Misscheduled        *int32             `json:"misscheduled,omitempty"`
	Succeeded           *int32             `json:"succeeded,omitempty"`
	Completions         *int32             `json:"completions,omitempty"`
	Schedule            string             `json:"schedule,omitempty"`
	Suspend             *bool              `json:"suspend,omitempty"`
	ActiveJobs          *int32             `json:"activeJobs,omitempty"`
	LastScheduleTime    string             `json:"lastScheduleTime,omitempty"`
	Conditions          []CatalogCondition `json:"conditions,omitempty"`
	ServiceType         string             `json:"serviceType,omitempty"`
	ClusterIP           string             `json:"clusterIP,omitempty"`
	ExternalIPs         []string           `json:"externalIPs,omitempty"`
	Ports               []string           `json:"ports,omitempty"`
	Selector            string             `json:"selector,omitempty"`
	ReadyEndpoints      *int32             `json:"readyEndpoints,omitempty"`
	TotalEndpoints      *int32             `json:"totalEndpoints,omitempty"`
	EndpointSummary     string             `json:"endpointSummary,omitempty"`
	ServiceName         string             `json:"serviceName,omitempty"`
	AddressType         string             `json:"addressType,omitempty"`
	LoadBalancers       []string           `json:"loadBalancers,omitempty"`
	IngressRulesSummary string             `json:"ingressRulesSummary,omitempty"`
	IngressController   string             `json:"ingressController,omitempty"`
	ParameterAPIGroup   string             `json:"parameterAPIGroup,omitempty"`
	ParameterScope      string             `json:"parameterScope,omitempty"`
	ParameterKind       string             `json:"parameterKind,omitempty"`
	ParameterNamespace  string             `json:"parameterNamespace,omitempty"`
	PolicyTypes         []string           `json:"policyTypes,omitempty"`
	Provisioner         string             `json:"provisioner,omitempty"`
	ReclaimPolicy       string             `json:"reclaimPolicy,omitempty"`
	VolumeBindingMode   string             `json:"volumeBindingMode,omitempty"`
	IsDefault           *bool              `json:"isDefault,omitempty"`
	VolumeName          string             `json:"volumeName,omitempty"`
	StorageClassName    string             `json:"storageClassName,omitempty"`
	Capacity            string             `json:"capacity,omitempty"`
	AccessModes         []string           `json:"accessModes,omitempty"`
	ClaimRef            string             `json:"claimRef,omitempty"`
	DataKeys            *int32             `json:"dataKeys,omitempty"`
	SecretType          string             `json:"secretType,omitempty"`
	ScaleTarget         string             `json:"scaleTarget,omitempty"`
	MetricsSummary      string             `json:"metricsSummary,omitempty"`
	MinReplicas         *int32             `json:"minReplicas,omitempty"`
	MaxReplicas         *int32             `json:"maxReplicas,omitempty"`
	PDBMinAvailable     string             `json:"pdbMinAvailable,omitempty"`
	PDBMaxUnavailable   string             `json:"pdbMaxUnavailable,omitempty"`
	PDBDisruptionsAllowed *int32           `json:"pdbDisruptionsAllowed,omitempty"`
	ConfigMapData         []CatalogDataEntry `json:"configMapData,omitempty"`
	LeaseHolder           string                 `json:"leaseHolder,omitempty"`
	NodeResources         *NodeCatalogResources  `json:"nodeResources,omitempty"`
	TableFields           map[string]string      `json:"tableFields,omitempty"`
	HelmStorageKind       string                 `json:"helmStorageKind,omitempty"`
	HelmStorageName       string                 `json:"helmStorageName,omitempty"`
}

// NodeCatalogResources holds Node list/inspector fields from status.capacity/allocatable.
type NodeCatalogResources struct {
	Roles                  string `json:"roles,omitempty"`
	KubeletVersion         string `json:"kubeletVersion,omitempty"`
	TaintCount             *int32 `json:"taintCount,omitempty"`
	CapacityCPUMilli       *int64 `json:"capacityCpuMilli,omitempty"`
	AllocatableCPUMilli    *int64 `json:"allocatableCpuMilli,omitempty"`
	CapacityMemoryBytes    *int64 `json:"capacityMemoryBytes,omitempty"`
	AllocatableMemoryBytes *int64 `json:"allocatableMemoryBytes,omitempty"`
	CapacityDiskBytes      *int64 `json:"capacityDiskBytes,omitempty"`
	AllocatableDiskBytes   *int64 `json:"allocatableDiskBytes,omitempty"`
	Ready                  *bool  `json:"ready,omitempty"`
}

// CatalogDataEntry is a ConfigMap data/binaryData key with byte size.
type CatalogDataEntry struct {
	Key       string `json:"key"`
	Value     string `json:"value,omitempty"`
	SizeBytes int32  `json:"sizeBytes"`
}

// CatalogCondition is a lightweight status condition for browse rows.
type CatalogCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// CatalogContainerStatus is a lightweight container state for pod browse rows.
type CatalogContainerStatus struct {
	Name    string `json:"name"`
	State   string `json:"state"`
	Ready   bool   `json:"ready"`
	Init    bool   `json:"init,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
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
	AllNamespaces       bool                           `json:"allNamespaces,omitempty"`
	Namespaces          []string                       `json:"namespaces,omitempty"`
}
