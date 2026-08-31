/**
 * Visual view-model for Overview investigation graphics.
 * Derives presentation-only structures from existing investigation state.
 * Does NOT invent causality or analysis — labels relationships honestly.
 */

import {
  formatClock,
  severityRankTL,
  asStringArray,
} from './investigationViews.js'

/** @typedef {'signal'|'failure'|'event'|'state'|'evidence'} NodeType */

const NODE_MARKERS = {
  signal: '●',
  failure: '▲',
  event: '◆',
  state: '■',
  evidence: '○',
}

const RELATION_LABELS = {
  explicit: 'followed by',
  temporal: 'detected after',
  correlation: 'correlated with',
  association: 'associated with',
}

/**
 * @param {object} params
 * @returns {object|null}
 */
export function buildVisualChain({
  state,
  findings,
  timelineItems,
  evidence,
  view,
}) {
  const causal = asStringArray(state?.causalChain).map(cleanChainLabel).filter(Boolean)
  const correlation = asStringArray(state?.correlation)

  if (causal.length >= 2) {
    return buildChainFromSteps(causal, {
      label: 'What Klew connected',
      mode: 'chain',
      edgeKind: 'explicit',
      edgeLabel: RELATION_LABELS.explicit,
      state,
      timelineItems,
      evidence,
      findings,
    })
  }

  if (correlation.length >= 2) {
    return buildChainFromSteps(correlation, {
      label: 'What Klew connected',
      mode: 'chain',
      edgeKind: 'correlation',
      edgeLabel: RELATION_LABELS.correlation,
      state,
      timelineItems,
      evidence,
      findings,
    })
  }

  const topFinding = findings.find((f) => f.chainSteps?.length >= 2)
  if (topFinding) {
    return buildChainFromSteps(topFinding.chainSteps, {
      label: 'Observed sequence',
      mode: 'sequence',
      edgeKind: 'temporal',
      edgeLabel: RELATION_LABELS.temporal,
      state,
      timelineItems,
      evidence,
      findings,
      findingId: topFinding.id,
    })
  }

  const fromTimeline = buildChainFromTimeline(state, timelineItems, evidence)
  if (fromTimeline?.nodes.length >= 2) {
    return fromTimeline
  }

  const primary = buildPrimaryObservation(findings, timelineItems, evidence, view)
  if (primary) return primary

  return null
}

function buildChainFromSteps(steps, opts) {
  const limited = steps.slice(0, 6)
  const nodes = limited.map((label, i) => {
    const enriched = enrichNodeFromSources(label, opts.state, opts.timelineItems, opts.evidence)
    return {
      id: `chain-${slug(label)}-${i}`,
      label,
      shortLabel: shortenLabel(label),
      type: enriched.type,
      timestamp: enriched.timestamp,
      meta: enriched.meta,
      count: enriched.count,
      sourceKind: enriched.sourceKind,
      sourceName: enriched.sourceName,
      navTab: enriched.navTab,
      severity: enriched.severity,
      findingIds: opts.findingId && i === 0 ? [opts.findingId] : enriched.findingIds,
    }
  })

  const edges = []
  for (let i = 0; i < nodes.length - 1; i++) {
    edges.push({
      from: nodes[i].id,
      to: nodes[i + 1].id,
      label: opts.edgeLabel,
    })
  }

  linkFindingsToNodes(nodes, opts.findings || [])

  return {
    mode: opts.mode,
    label: opts.label,
    edgeKind: opts.edgeKind,
    nodes,
    edges,
    hasMore: steps.length > 6,
    moreNav: { tab: 'evidence', label: 'View full investigation →' },
  }
}

