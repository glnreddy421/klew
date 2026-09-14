import { buildFocusScope, buildChainRows } from './focusScope.js'
import { buildInspectKey, parseInspectKey } from './matches.js'

/** Kinds that participate in a workload connection trace. */
export const WORKLOAD_TRACE_KINDS = new Set([
  'Pod',
  'Deployment',
  'ReplicaSet',
  'StatefulSet',
  'DaemonSet',
  'Job',
  'CronJob',
  'Service',
  'Ingress',
])

/** Left-to-right display order (traffic → workload → runtime). */
const TRACE_DISPLAY_ORDER = [
  'Ingress',
  'Service',
  'Deployment',
  'StatefulSet',
  'DaemonSet',
  'CronJob',
  'Job',
  'ReplicaSet',
  'Pod',
]

const VIA_LABELS = {
  routes: 'routes to',
  selects: 'selects',
  owns: 'owns',
  owner: 'owned by',
  name: 'related',
  focus: '',
  mounts: 'mounts',
  prefix: 'matches',
}

export function isWorkloadTraceKind(kind) {
  return WORKLOAD_TRACE_KINDS.has(kind)
}

function rowKeysMatch(a, b) {
  if (!a || !b) return false
  if (a === b) return true
  const pa = parseInspectKey(a)
  const pb = parseInspectKey(b)
  return Boolean(pa && pb && pa.kind === pb.kind && pa.name === pb.name
    && (pa.namespace || '') === (pb.namespace || ''))
}

function ensureFocusRow(row) {
  if (!row?.kind && !row?.name) return null
  const kind = row.ref?.kind || row.kind
  const name = row.ref?.name || row.name
  const namespace = row.ref?.namespace || row.namespace || ''
  const key = row.key || buildInspectKey(kind, name, namespace) || `${kind}/${name}`
  return {
    ...row,
    key,
    kind,
    name,
    namespace,
    ref: row.ref || { kind, name, namespace },
  }
}

function namesRelated(a, b) {
  if (!a || !b) return false
  const x = String(a).toLowerCase()
  const y = String(b).toLowerCase()
  if (x === y) return true
  return x.startsWith(`${y}-`) || y.startsWith(`${x}-`)
}

function nameScore(name, anchor) {
  if (!name || !anchor) return 0
  if (namesRelated(name, anchor)) return 10
  const prefix = anchor.split('-')[0]
  if (prefix && namesRelated(name, prefix)) return 5
  return 0
}

function pickBestRow(rows, anchorName, focusRow) {
  if (!rows.length) return null
  if (rows.length === 1) return rows[0]
  return [...rows].sort((a, b) => {
    const focusA = rowKeysMatch(a.key, focusRow?.key) ? 100 : 0
    const focusB = rowKeysMatch(b.key, focusRow?.key) ? 100 : 0
    if (focusA !== focusB) return focusB - focusA
    const scoreA = nameScore(a.name, anchorName)
    const scoreB = nameScore(b.name, anchorName)
    if (scoreA !== scoreB) return scoreB - scoreA
    return (a.name || '').localeCompare(b.name || '')
  })[0]
}

function resolveAnchorName(scope, chainRows, focusRow) {
  const root = scope.rootRef || scope.focusRef
  if (root?.name) return root.name

  const deployment = chainRows.find((r) => r.kind === 'Deployment')
  if (deployment?.name) return deployment.name

  return focusRow?.name || ''
}

function inferEdgeLabel(fromNode, toNode, viaByKey) {
  if (fromNode.kind === 'Ingress' && toNode.kind === 'Service') return 'routes to'
  if (fromNode.kind === 'Service' && ['Deployment', 'StatefulSet', 'DaemonSet'].includes(toNode.kind)) {
    return 'selects'
  }
  if (fromNode.kind === 'Service' && toNode.kind === 'Pod') return 'selects'
  if (['Deployment', 'StatefulSet', 'DaemonSet', 'Job', 'CronJob'].includes(fromNode.kind)
    && toNode.kind === 'ReplicaSet') {
    return 'scales'
  }
  if (fromNode.kind === 'ReplicaSet' && toNode.kind === 'Pod') return 'runs'
  if (fromNode.kind === 'Job' && toNode.kind === 'Pod') return 'runs'
  if (fromNode.kind === 'CronJob' && toNode.kind === 'Job') return 'creates'

  const via = viaByKey.get(toNode.key)
  return VIA_LABELS[via] || '→'
}

function nodeBadge(row) {
  if (row.status === 'critical' || row.status === 'degraded') {
    const text = row.signal || (row.ready != null && row.total != null ? `${row.ready}/${row.total}` : 'issue')
    return { text, tone: row.status === 'critical' ? 'failure' : 'warn' }
  }
  if (row.ready != null && row.total != null && row.ready < row.total) {
    return { text: `${row.ready}/${row.total}`, tone: 'warn' }
  }
  return null
}

/**
 * Build a linear trace path for browse-mode workload inspection.
 * Uses focus-chain catalog rows (no investigation snapshot required).
 */
export function buildWorkloadTrace({ focusRow, catalogRows = [], view = null }) {
  const row = ensureFocusRow(focusRow)
  if (!row || !isWorkloadTraceKind(row.kind)) {
    return { nodes: [], edges: [], anchorName: '', podCount: 0 }
  }

  const emptyView = () => ({ state: { snapshot: {} } })
  const scope = buildFocusScope(view || emptyView(), row, { catalogRows })
  if (!scope.active) {
    return { nodes: [], edges: [], anchorName: row.name, podCount: 0 }
  }

  const mergedRows = [...catalogRows]
  if (!mergedRows.some((r) => rowKeysMatch(r.key, row.key))) {
    mergedRows.push(row)
  }

  const chainRows = buildChainRows(view || emptyView(), scope, mergedRows)
  const anchorName = resolveAnchorName(scope, chainRows, row)
  const viaByKey = new Map((scope.relations || []).map((rel) => [rel.key, rel.via]))

  const nodes = []
  for (const kind of TRACE_DISPLAY_ORDER) {
    const candidates = chainRows.filter((r) => r.kind === kind)
    const pick = pickBestRow(candidates, anchorName, row)
    if (!pick) continue

    const badge = nodeBadge(pick)
    nodes.push({
      key: pick.key,
      kind: pick.kind,
      name: pick.name,
      namespace: pick.namespace || pick.ref?.namespace || '',
      isFocus: rowKeysMatch(pick.key, row.key),
      badge,
      extraCount: kind === 'Pod' && candidates.length > 1 ? candidates.length - 1 : 0,
    })
  }

  const edges = []
  for (let i = 1; i < nodes.length; i += 1) {
    const fromNode = nodes[i - 1]
    const toNode = nodes[i]
    edges.push({
      from: fromNode.key,
      to: toNode.key,
      label: inferEdgeLabel(fromNode, toNode, viaByKey),
    })
  }

  return {
    nodes,
    edges,
    anchorName,
    podCount: scope.relatedPodCount || 0,
  }
}
