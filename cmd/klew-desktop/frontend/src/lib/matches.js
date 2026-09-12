export const DEFAULT_PRECHECK = 3
export const WORKLOAD_ROOT_KINDS = [
  'Deployment',
  'StatefulSet',
  'DaemonSet',
  'CronJob',
  'Job',
]

/** Display order when grouping matches; unknown kinds append after. */
const KIND_ORDER = [
  'Deployment',
  'StatefulSet',
  'DaemonSet',
  'ReplicaSet',
  'Job',
  'CronJob',
  'Pod',
  'Service',
  'Ingress',
  'NetworkPolicy',
  'HorizontalPodAutoscaler',
  'PodDisruptionBudget',
  'ConfigMap',
  'Secret',
  'PersistentVolumeClaim',
  'ServiceAccount',
  'Role',
  'RoleBinding',
  'LimitRange',
  'ResourceQuota',
]

const KIND_DISPLAY = {
  Deployment: 'Deployments',
  StatefulSet: 'StatefulSets',
  DaemonSet: 'DaemonSets',
  ReplicaSet: 'ReplicaSets',
  Job: 'Jobs',
  CronJob: 'CronJobs',
  Pod: 'Pods',
  Service: 'Services',
  Ingress: 'Ingresses',
  NetworkPolicy: 'NetworkPolicies',
  HorizontalPodAutoscaler: 'HPAs',
  PodDisruptionBudget: 'PDBs',
  ConfigMap: 'ConfigMaps',
  Secret: 'Secrets',
  PersistentVolumeClaim: 'PVCs',
  ServiceAccount: 'ServiceAccounts',
  Role: 'Roles',
  RoleBinding: 'RoleBindings',
  LimitRange: 'LimitRanges',
  ResourceQuota: 'ResourceQuotas',
}

/** Short badges for matched-list / chain rows. Unknown kinds use a compact fallback. */
const KIND_BADGE = {
  Deployment: 'Deploy',
  StatefulSet: 'STS',
  DaemonSet: 'DS',
  ReplicaSet: 'RS',
  Job: 'Job',
  CronJob: 'Cron',
  Pod: 'Pod',
  Service: 'Svc',
  Ingress: 'Ing',
  Endpoints: 'EP',
  EndpointSlice: 'EPS',
  NetworkPolicy: 'NetPol',
  HorizontalPodAutoscaler: 'HPA',
  PodDisruptionBudget: 'PDB',
  ConfigMap: 'CM',
  Secret: 'Sec',
  PersistentVolumeClaim: 'PVC',
  PersistentVolume: 'PV',
  StorageClass: 'SC',
  ServiceAccount: 'SA',
  Role: 'Role',
  RoleBinding: 'RB',
  ClusterRole: 'CR',
  ClusterRoleBinding: 'CRB',
  LimitRange: 'Limit',
  ResourceQuota: 'Quota',
  Namespace: 'NS',
  Node: 'Node',
  Event: 'Event',
  Lease: 'Lease',
  PriorityClass: 'Prio',
  ValidatingWebhookConfiguration: 'VWh',
  MutatingWebhookConfiguration: 'MWh',
  CustomResourceDefinition: 'CRD',
}

export function kindBadge(kind) {
  if (!kind) return '?'
  if (KIND_BADGE[kind]) return KIND_BADGE[kind]
  // Compact fallback for CRDs / anything not in the map
  if (kind.length <= 5) return kind
  const caps = kind.replace(/[^A-Z]/g, '')
  if (caps.length >= 2 && caps.length <= 5) return caps
  return kind.slice(0, 5)
}


const STATUS_RANK = { critical: 0, degraded: 1, warning: 1, healthy: 2, unknown: 3 }

export function matchKey(ref) {
  if (!ref) return ''
  return `${ref.kind}/${ref.name}`
}

/** Go nil slices deserialize as null — always coerce to an array. */
export function normalizeMatches(matches) {
  return Array.isArray(matches) ? matches : []
}

export function getMatchedObjects(view) {
  const st = view?.state || {}
  if (Array.isArray(st.matchedObjects)) return st.matchedObjects
  const snap = st.snapshot || {}
  return normalizeMatches(snap.matchedObjects)
}