function buildChainFromTimeline(state, timelineItems, evidence) {
  const sources = []

  for (const item of timelineItems || []) {
    sources.push({
      label: cleanChainLabel(item.label),
      timestamp: item.timestamp,
      type: mapTimelineItemType(item),
      severity: item.severity,
      sourceKind: item.sourceKind,
      sourceName: item.sourceName,
      count: item.count,
      navTab: item.navTab || 'evidence',
    })
  }

  if (sources.length < 2) {
    for (const ev of rankEvidenceForChain(evidence).slice(0, 6)) {
      sources.push({
        label: cleanChainLabel(ev.reason || ev.message || ev.headline),
        timestamp: ev.timestamp,
        type: inferNodeType(ev.reason || ev.message, ev.sourceType),
        severity: ev.severity,
        sourceKind: ev.sourceKind,
        sourceName: ev.sourceName || ev.pod,
        count: 1,
        navTab: 'evidence',
      })
    }
  }

  const deduped = dedupeByLabel(sources)
  if (deduped.length < 2) return null

  deduped.sort((a, b) => new Date(a.timestamp || 0) - new Date(b.timestamp || 0))
  const picked = deduped.slice(0, 6)

  const nodes = picked.map((s, i) => ({
    id: `seq-${slug(s.label)}-${i}`,
    label: s.label,
    shortLabel: shortenLabel(s.label),
    type: s.type,
    timestamp: s.timestamp,
    meta: formatNodeMeta(s),
    count: s.count || 1,
    sourceKind: s.sourceKind,
    sourceName: s.sourceName,
    navTab: s.navTab,
    severity: s.severity,
    findingIds: [],
  }))

  const edges = nodes.slice(0, -1).map((n, i) => ({
    from: n.id,
    to: nodes[i + 1].id,
    label: RELATION_LABELS.temporal,
  }))

  return {
    mode: 'sequence',
    label: 'Observed sequence',
    edgeKind: 'temporal',
    nodes,
    edges,
    hasMore: deduped.length > 6,
    moreNav: { tab: 'evidence', label: 'View full timeline →' },
  }
}

function buildPrimaryObservation(findings, timelineItems, evidence, view) {
  const topFinding = findings[0]
  const topTimeline = timelineItems?.[0]
  const topEvidence = rankEvidenceForChain(evidence)[0]

  let source = null
  if (topFinding) {
    source = {
      label: shortenFindingTitle(topFinding.title),
      type: inferNodeType(topFinding.title),
      timestamp: null,
      meta: topFinding.meta,
      count: countFromMeta(topFinding.meta),
      navTab: topFinding.nav?.tab || 'evidence',
      severity: topFinding.severity,
      findingIds: [topFinding.id],
    }
  } else if (topTimeline) {
    source = {
      label: cleanChainLabel(topTimeline.label),
      type: mapTimelineItemType(topTimeline),
      timestamp: topTimeline.timestamp,
      meta: topTimeline.count > 1 ? `${topTimeline.count} occurrences` : '',
      count: topTimeline.count || 1,
      sourceKind: topTimeline.sourceKind,
      sourceName: topTimeline.sourceName,
      navTab: topTimeline.navTab || 'evidence',
      severity: topTimeline.severity,
      findingIds: [],
    }
  } else if (topEvidence) {
    source = {
      label: cleanChainLabel(topEvidence.reason || topEvidence.message),
      type: inferNodeType(topEvidence.reason, topEvidence.sourceType),
      timestamp: topEvidence.timestamp,
      meta: topEvidence.sourceName || topEvidence.pod || '',
      count: 1,
      navTab: 'evidence',
      severity: topEvidence.severity,
      findingIds: [],
    }
  }

  if (!source?.label) return null

  const supportingCount = (evidence || []).length

  return {
    mode: 'primary',
    label: 'Primary observation',
    edgeKind: null,
    nodes: [{
      id: 'primary-0',
      ...source,
      shortLabel: shortenLabel(source.label),
    }],
    edges: [],
    supportingCount,
    hasMore: false,
    moreNav: supportingCount > 0
      ? { tab: 'evidence', label: 'View evidence →' }
      : null,
  }
}

/**
 * @param {object[]} timelineItems
 * @param {object} state
 * @param {object[]} evidence
 */
export function enrichTimelineVisual(timelineItems, state, evidence) {
  const items = (timelineItems || []).map((item, i) => {
    const nodeType = mapTimelineItemType(item)
    const relatedEvidence = countRelatedEvidence(item, evidence)
    return {
      ...item,
      id: `tl-${i}-${slug(item.label)}`,
      nodeType,
      marker: NODE_MARKERS[nodeType] || NODE_MARKERS.event,
      shortLabel: shortenLabel(item.label),
      hoverTitle: item.label,
      hoverMeta: [item.sourceKind, item.sourceName].filter(Boolean).join('/'),
      hoverEvidenceCount: relatedEvidence,
      timeLabel: formatTimeAxis(item.timestamp),
    }
  })

  const totalTimeline = (state?.timeline || []).length
  const hasMore = totalTimeline > items.length || (evidence?.length || 0) > items.length

  return {
    items: items.slice(0, 8),
    hasMore,
    moreNav: { tab: 'evidence', label: 'View full timeline →' },
    span: computeTimeSpan(items),
  }
}

/**
 * @param {object[]} path
 * @param {object[]} rows
 * @param {object} view
 */
