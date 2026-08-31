package kube

// EventInInvestigationScope decides whether a Kubernetes event belongs in live
// evidence for the current investigation. scopePods lists every pod in the
// investigation snapshot; Pod events for those names are always kept.
func EventInInvestigationScope(kind, name, query string, scopePods investigationPods) bool {
	switch kind {
	case "Node", "PersistentVolumeClaim":
		return true
	case "Pod":
		if _, ok := scopePods[name]; ok {
			return true
		}
		return query == "" || matchesQueryName(name, query)
	default:
		return query == "" || matchesQueryName(name, query)
	}
}
