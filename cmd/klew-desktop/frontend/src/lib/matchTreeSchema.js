/** Namespace-scoped default tree — kinds always listed; counts come from matches. */
export const NAMESPACE_SCOPE_TREE = [
  {
    id: 'workloads',
    label: 'Workloads',
    kinds: ['Pod', 'Deployment', 'ReplicaSet', 'StatefulSet', 'DaemonSet', 'Job', 'CronJob'],
  },
  {
    id: 'networking',
    label: 'Networking',
    kinds: ['Service', 'Ingress', 'EndpointSlice', 'Endpoints', 'NetworkPolicy'],
  },
  {
    id: 'config-storage',
    label: 'Config & Storage',
    kinds: ['ConfigMap', 'Secret', 'PersistentVolumeClaim'],
  },
  {
    id: 'access-control',
    label: 'Access Control',
    kinds: ['ServiceAccount', 'Role', 'RoleBinding'],
  },
  {
    id: 'scaling-policy',
    label: 'Scaling & Policy',
    kinds: ['HorizontalPodAutoscaler', 'PodDisruptionBudget', 'LimitRange', 'ResourceQuota'],
  },
  {
    id: 'cluster-meta',
    label: 'Cluster/Meta',
    kinds: ['Event', 'Lease'],
  },
]

export const KIND_LABELS = {
  Pod: 'Pods',
  Deployment: 'Deployments',
  ReplicaSet: 'ReplicaSets',
  StatefulSet: 'StatefulSets',
  DaemonSet: 'DaemonSets',
  Job: 'Jobs',
  CronJob: 'CronJobs',
  Service: 'Services',
  Ingress: 'Ingresses',
  EndpointSlice: 'EndpointSlices',
  Endpoints: 'Endpoints',
  NetworkPolicy: 'NetworkPolicies',
  ConfigMap: 'ConfigMaps',
  Secret: 'Secrets',
  PersistentVolumeClaim: 'PersistentVolumeClaims',
  ServiceAccount: 'ServiceAccounts',
  Role: 'Roles',
  RoleBinding: 'RoleBindings',
  HorizontalPodAutoscaler: 'HorizontalPodAutoscalers',
  PodDisruptionBudget: 'PodDisruptionBudgets',
  LimitRange: 'LimitRanges',
  ResourceQuota: 'ResourceQuotas',
  Event: 'Events',
  Lease: 'Leases',
}

const SCHEMA_KINDS = new Set(
  NAMESPACE_SCOPE_TREE.flatMap((c) => c.kinds),
)

export function kindLabel(kind) {
  return KIND_LABELS[kind] || kind
}

/**
 * Build category → kind → items tree from match rows.
 * Pod kind lists every pod in the investigation snapshot; other kinds use match rows.
 * Items are entity collections for the master list — not nested in the sidebar.
 */
export function buildCategorizedScopeTree(rows, pods) {
  const list = Array.isArray(rows) ? rows : []
  const podList = Array.isArray(pods) ? pods : []
  const byKind = indexRowsByKind(list)

  const kindNodes = {}
  for (const kind of SCHEMA_KINDS) {
    if (kind === 'Pod') continue
    kindNodes[kind] = buildKindNodes(kind, byKind[kind] || [], list, podList, {
      nestPods: false,
      nestedPodKeys: new Set(),
    })
  }

  kindNodes.Pod = buildPodKindNodes(list, podList, byKind.Pod || [])

  const categories = NAMESPACE_SCOPE_TREE.map((cat) => {
    const kinds = cat.kinds.map((kind) => ({
      kind,
      label: kindLabel(kind),
      count: kindNodes[kind]?.count ?? 0,
      items: kindNodes[kind]?.items ?? [],
    }))
    const count = kinds.reduce((n, k) => n + k.count, 0)
    return { ...cat, count, kinds }
  })

  const otherKinds = Object.keys(byKind)
    .filter((k) => !SCHEMA_KINDS.has(k))
    .sort((a, b) => a.localeCompare(b))

  if (otherKinds.length) {
    const kinds = otherKinds.map((kind) => {
      const items = (byKind[kind] || []).map((row) => ({ row, children: [] }))
      return { kind, label: kindLabel(kind), count: items.length, items }
    })
    categories.push({
      id: 'other',
      label: 'Other',
      count: kinds.reduce((n, k) => n + k.count, 0),
      kinds,
    })
  }

  return {
    count: list.length,
    categories,
  }
}

