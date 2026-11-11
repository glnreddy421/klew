import { getKindCountDisplay } from './resourceCatalog.js'
import { podHealthLabel } from './investigationViews.js'
import { WORKLOAD_OVERVIEW_KINDS } from './workloadMetrics.js'

const SEGMENT_TONES = {
  ok: '--ok',
  warn: '--warn',
  crit: '--crit',
  info: '--accent',
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

const POD_STATES = [
  { label: 'Running', tone: 'ok' },
  { label: 'Pending', tone: 'warn' },
  { label: 'Failed', tone: 'crit' },
  { label: 'Other', tone: 'muted' },
]

function podState(row) {
  const phase = String(row.phase || row.signal || '').toLowerCase()
  if (phase === 'succeeded' || phase === 'completed') return 'Running'
  const health = podHealthLabel(row)
  if (health === 'critical' || phase === 'failed') return 'Failed'
  if (health === 'warning' || phase === 'pending') return 'Pending'
  if (health === 'healthy' || phase === 'running') return 'Running'
  return 'Other'
}

/** Count rows per state, keeping the order of `states` in the result. */
function countStates(rows, classify, states) {
  const counts = new Map()
  for (const row of rows) {
    if (!row) continue
    const label = classify(row)
    counts.set(label, (counts.get(label) || 0) + 1)
  }
  return states
    .filter((s) => counts.get(s.label))
    .map((s) => ({ ...s, count: counts.get(s.label) }))
}

const REPLICA_STATES = [
  { label: 'Ready', tone: 'ok' },
  { label: 'Degraded', tone: 'warn' },
  { label: 'Unavailable', tone: 'crit' },
  { label: 'Scaled to zero', tone: 'muted' },
  { label: 'Unknown', tone: 'muted' },
]

/** One state per Deployment, StatefulSet, ReplicaSet or DaemonSet — not per replica. */
function replicaState(row) {
  const desired = row.desiredReplicas
  if (desired == null) return row.status === 'healthy' ? 'Ready' : 'Unknown'
  if (desired === 0) return 'Scaled to zero'
  const ready = row.readyReplicas ?? 0
  if (ready >= desired) return 'Ready'
  return ready === 0 ? 'Unavailable' : 'Degraded'
}

function conditionTrue(row, type) {
  return (row.conditions || []).some((c) => c?.type === type && c?.status === 'True')
}

const JOB_STATES = [
  { label: 'Complete', tone: 'ok' },
  { label: 'Running', tone: 'info' },
  { label: 'Failed', tone: 'crit' },
]

function jobState(row) {
  if (conditionTrue(row, 'Failed')) return 'Failed'
  if (conditionTrue(row, 'Complete')) return 'Complete'
  if (row.succeeded != null && row.completions != null && row.succeeded >= row.completions) {
    return 'Complete'
  }
  return 'Running'
}

const CRONJOB_STATES = [
  { label: 'Scheduled', tone: 'ok' },
  { label: 'Running', tone: 'info' },
  { label: 'Suspended', tone: 'muted' },
]

function cronJobState(row) {
  if (row.suspend) return 'Suspended'
  return row.activeJobs > 0 ? 'Running' : 'Scheduled'
}

function segmentsForKind(kind, rows) {
  const source = (rows || []).filter((r) => !r?.kind || r.kind === kind)
  if (!source.length) return []
  switch (kind) {
    case 'Pod':
      return countStates(source, podState, POD_STATES)
    case 'Deployment':
    case 'StatefulSet':
    case 'ReplicaSet':
    case 'DaemonSet':
      return countStates(source, replicaState, REPLICA_STATES)
    case 'Job':
      return countStates(source, jobState, JOB_STATES)
    case 'CronJob':
      return countStates(source, cronJobState, CRONJOB_STATES)
    default:
      return []
  }
}

/**
 * Scope totals across cards. Every segment counts objects, so the three
 * numbers share a unit. In-progress (info) and inactive (muted) objects are
 * neither healthy nor in need of attention.
 */
export function summarizeWorkloadCards(cards = []) {
  let total = 0
  let healthy = 0
  let attention = 0
  for (const card of cards) {
    total += card.total
    for (const seg of card.segments) {
      if (seg.tone === 'ok') healthy += seg.count
      else if (seg.tone === 'warn' || seg.tone === 'crit') attention += seg.count
    }
  }
  return { total, healthy, attention }
}

function podReason(row) {
  const waiting = (row.containerStatuses || []).find((c) => !c?.ready && c?.reason)
  const base = waiting?.reason || row.phase || row.signal || 'Not ready'
  const restarts = row.restartCount || 0
  return restarts > 0 ? `${base} · ${restarts} restart${restarts === 1 ? '' : 's'}` : base
}

function attentionFor(kind, row) {
  switch (kind) {
    case 'Pod': {
      const state = podState(row)
      if (state === 'Failed') return { tone: 'crit', reason: podReason(row) }
      if (state === 'Pending') return { tone: 'warn', reason: podReason(row) }
      return null
    }
    case 'Deployment':
    case 'StatefulSet':
    case 'ReplicaSet':
    case 'DaemonSet': {
      const state = replicaState(row)
      if (state !== 'Unavailable' && state !== 'Degraded') return null
      return {
        tone: state === 'Unavailable' ? 'crit' : 'warn',
        reason: `${row.readyReplicas ?? 0}/${row.desiredReplicas} ready`,
      }
    }
    case 'Job': {
      if (jobState(row) !== 'Failed') return null
      const failed = (row.conditions || []).find((c) => c?.type === 'Failed')
      return { tone: 'crit', reason: failed?.reason || 'Failed' }
    }
    default:
      return null
  }
}

const TONE_RANK = { crit: 0, warn: 1 }

/**
 * Objects behind the red and amber card segments, worst first and
 * controllers before pods, for click-through to the inspector.
 */
export function buildAttentionItems(kindGroups, entitiesByResourceId = {}) {
  const items = []
  for (const kindGroup of overviewKindGroups(kindGroups)) {
    for (const row of entitiesByResourceId[kindGroup.resourceId] || []) {
      if (!row || (row.kind && row.kind !== kindGroup.kind)) continue
      const hit = attentionFor(kindGroup.kind, row)
      if (!hit) continue
      items.push({
        key: row.key,
        kind: kindGroup.kind,
        name: row.name,
        namespace: row.namespace || '',
        ...hit,
      })
    }
  }
  return items.sort((a, b) => (
    TONE_RANK[a.tone] - TONE_RANK[b.tone]
    || (a.kind === 'Pod') - (b.kind === 'Pod')
    || a.name.localeCompare(b.name)
  ))
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
      segments = [{ label: 'Status unknown', count: total, tone: 'muted' }]
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

const DONUT_CX = 50
const DONUT_CY = 50
const DONUT_R = 36
/** Degrees left empty between segments so edges stay readable. */
const DONUT_GAP_DEG = 3

function polar(cx, cy, r, deg) {
  const a = ((deg - 90) * Math.PI) / 180
  return [cx + r * Math.cos(a), cy + r * Math.sin(a)]
}

function arcPath(startDeg, endDeg) {
  const span = endDeg - startDeg
  if (span <= 0) return ''
  // A full circle needs two arcs; a near-full one still uses a single arc.
  if (span >= 359.9) {
    const [x1, y1] = polar(DONUT_CX, DONUT_CY, DONUT_R, 0)
    const [x2, y2] = polar(DONUT_CX, DONUT_CY, DONUT_R, 179.9)
    const [x3, y3] = polar(DONUT_CX, DONUT_CY, DONUT_R, 359.9)
    return `M ${x1} ${y1} A ${DONUT_R} ${DONUT_R} 0 1 1 ${x2} ${y2} A ${DONUT_R} ${DONUT_R} 0 1 1 ${x3} ${y3}`
  }
  const [x1, y1] = polar(DONUT_CX, DONUT_CY, DONUT_R, startDeg)
  const [x2, y2] = polar(DONUT_CX, DONUT_CY, DONUT_R, endDeg)
  const large = span > 180 ? 1 : 0
  return `M ${x1} ${y1} A ${DONUT_R} ${DONUT_R} 0 ${large} 1 ${x2} ${y2}`
}

/**
 * SVG ring arcs for a workload donut. Each segment is a stroked path with a
 * small gap from its neighbours so the chart reads cleanly at card size.
 * A single full segment is a closed circle (no path gap at 12 o'clock).
 */
export function donutArcs(segments) {
  const total = segments.reduce((n, s) => n + (s.count || 0), 0)
  if (!total) return []

  const active = segments.filter((s) => s.count > 0)
  const colorOf = (tone) => `var(${SEGMENT_TONES[tone] || SEGMENT_TONES.muted})`
  if (active.length === 1) {
    return [{
      label: active[0].label,
      tone: active[0].tone,
      full: true,
      color: colorOf(active[0].tone),
    }]
  }

  const gap = DONUT_GAP_DEG
  const usable = 360 - gap * active.length
  let cursor = 0
  return active.map((seg) => {
    const span = (seg.count / total) * usable
    const start = cursor + gap / 2
    const end = start + span
    cursor += span + gap
    return {
      label: seg.label,
      tone: seg.tone,
      d: arcPath(start, end),
      color: colorOf(seg.tone),
    }
  }).filter((a) => a.d)
}