export function filterByScope(matches, activeScope) {
  const list = normalizeMatches(matches)
  if (!activeScope || activeScope.size === 0) return list
  return list.filter((m) => activeScope.has(matchKey(m.ref)))
}

export function groupByKind(matches) {
  const byKind = {}
  for (const m of normalizeMatches(matches)) {
    const kind = m.ref?.kind || 'Unknown'
    if (!byKind[kind]) byKind[kind] = []
    byKind[kind].push(m)
  }
  const groups = []
  for (const kind of KIND_ORDER) {
    if (byKind[kind]?.length) {
      groups.push({
        kind,
        label: KIND_DISPLAY[kind] || kind,
        items: [...byKind[kind]].sort((a, b) => (b.score || 0) - (a.score || 0)),
      })
    }
  }
  for (const kind of Object.keys(byKind)) {
    if (!KIND_ORDER.includes(kind)) {
      groups.push({
        kind,
        label: kind,
        items: [...byKind[kind]].sort((a, b) => (b.score || 0) - (a.score || 0)),
      })
    }
  }
  return groups
}

/** Group derived match rows for the incident list (same order as groupByKind). */
export function groupRowsByKind(rows) {
  const byKind = {}
  for (const row of rows || []) {
    const kind = row.kind || row.ref?.kind || 'Unknown'
    if (!byKind[kind]) byKind[kind] = []
    byKind[kind].push(row)
  }
  const groups = []
  for (const kind of KIND_ORDER) {
    if (byKind[kind]?.length) {
      groups.push({
        kind,
        label: KIND_DISPLAY[kind] || kind,
        items: byKind[kind],
      })
    }
  }
  for (const kind of Object.keys(byKind)) {
    if (!KIND_ORDER.includes(kind)) {
      groups.push({
        kind,
        label: kind,
        items: byKind[kind],
      })
    }
  }
  return groups
}

export function defaultSelectedKeys(matches, limit = DEFAULT_PRECHECK, opts = {}) {
  const list = normalizeMatches(matches)
  if (opts.selectAll && list.length) {
    return new Set(list.map((m) => matchKey(m.ref)))
  }
  const roots = list
    .filter((m) => WORKLOAD_ROOT_KINDS.includes(m.ref?.kind))
    .sort((a, b) => (b.score || 0) - (a.score || 0))
  const keys = new Set()
  for (const m of roots.slice(0, limit)) {
    keys.add(matchKey(m.ref))
  }
  if (keys.size === 0) {
    for (const m of [...list].sort((a, b) => (b.score || 0) - (a.score || 0)).slice(0, limit)) {
      keys.add(matchKey(m.ref))
    }
  }
  return keys
}

/** Pick a sensible initial scope — avoids checking hundreds of resources on large namespaces. */
export function smartDefaultSelectedKeys(matches, query, { largeThreshold = 12 } = {}) {
  const list = normalizeMatches(matches)
  if (list.length === 0) return new Set()
  if (list.length <= 5) {
    return new Set(list.map((m) => matchKey(m.ref)))
  }
  const hasQuery = String(query || '').trim() !== ''
  if (hasQuery && list.length <= largeThreshold) {
    return new Set(list.map((m) => matchKey(m.ref)))
  }
  const limit = hasQuery ? 5 : 3
  return defaultSelectedKeys(list, limit)
}