/** Pods come from the investigation snapshot, merged with any explicit Pod match rows. */
function buildPodKindNodes(allRows, podList, podMatchRows) {
  const byKey = new Map()
  for (const p of podList) {
    if (!p?.name) continue
    const key = `Pod/${p.name}`
    const existing = allRows.find((r) => r.key === key)
    byKey.set(key, {
      row: existing || syntheticPodRow(p),
      children: [],
    })
  }
  for (const row of podMatchRows) {
    const key = row.key || `Pod/${row.name}`
    if (!byKey.has(key)) {
      byKey.set(key, { row, children: [] })
    }
  }
  const items = [...byKey.values()].sort((a, b) =>
    (a.row.name || '').localeCompare(b.row.name || ''),
  )
  return { count: items.length, items }
}

function indexRowsByKind(rows) {
  const byKind = {}
  for (const row of rows) {
    const kind = row.kind || row.ref?.kind || 'Unknown'
    if (!byKind[kind]) byKind[kind] = []
    byKind[kind].push(row)
  }
  return byKind
}

function buildKindNodes(kind, kindRows, allRows, pods, { nestPods, nestedPodKeys }) {
  const items = kindRows.map((row) => ({ row, children: [] }))
  return { count: kindRows.length, items }
}

function syntheticPodRow(p) {
  return {
    key: `Pod/${p.name}`,
    kind: 'Pod',
    name: p.name,
    ref: { kind: 'Pod', name: p.name, namespace: p.namespace },
    ready: p.ready ? 1 : 0,
    total: 1,
    restarts: p.restartCount || 0,
    status: p.ready ? 'healthy' : 'degraded',
  }
}

export function filterCategorizedScopeTree(tree, query) {
  const q = String(query || '').trim().toLowerCase()
  if (!q) return tree

  let count = 0
  const categories = []

  for (const cat of tree.categories) {
    const kinds = []
    for (const kindGroup of cat.kinds) {
      const items = []
      for (const node of kindGroup.items) {
        const selfHit = matchesRow(node.row, q)
          || kindGroup.label.toLowerCase().includes(q)
          || cat.label.toLowerCase().includes(q)
        const kids = (node.children || []).filter((c) => matchesRow(c.row, q))
        if (selfHit || kids.length) {
          items.push({
            row: node.row,
            children: selfHit ? (node.children || []) : kids,
          })
          count += 1
        }
      }
      const kindHit = kindGroup.label.toLowerCase().includes(q)
      if (kindHit && !items.length) {
        kinds.push({ ...kindGroup })
        count += kindGroup.count
      } else if (items.length || (kindHit && kindGroup.count === 0)) {
        kinds.push({
          ...kindGroup,
          items,
          count: items.length,
        })
        if (items.length) count += 0 // already counted items
      } else if (kindGroup.count === 0 && kindHit) {
        kinds.push({ ...kindGroup, items: [], count: 0 })
      }
    }
    const catHit = cat.label.toLowerCase().includes(q)
    if (kinds.length || catHit) {
      categories.push({
        ...cat,
        kinds: kinds.length ? kinds : cat.kinds,
        count: kinds.reduce((n, k) => n + k.count, 0),
      })
    }
  }

  return { count, categories }
}

function matchesRow(row, q) {
  const hay = `${row.kind || ''} ${row.name || ''} ${row.key || ''}`.toLowerCase()
  return hay.includes(q)
}
