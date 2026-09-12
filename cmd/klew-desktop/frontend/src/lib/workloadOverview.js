import { getKindCountDisplay } from './resourceCatalog.js'
import { podHealthLabel } from './investigationViews.js'
import { WORKLOAD_OVERVIEW_KINDS } from './workloadMetrics.js'

const SEGMENT_TONES = {
  ok: '--ok',
  warn: '--warn',
  crit: '--crit',
  muted: '--text-secondary',
}

function kindTotal(kindGroup) {
  const display = getKindCountDisplay(kindGroup)
  if (display.label && display.className !== 'count-unknown') {
    const n = Number(display.label)
    if (Number.isFinite(n)) return n
  }
  return kindGroup?.count ?? 0
}

function podSegments(rows) {
  if (!rows?.length) return []
  const source = rows

  let running = 0
  let pending = 0
  let failed = 0
  let other = 0

  for (const item of source) {
    if (!item) continue
    const health = podHealthLabel(item)
    const phase = String(item.phase || item.signal || '').toLowerCase()
    if (health === 'critical' || phase === 'failed') failed += 1
    else if (health === 'warning' || phase === 'pending') pending += 1
    else if (health === 'healthy' || phase === 'running' || phase === 'succeeded') running += 1
    else other += 1
  }

  const segments = []
  if (running) segments.push({ label: 'Running', count: running, tone: 'ok' })
  if (pending) segments.push({ label: 'Pending', count: pending, tone: 'warn' })
  if (failed) segments.push({ label: 'Failed', count: failed, tone: 'crit' })
  if (other) segments.push({ label: 'Other', count: other, tone: 'muted' })
  return segments
}

function replicaSegments(rows, kind) {
  const source = rows.filter((r) => r?.kind === kind)
  if (!source?.length) return []

  let ready = 0
  let pending = 0

  for (const item of source) {
    const desired = item.replicas ?? item.desired ?? item.desiredReplicas ?? item.total
    const currentReady = item.ready ?? item.readyReplicas
    if (desired != null && currentReady != null) {
      ready += Math.min(currentReady, desired)
      if (currentReady < desired) pending += desired - currentReady
    } else if (item.status === 'healthy' || String(item.signal || '').includes('/')) {
      ready += 1
    } else {
      pending += 1
    }
  }

  const segments = []
  if (ready) segments.push({ label: 'Ready', count: ready, tone: 'ok' })
  if (pending) segments.push({ label: 'Pending', count: pending, tone: 'warn' })
  return segments
}

function daemonSetSegments(rows) {
  if (!rows.length) return []
  let ready = 0
  let pending = 0
  for (const row of rows) {
    const parsed = String(row.signal || '').match(/(\d+)\s*\/\s*(\d+)\s*ready/i)
    if (parsed) {
      ready += Number(parsed[1])
      pending += Math.max(0, Number(parsed[2]) - Number(parsed[1]))
    } else if (row.readyReplicas != null && row.desiredReplicas != null) {
      ready += row.readyReplicas
      pending += Math.max(0, row.desiredReplicas - row.readyReplicas)
    }
  }
  const segments = []
  if (ready) segments.push({ label: 'Ready', count: ready, tone: 'ok' })
  if (pending) segments.push({ label: 'Pending', count: pending, tone: 'warn' })
  return segments
}

function jobSegments(rows) {
  if (!rows.length) return []
  let complete = 0
  let active = 0
  for (const row of rows) {
    const hint = String(row.signal || '')
    if (hint.includes('/')) {
      const m = hint.match(/(\d+)\s*\/\s*(\d+)/)
      if (m) complete += Number(m[1])
    } else if (row.status === 'healthy') complete += 1
    else active += 1
  }
  const segments = []
  if (complete) segments.push({ label: 'Complete', count: complete, tone: 'ok' })
  if (active) segments.push({ label: 'Active', count: active, tone: 'warn' })
  return segments
}

function segmentsForKind(kind, rows) {
  switch (kind) {
    case 'Pod':
      return podSegments(rows)
    case 'Deployment':
    case 'StatefulSet':
    case 'ReplicaSet':
      return replicaSegments(rows, kind)
    case 'DaemonSet':
      return daemonSetSegments(rows)
    case 'Job':
    case 'CronJob':
      return jobSegments(rows)
    default:
      return []
  }
}

function overviewKindGroups(kindGroups) {
  const set = new Set(WORKLOAD_OVERVIEW_KINDS)
  return (kindGroups || []).filter((kg) => set.has(kg.kind))
}

/** Build donut cards from async catalog list results (Resources → Workloads → Overview). */
export function buildWorkloadKindCardsFromRows(
  kindGroups,
  entitiesByResourceId = {},
  accessStateByResourceId = {},
) {
  const groups = overviewKindGroups(kindGroups)
  if (!groups.length) return []

  return groups.map((kindGroup) => {
    const accessState = accessStateByResourceId[kindGroup.resourceId]
      || kindGroup.accessState
      || (kindGroup.countState?.state === 'forbidden' ? 'forbidden' : 'unknown')
    const denied = accessState === 'forbidden'
    const rows = denied ? [] : (entitiesByResourceId[kindGroup.resourceId] || [])
    const total = denied ? 0 : (rows.length > 0 ? rows.length : kindTotal(kindGroup))
    let segments = segmentsForKind(kindGroup.kind, rows)
    if (!segments.length && total > 0) {
      segments = [{ label: 'Total', count: total, tone: 'ok' }]
    }
    return {
      kind: kindGroup.kind,
      label: kindGroup.label || kindGroup.kind,
      total,
      segments,
      groupId: 'workloads',
      resourceId: kindGroup.resourceId,
      accessState,
      denied,
    }
  })
}

export function donutGradient(segments) {
  const total = segments.reduce((n, s) => n + s.count, 0)
  if (!total) return 'conic-gradient(var(--surface-3) 0 100%)'

  let cursor = 0
  const stops = segments.map((seg) => {
    const pct = (seg.count / total) * 100
    const color = `var(${SEGMENT_TONES[seg.tone] || SEGMENT_TONES.muted})`
    const start = cursor
    cursor += pct
    return `${color} ${start}% ${cursor}%`
  })
  return `conic-gradient(${stops.join(', ')})`
}
