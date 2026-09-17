/** Active kube context for this window (never loads multiple contexts). */
export function activeClusterContext(cluster) {
  if (!cluster) return ''
  return cluster.selectedContext || cluster.currentContext || ''
}

export function hasActiveClusterContext(cluster) {
  return Boolean(activeClusterContext(cluster))
}

/** Whether kubeconfig contexts are available for pickers (connection optional). */
export function hasClusterContexts(cluster) {
  if (!cluster) return false
  if (Array.isArray(cluster.contexts) && cluster.contexts.length > 0) return true
  return Boolean(cluster.selectedContext || cluster.currentContext)
}

export function clusterContextNames(cluster) {
  if (!cluster) return []
  if (Array.isArray(cluster.contexts) && cluster.contexts.length > 0) {
    return cluster.contexts.map((c) => c.name).filter(Boolean)
  }
  const fallback = cluster.selectedContext || cluster.currentContext
  return fallback ? [fallback] : []
}