export function enrichResourcePath(path, rows, view) {
  if (!path?.length) return null

  const rowByKey = Object.fromEntries((rows || []).map((r) => [r.key, r]))
  const snap = view?.state?.snapshot || {}

  return path.slice(0, 5).map((step) => {
    const key = `${step.kind}/${step.name}`
    const row = rowByKey[key]
    const badges = []

    if (row?.restarts) badges.push({ tone: 'failure', text: `${row.restarts} restart${row.restarts === 1 ? '' : 's'}` })
    if (row?.signal && row.signal !== 'healthy') {
      badges.push({ tone: row.status === 'critical' ? 'failure' : 'warn', text: row.signal })
    }

    const pod = (snap.pods || []).find((p) => p.name === step.name)
    if (pod && !row) {
      if (pod.restartCount) badges.push({ tone: 'failure', text: `${pod.restartCount} restart${pod.restartCount === 1 ? '' : 's'}` })
      if (!pod.ready) badges.push({ tone: 'warn', text: 'not ready' })
    }

    const svc = (snap.services || []).find((s) => s.name === step.name)
    if (svc) {
      const er = svc.endpointsReady ?? svc.readyEndpoints
      const et = svc.endpointsTotal ?? svc.totalEndpoints
      if (et != null && er != null && et > 0 && er < et) {
        badges.push({ tone: 'warn', text: 'unhealthy endpoint' })
      }
    }

    return {
      ...step,
      badges: badges.slice(0, 2),
      viaLabel: formatRelationLabel(step.via),
    }
  })
}

/**
 * @param {object[]} findings
 * @param {object|null} chain
 */
export function linkFindingsToChainNodes(findings, chain) {
  if (!chain?.nodes?.length) return findings

  return findings.map((f) => {
    const chainNodeIds = []
    for (const node of chain.nodes) {
      if (f.chainSteps?.some((step) => labelsMatch(step, node.label))) {
        chainNodeIds.push(node.id)
      }
      if (labelsMatch(f.title, node.label)) {
        chainNodeIds.push(node.id)
      }
    }
    return { ...f, chainNodeIds: [...new Set(chainNodeIds)] }
  })
}

export function getNodeMarker(type) {
  return NODE_MARKERS[type] || NODE_MARKERS.signal
}

export function nodeTypeClass(type) {
  return `inv-node-type-${type || 'signal'}`
}

// ── helpers ──

function enrichNodeFromSources(label, state, timelineItems, evidence) {
  const cleaned = cleanChainLabel(label)
  const tlHit = (timelineItems || []).find((t) => labelsMatch(t.label, cleaned))
  if (tlHit) {
    return {
      type: mapTimelineItemType(tlHit),
      timestamp: tlHit.timestamp,
      meta: formatNodeMeta(tlHit),
      count: tlHit.count || 1,
      sourceKind: tlHit.sourceKind,
      sourceName: tlHit.sourceName,
      navTab: tlHit.navTab || 'evidence',
      severity: tlHit.severity,
      findingIds: [],
    }
  }

  for (const ev of state?.timeline || []) {
    const evLabel = cleanChainLabel(ev.reason || ev.message || '')
    if (labelsMatch(evLabel, cleaned)) {
      return {
        type: inferNodeType(ev.reason || ev.message, ev.type),
        timestamp: ev.timestamp,
        meta: [ev.sourceKind, ev.sourceName].filter(Boolean).join('/'),
        count: 1,
        sourceKind: ev.sourceKind,
        sourceName: ev.sourceName,
        navTab: 'evidence',
        severity: ev.severity,
        findingIds: [],
      }
    }
  }

  const evHit = rankEvidenceForChain(evidence).find((e) =>
    labelsMatch(e.reason || e.message, cleaned),
  )
  if (evHit) {
    return {
      type: inferNodeType(evHit.reason, evHit.sourceType),
      timestamp: evHit.timestamp,
      meta: evHit.sourceName || evHit.pod || '',
      count: 1,
      sourceKind: evHit.sourceKind,
      sourceName: evHit.sourceName || evHit.pod,
      navTab: 'evidence',
      severity: evHit.severity,
      findingIds: [],
    }
  }

  return {
    type: inferNodeType(cleaned),
    timestamp: null,
    meta: '',
    count: 1,
    sourceKind: null,
    sourceName: null,
    navTab: 'evidence',
    severity: null,
    findingIds: [],
  }
}

