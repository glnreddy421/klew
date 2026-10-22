import { parseInspectKey } from './matches'

/** Compact breadcrumb segments for the active investigation context. */
export function investigationBreadcrumb({
  cluster,
  inspectKey,
  inspectRow,
  focusPinned,
  drillDown,
}) {
  const namespace = cluster?.selectedNamespace || cluster?.currentContext || ''
  const row = inspectRow || (inspectKey ? parseInspectKey(inspectKey) : null)
  if (!row?.kind && !row?.name && !focusPinned) return null

  const segments = []
  if (focusPinned && drillDown?.label) {
    segments.push(drillDown.label)
  } else if (row?.kind) {
    segments.push(row.kind)
    if (row.name) segments.push(row.name)
  }

  if (!segments.length) return null

  return {
    namespace: namespace || '—',
    segments,
    full: [namespace, ...segments].filter(Boolean).join(' / '),
  }
}

export function inspectRowFromKey(key, view, rows = []) {
  if (!key) return null
  const fromRows = rows.find((r) => r.key === key)
  if (fromRows) return withInspectNamespace(fromRows, view)
  const parsed = parseInspectKey(key)
  if (!parsed) return null
  return withInspectNamespace({
    key,
    kind: parsed.kind,
    name: parsed.name,
    namespace: parsed.namespace,
    ref: parsed,
  }, view)
}

/** Built-in kinds that always live in a namespace. CRDs are left to discovery. */
export const NAMESPACED_BUILTIN_KINDS = new Set([
  'Pod', 'Deployment', 'ReplicaSet', 'StatefulSet', 'DaemonSet', 'Job', 'CronJob',
  'ReplicationController', 'Service', 'Endpoints', 'EndpointSlice', 'Ingress',
  'ConfigMap', 'Secret', 'PersistentVolumeClaim', 'ServiceAccount', 'Role',
  'RoleBinding', 'HorizontalPodAutoscaler', 'NetworkPolicy', 'PodDisruptionBudget',
  'Event', 'LimitRange', 'ResourceQuota',
])

/** The investigation's namespace (single-namespace scope), if any. */
export function investigationNamespace(view) {
  const st = view?.state || {}
  return st.namespaceScope?.primary || st.snapshot?.namespace || st.kubeContext?.namespace || ''
}

/**
 * Investigation surfaces (Overview, Graph, signals, inspector cross-links) use
 * `Kind/name` keys, which carry no namespace. Fill it from the investigation
 * for built-in namespaced kinds so the inspector never treats a Pod as
 * cluster-scoped. Unknown kinds keep their (possibly empty) namespace.
 */
export function withInspectNamespace(row, view, fallbackNamespace = '') {
  if (!row || row.namespace || !NAMESPACED_BUILTIN_KINDS.has(row.kind)) return row
  const ns = investigationNamespace(view) || fallbackNamespace
  return ns ? { ...row, namespace: ns } : row
}