export function deriveMatchRows(view, matches) {
  const snap = view?.state?.snapshot || {}
  const workloads = snap.workloads || []
  const pods = snap.pods || []
  const services = snap.services || []

  return normalizeMatches(matches).map((m) => {
    const ref = m.ref || {}
    const key = matchKey(ref)
    const wl = workloads.find((w) => w.kind === ref.kind && w.name === ref.name)
    const svc = ref.kind === 'Service'
      ? services.find((s) => s.name === ref.name)
      : null

    let ready = null
    let total = null
    let restarts = 0
    let status = 'healthy'
    let signal = null

    if (wl) {
      ready = wl.ready ?? 0
      total = wl.replicas ?? wl.ready ?? 0
      if (ready < total) {
        status = 'degraded'
        signal = `${ready}/${total} ready`
      }
    }

    const relatedPods = podsForMatch(ref, pods)
    if (relatedPods.length > 0) {
      const podReady = relatedPods.filter((p) => p.ready).length
      const podTotal = relatedPods.length
      restarts = relatedPods.reduce((n, p) => n + (p.restartCount || 0), 0)
      if (ready == null) {
        ready = podReady
        total = podTotal
      }
      const worst = worstPodStatus(relatedPods)
      if (worst.rank < STATUS_RANK[status]) {
        status = worst.status
        signal = worst.signal
      } else if (!signal && podReady < podTotal) {
        status = 'degraded'
        signal = `${podReady}/${podTotal} pods ready`
      }
    }

    if (svc) {
      const epReady = svc.readyEndpoints ?? 0
      const epTotal = svc.totalEndpoints ?? 0
      if (epTotal > 0 && epReady < epTotal) {
        status = 'degraded'
        signal = signal || `${epReady}/${epTotal} ready`
      }
      if (ready == null) {
        ready = epReady
        total = epTotal
      }
    }

    const serviceFields = svc ? {
      serviceType: svc.type || '',
      clusterIP: svc.clusterIP || '',
      selector: svc.selector || '',
      ports: svc.ports || [],
      externalIPs: svc.externalIPs || [],
      readyEndpoints: svc.readyEndpoints,
      totalEndpoints: svc.totalEndpoints,
    } : {}

    if (ref.kind === 'Pod' && relatedPods.length === 1) {
      const p = relatedPods[0]
      ready = p.ready ? 1 : 0
      total = 1
      restarts = p.restartCount || 0
      const worst = worstPodStatus([p])
      status = worst.status
      signal = worst.signal
    }

    return {
      key,
      ref,
      score: m.score || 0,
      matchBy: m.matchBy,
      kind: ref.kind,
      name: ref.name,
      kindBadge: kindBadge(ref.kind),
      ready,
      total,
      restarts,
      status,
      signal,
      ...serviceFields,
    }
  })
}

function podsForMatch(ref, pods) {
  if (!ref?.name) return []
  if (ref.kind === 'Pod') {
    return pods.filter((p) => p.name === ref.name)
  }
  return pods.filter((p) => {
    if (p.name.includes(ref.name)) return true
    for (const o of p.ownerRefs || []) {
      if (o.name?.includes(ref.name)) return true
      if (ref.kind === 'Deployment' && o.kind === 'ReplicaSet' && o.name.includes(ref.name)) return true
    }
    return false
  })
}

export { podsForMatch }

function worstPodStatus(pods) {
  let worst = { status: 'healthy', signal: null, rank: STATUS_RANK.healthy }
  for (const p of pods) {
    const row = podStatus(p)
    if (row.rank < worst.rank) worst = row
  }
  return worst
}

function podStatus(p) {
  if (!p.ready) {
    for (const c of p.containers || []) {
      const reason = (c.reason || c.lastReason || '').toLowerCase()
      if (reason.includes('crash') || reason.includes('backoff') || reason.includes('oom')) {
        return { status: 'critical', signal: c.reason || c.lastReason || 'not ready', rank: STATUS_RANK.critical }
      }
      if (c.state === 'waiting' || c.lastState === 'terminated') {
        return { status: 'degraded', signal: c.reason || 'waiting', rank: STATUS_RANK.degraded }
      }
    }
    return { status: 'degraded', signal: 'not ready', rank: STATUS_RANK.degraded }
  }
  if ((p.restartCount || 0) >= 3) {
    return { status: 'degraded', signal: `${p.restartCount} restarts`, rank: STATUS_RANK.degraded }
  }
  return { status: 'healthy', signal: null, rank: STATUS_RANK.healthy }
}

export function pickWorstRow(rows) {
  if (!rows.length) return null
  const sorted = [...rows].sort((a, b) => {
    const ra = STATUS_RANK[a.status] ?? 3
    const rb = STATUS_RANK[b.status] ?? 3
    if (ra !== rb) return ra - rb
    return (b.score || 0) - (a.score || 0)
  })
  return sorted[0]
}

