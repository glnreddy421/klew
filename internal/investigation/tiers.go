package investigation

// Tier classifies how a resource kind enters an investigation scope.
type Tier int

const (
	// TierUnknown is not part of any scope.
	TierUnknown Tier = iota
	// Tier1 is always in scope for a workload.
	Tier1
	// Tier2 is in scope only when referenced by the workload.
	Tier2
	// Tier3 is an operator/extension custom resource (detected via CRDs).
	Tier3
)

var tier1Kinds = map[string]bool{
	"Deployment": true, "StatefulSet": true, "DaemonSet": true,
	"ReplicaSet": true, "ReplicationController": true, "Pod": true,
	"Service": true, "Endpoints": true, "EndpointSlice": true,
	"Ingress": true, "Event": true, "Node": true,
}

var tier2Kinds = map[string]bool{
	"ConfigMap": true, "Secret": true, "PersistentVolumeClaim": true,
	"ServiceAccount": true, "Role": true, "RoleBinding": true,
	"NetworkPolicy": true, "PodDisruptionBudget": true,
	"HorizontalPodAutoscaler": true, "Lease": true,
	"LimitRange": true, "ResourceQuota": true,
}

// WorkloadRootKinds are the top-level controllers a workload can be rooted at.
var WorkloadRootKinds = map[string]bool{
	"Deployment": true, "StatefulSet": true, "DaemonSet": true,
	"ReplicationController": true, "CronJob": true, "Job": true,
}

// TierOf reports the tier for a resource kind.
func TierOf(kind string) Tier {
	switch {
	case tier1Kinds[kind]:
		return Tier1
	case tier2Kinds[kind]:
		return Tier2
	default:
		return TierUnknown
	}
}