function linkFindingsToNodes(nodes, findings) {
  for (const f of findings) {
    const ids = []
    for (const node of nodes) {
      if (f.chainSteps?.some((step) => labelsMatch(step, node.label))) ids.push(node.id)
      if (labelsMatch(f.title, node.label)) ids.push(node.id)
    }
    if (ids.length) f.chainNodeIds = [...new Set([...(f.chainNodeIds || []), ...ids])]
  }
}

function inferNodeType(text, sourceType) {
  const t = String(text || '').toLowerCase()
  const st = String(sourceType || '').toLowerCase()

  if (/oom|crash|backoff|fail|error|refused|unhealthy|not ready|readiness|killed/.test(t)) {
    return 'failure'
  }
  if (/pressure|memory|cpu|metric|hypothesis|signal|saturation/.test(t)) {
    return 'signal'
  }
  if (/ready.*→|status|phase|became|changed/.test(t)) {
    return 'state'
  }
  if (st === 'log') return 'evidence'
  if (st === 'k8s_event' || st === 'event' || t.includes('event')) return 'event'
  if (st === 'object_change' || st === 'change') return 'state'
  return 'signal'
}

function mapTimelineItemType(item) {
  const typeStr = String(item.type || '').toLowerCase()
  if (typeStr.includes('failure') || typeStr.includes('signal')) {
    return typeStr.includes('failure') ? 'failure' : 'signal'
  }
  return inferNodeType(item.label, item.type === 'EVENT' ? 'k8s_event' : item.type)
}

function rankEvidenceForChain(evidence) {
  return [...(evidence || [])]
    .filter((e) => severityRankTL(e.severity) >= 1 || e.reason)
    .sort((a, b) => {
      const sd = severityRankTL(b.severity) - severityRankTL(a.severity)
      if (sd) return sd
      return new Date(a.timestamp || 0) - new Date(b.timestamp || 0)
    })
}

function dedupeByLabel(items) {
  const seen = new Set()
  const out = []
  for (const item of items) {
    const key = slug(item.label)
    if (!key || seen.has(key)) continue
    seen.add(key)
    out.push(item)
  }
  return out
}

function labelsMatch(a, b) {
  if (!a || !b) return false
  const na = cleanChainLabel(String(a)).toLowerCase()
  const nb = cleanChainLabel(String(b)).toLowerCase()
  if (na === nb) return true
  if (na.includes(nb) || nb.includes(na)) return true
  return false
}

function cleanChainLabel(label) {
  return String(label || '')
    .replace(/\s+\+\d+[smhd](?:\d+[smhd])?$/i, '')
    .replace(/\s+\+\d+[smhd]$/i, '')
    .trim()
}

function shortenLabel(label) {
  const s = cleanChainLabel(label)
  if (s.length <= 28) return s
  return `${s.slice(0, 25)}…`
}

function shortenFindingTitle(title) {
  const s = String(title || '')
  const dash = s.indexOf(' — ')
  if (dash > 0 && dash < 40) return s.slice(0, dash)
  return shortenLabel(s)
}

function slug(s) {
  return String(s || '').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')
}

function formatNodeMeta(item) {
  const parts = []
  if (item.count > 1) parts.push(`${item.count} occurrences`)
  if (item.sourceKind && item.sourceName) parts.push(`${item.sourceKind}/${item.sourceName}`)
  return parts.join(' · ')
}

function countFromMeta(meta) {
  const m = String(meta || '').match(/(\d+)\s+occurrence/)
  return m ? Number(m[1]) : 1
}

function countRelatedEvidence(item, evidence) {
  if (!evidence?.length) return 0
  const label = cleanChainLabel(item.label).toLowerCase()
  return evidence.filter((e) => {
    const text = `${e.reason || ''} ${e.message || ''}`.toLowerCase()
    return text.includes(label) || label.includes((e.reason || '').toLowerCase())
  }).length
}

function formatTimeAxis(ts) {
  if (!ts) return '—'
  try {
    return new Date(ts).toLocaleTimeString(undefined, {
      hour: '2-digit',
      minute: '2-digit',
      hour12: false,
    })
  } catch {
    return '—'
  }
}

function computeTimeSpan(items) {
  const times = items.map((i) => new Date(i.timestamp || 0).getTime()).filter((t) => t > 0)
  if (times.length < 2) return null
  return { start: Math.min(...times), end: Math.max(...times) }
}

function formatRelationLabel(relation) {
  switch (relation) {
    case 'owns': return 'owns'
    case 'selects': return 'selects'
    case 'routesTo': return 'routes to'
    default: return relation || 'owns'
  }
}

export { formatTimeAxis, formatClock, NODE_MARKERS }