export function pickDefaultFocus(rows) {
  const worst = pickWorstRow(rows)
  if (worst && worst.status !== 'healthy') return worst.key
  if (!rows.length) return null
  const byScore = [...rows].sort((a, b) => (b.score || 0) - (a.score || 0))
  return byScore[0]?.key || null
}

export function scopeStatus(rows) {
  const worst = pickWorstRow(rows)
  if (!worst) return { status: 'unknown', label: 'HEALTHY' }
  const status = worst.status === 'healthy' ? 'healthy' : worst.status
  const label = status === 'healthy' ? 'HEALTHY' : status === 'critical' ? 'CRITICAL' : 'DEGRADED'
  const unhealthyCount = rows.filter((r) => r.status && r.status !== 'healthy').length
  return { status, label, row: worst, unhealthyCount, matchCount: rows.length }
}

export function filterSignals(signals, focusRow, mode) {
  if (mode === 'all' || !focusRow) return signals || []
  const name = focusRow.name
  const kind = focusRow.kind
  return (signals || []).filter((s) => {
    const ref = s.objectRef
    if (ref?.name) {
      if (ref.kind === kind && ref.name === name) return true
      if (ref.name.includes(name) || name.includes(ref.name)) return true
    }
    const ev = (s.evidence || s.label || '').toLowerCase()
    return ev.includes(name.toLowerCase())
  })
}

export function buildInvestigationQuery(originalQuery, selectedKeys, matches) {
  const list = normalizeMatches(matches)
  if (!selectedKeys || selectedKeys.size === 0 || selectedKeys.size === list.length) {
    return originalQuery
  }
  if (selectedKeys.size === 1) {
    const key = [...selectedKeys][0]
    const m = list.find((x) => matchKey(x.ref) === key)
    if (m?.ref) return typedQuery(m.ref)
  }
  return originalQuery
}

function typedQuery(ref) {
  switch (ref.kind) {
    case 'Deployment':
      return `deployment/${ref.name}`
    case 'StatefulSet':
      return `statefulset/${ref.name}`
    case 'Service':
      return `service/${ref.name}`
    case 'Pod':
      return `pod/${ref.name}`
    default:
      return ref.name
  }
}

export function formatReady(ready, total) {
  if (ready == null && total == null) return '—'
  if (total == null || total === 0) return String(ready ?? '—')
  return `${ready ?? 0}/${total}`
}

/** Cluster-scoped kinds — no namespace on fetch or inspect row. */
export const CLUSTER_SCOPED_KINDS = new Set([
  'Node',
  'Namespace',
  'PersistentVolume',
  'StorageClass',
  'ClusterRole',
  'ClusterRoleBinding',
  'ClusterPolicy',
])

/** Kinds we can fetch on demand via GetObjectDetails when clicked outside the match list. */
export const ON_DEMAND_INSPECT_KINDS = new Set([
  'Pod',
  'Node',
  'Namespace',
  'Deployment',
  'ReplicaSet',
  'ReplicationController',
  'StatefulSet',
  'DaemonSet',
  'Job',
  'CronJob',
  'Service',
  'Endpoints',
  'Ingress',
  'IngressClass',
  'EndpointSlice',
  'ConfigMap',
  'Secret',
  'PersistentVolumeClaim',
  'PersistentVolume',
  'StorageClass',
  'ServiceAccount',
  'HorizontalPodAutoscaler',
  'PodDisruptionBudget',
  'Lease',
  'ResourceQuota',
  'LimitRange',
  'NetworkPolicy',
  'Role',
  'ClusterRole',
  'RoleBinding',
  'ClusterRoleBinding',
  'HelmRelease',
  'NetworkPolicy',
  'Role',
  'RoleBinding',
  'ClusterRole',
  'ClusterRoleBinding',
])

const KIND_ALIASES = {
  pvc: 'PersistentVolumeClaim',
  persistentvolumeclaim: 'PersistentVolumeClaim',
  pv: 'PersistentVolume',
  persistentvolume: 'PersistentVolume',
  hpa: 'HorizontalPodAutoscaler',
  horizontalpodautoscaler: 'HorizontalPodAutoscaler',
  sa: 'ServiceAccount',
  serviceaccount: 'ServiceAccount',
  cm: 'ConfigMap',
  configmap: 'ConfigMap',
  deploy: 'Deployment',
  deployment: 'Deployment',
  sts: 'StatefulSet',
  statefulset: 'StatefulSet',
  ds: 'DaemonSet',
  daemonset: 'DaemonSet',
  rs: 'ReplicaSet',
  replicaset: 'ReplicaSet',
  ing: 'Ingress',
  ingress: 'Ingress',
  svc: 'Service',
  service: 'Service',
  ns: 'Namespace',
  namespace: 'Namespace',
  sc: 'StorageClass',
  storageclass: 'StorageClass',
  netpol: 'NetworkPolicy',
  networkpolicy: 'NetworkPolicy',
  cj: 'CronJob',
  cronjob: 'CronJob',
}

export function normalizeInspectKind(kind) {
  if (!kind) return ''
  const k = String(kind).trim()
  return KIND_ALIASES[k.toLowerCase()] || k
}

/** Build a canonical inspect key from kind, name, and optional namespace. */
export function buildInspectKey(kind, name, namespace = '') {
  const k = normalizeInspectKind(kind)
  const n = String(name || '').trim()
  const ns = String(namespace || '').trim()
  if (!k || !n) return ''
  if (ns && !CLUSTER_SCOPED_KINDS.has(k)) return `${k}/${ns}/${n}`
  return `${k}/${n}`
}

/**
 * Parse inspect keys:
 * - `Kind/name` — cluster-scoped or single-namespace investigation scope
 * - `Kind/namespace/name` — catalog browse (K8s names never contain `/`)
 */
export function parseInspectKey(key) {
  if (!key || typeof key !== 'string') return null
  const parts = key.split('/').filter((p) => p.length > 0)
  if (parts.length < 2) return null

  const kind = normalizeInspectKind(parts[0])
  if (!kind) return null

  if (parts.length === 2) {
    const name = parts[1]
    if (!name) return null
    return { kind, name, namespace: '', key: buildInspectKey(kind, name) }
  }

  const namespace = parts[1]
  const name = parts.slice(2).join('/')
  if (!namespace || !name) return null

  if (CLUSTER_SCOPED_KINDS.has(kind)) {
    const clusterName = parts.slice(1).join('/')
    return { kind, name: clusterName, namespace: '', key: buildInspectKey(kind, clusterName) }
  }

  return {
    kind,
    name,
    namespace,
    key: buildInspectKey(kind, name, namespace),
  }
}

export function isInspectableKey(key, view, rows) {
  if (!key) return false
  const list = Array.isArray(rows) ? rows : []
  if (list.some((r) => r.key === key)) return true
  return !!inspectRowForKey(key, view, rows)
}

function matchesSnapshotName(item, name, namespace = '') {
  if (!item || item.name !== name) return false
  if (!namespace) return true
  return !item.namespace || item.namespace === namespace
}

function lookupSnapshotObject(snap, kind, name, namespace = '') {
  if (!snap || !name) return null
  const match = (item) => matchesSnapshotName(item, name, namespace)
  if (kind === 'Pod') return snap.pods?.find(match) || null
  if (kind === 'Node') return snap.nodes?.find(match) || null
  if (kind === 'Service') return snap.services?.find(match) || null
  if (kind === 'Ingress') return snap.ingresses?.find(match) || null
  if (kind === 'ReplicaSet') return snap.replicaSets?.find(match) || null
  if (kind === 'HorizontalPodAutoscaler') return snap.hpas?.find(match) || null
  if (['Deployment', 'StatefulSet', 'DaemonSet', 'Job', 'CronJob'].includes(kind)) {
    return snap.workloads?.find((w) => match(w) && (!w.kind || w.kind === kind)) || null
  }
  if (kind === 'ConfigMap') return snap.configRefs?.find(match) || null
  if (kind === 'Secret') return snap.secretRefs?.find(match) || null
  if (kind === 'PersistentVolumeClaim') return snap.pvcRefs?.find(match) || null
  return null
}

function investigationNamespace(view) {
  return view?.summary?.namespace
    || view?.state?.snapshot?.namespace
    || view?.state?.context?.namespace
    || ''
}

function synthesizeInspectRow(kind, name, view, status = 'unknown', explicitNamespace) {
  const ns = CLUSTER_SCOPED_KINDS.has(kind)
    ? ''
    : (explicitNamespace !== undefined ? explicitNamespace : investigationNamespace(view))
  return {
    key: buildInspectKey(kind, name, ns),
    kind,
    name,
    ref: { kind, name, namespace: ns },
    namespace: ns,
    status,
    adhoc: true,
  }
}

function snapshotObjectToInspectRow(obj, kind, view) {
  if (kind === 'Node') {
    return synthesizeInspectRow(kind, obj.name, view, obj.ready ? 'healthy' : 'degraded')
  }
  if (kind === 'Service') {
    const degraded = obj.totalEndpoints > 0 && obj.readyEndpoints < obj.totalEndpoints
    return synthesizeInspectRow(kind, obj.name, view, degraded ? 'degraded' : 'healthy')
  }
  if (['Deployment', 'StatefulSet', 'DaemonSet', 'Job', 'CronJob', 'ReplicaSet'].includes(kind)) {
    const ready = obj.ready ?? 0
    const replicas = obj.replicas ?? ready
    const status = replicas > 0 && ready < replicas ? 'degraded' : 'healthy'
    return {
      ...synthesizeInspectRow(kind, obj.name, view, status),
      adhoc: false,
      ready,
      total: replicas,
    }
  }
  return synthesizeInspectRow(kind, obj.name, view)
}

/** Minimal row for a focus key when the object is not in matches or catalog yet. */
export function synthesizeFocusRow(key, namespace = '') {
  const parsed = parseInspectKey(key)
  if (parsed) {
    const ns = parsed.namespace || namespace
    return {
      key,
      ref: { kind: parsed.kind, name: parsed.name, namespace: ns },
      kind: parsed.kind,
      name: parsed.name,
      namespace: ns,
      status: 'unknown',
      signal: null,
      score: 0,
    }
  }
  const [kind, ...rest] = String(key || '').split('/')
  const name = rest.join('/')
  if (!kind || !name) return null
  return {
    key,
    ref: { kind, name, namespace },
    kind,
    name,
    namespace,
    status: 'unknown',
    signal: null,
    score: 0,
  }
}

/** Resolve an inspect row by key, including on-demand cluster/namespace objects. */
export function inspectRowForKey(key, view, rows) {
  const list = Array.isArray(rows) ? rows : []
  const hit = list.find((r) => r.key === key)
  if (hit) return hit

  const parsed = parseInspectKey(key)
  if (!parsed) return null

  const snap = view?.state?.snapshot || {}
  const obj = lookupSnapshotObject(snap, parsed.kind, parsed.name, parsed.namespace)
  if (obj) {
    if (parsed.kind === 'Pod') return podSummaryToInspectRow(obj)
    return snapshotObjectToInspectRow(obj, parsed.kind, view)
  }

  if (ON_DEMAND_INSPECT_KINDS.has(parsed.kind)) {
    return synthesizeInspectRow(parsed.kind, parsed.name, view, 'unknown', parsed.namespace)
  }
  return null
}

export function podSummaryToInspectRow(p) {
  if (!p?.name) return null
  let status = 'healthy'
  if (!p.ready) {
    status = 'degraded'
    for (const c of p.containers || []) {
      const reason = (c.reason || c.lastReason || '').toLowerCase()
      if (reason.includes('crash') || reason.includes('oom') || reason.includes('backoff')) {
        status = 'critical'
        break
      }
    }
  }
  return {
    key: `Pod/${p.name}`,
    kind: 'Pod',
    name: p.name,
    ref: { kind: 'Pod', name: p.name, namespace: p.namespace },
    namespace: p.namespace,
    ready: p.ready ? 1 : 0,
    total: 1,
    restarts: p.restartCount || 0,
    status,
    phase: p.phase,
  }
}
