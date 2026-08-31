/**
 * Investigation Brief view model — composes existing investigation outputs
 * for Overview. Does not duplicate backend detection logic.
 */

import {
  confidenceLabel,
  formatClock,
  getSnapshot,
  getState,
  buildTimelineRuns,
  rankedVerdictSignals,
  severityRankTL,
  podHealthLabel,
  investigationWindowLabel,
  asStringArray,
} from './investigationViews.js'
import {
  buildVisualChain,
  enrichTimelineVisual,
  enrichResourcePath,
  linkFindingsToChainNodes,
} from './investigationOverviewVisuals.js'
import { deriveMatchRows, getMatchedObjects, pickWorstRow } from './matches.js'

const STATUS_RANK = { critical: 3, degraded: 2, warning: 2, healthy: 0, unknown: 1 }

/**
 * @typedef {'empty'|'quiet'|'active'} OverviewPhase
 */

/**
 * @param {object} view
 * @param {{ rows?: object[], timeWindowLabel?: string, live?: boolean }} [opts]
 */
export function buildInvestigationOverview(view, opts = {}) {
  const state = getState(view)
  const summary = view?.summary || {}
  const verdict = state.verdict || {}
  const matches = getMatchedObjects(view)
  const rows = opts.rows || deriveMatchRows(view, matches)
  const worstRow = pickWorstRow(rows)
  const signals = view?.signals || []
  const evidence = view?.evidence || state.liveEvidence || []
  const timeline = state.timeline || []
  const patterns = view?.logPatterns || state.logPatterns || null

  const phase = detectPhase(view, rows, evidence, timeline, signals)
  const stats = buildStats(view, rows, evidence, patterns, signals)
  const verdictBlock = buildVerdict(view, rows, worstRow, phase, opts)
  const findings = selectFindings(view, rows, patterns, phase)
  const investigationChain = buildInvestigationChain(state, findings)
  const timelineItems = selectTimelineItems(view, evidence)
  const affectedResources = selectAffectedResources(view, rows, verdict)
  const relationshipPath = buildRelationshipPath(view, worstRow, rows)
  const evidencePreview = selectEvidencePreview(evidence)
  const visibilityWarning = buildVisibilityWarning(view)
  const recentObservations = phase === 'quiet' ? selectRecentObservations(view, evidence, timeline) : []
  const nextPaths = buildNextPaths(view, findings, phase)

  const visualChain = phase === 'active'
    ? buildVisualChain({ state, findings, timelineItems, evidence, view })
    : null
  const timelineVisual = enrichTimelineVisual(timelineItems, state, evidence)
  const resourcePathVisual = enrichResourcePath(relationshipPath, rows, view)
  let findingsLinked = findings
  if (visualChain) {
    findingsLinked = linkFindingsToChainNodes(findings, visualChain)
  }

  return {
    phase,
    verdict: verdictBlock,
    stats,
    findings: findingsLinked,
    investigationChain,
    visualChain,
    timeline: timelineItems,
    timelineVisual,
    affectedResources,
    relationshipPath,
    resourcePathVisual,
    evidencePreview,
    visibilityWarning,
    recentObservations,
    nextPaths,
    windowLabel: opts.timeWindowLabel || investigationWindowLabel(state),
    live: opts.live ?? summary.live ?? false,
  }
}

function detectPhase(view, rows, evidence, timeline, signals) {
  const matches = getMatchedObjects(view)
  const snap = getSnapshot(view)
  const hasScope = matches.length > 0 || rows.length > 0
  const hasActivity = evidence.length > 0
    || timeline.length > 0
    || signals.length > 0
    || (snap.pods?.length ?? 0) > 0

  if (!hasScope && !hasActivity) return 'empty'

  const unhealthy = rows.filter((r) => r.status && r.status !== 'healthy')
  const state = getState(view)
  const verdict = state.verdict || {}
  const status = String(verdict.status || view?.summary?.status || '').toLowerCase()
  const failingPods = (snap.pods || []).filter((p) => {
    const h = podHealthLabel(p)
    return h === 'critical' || h === 'warning'
  }).length

  if (
    unhealthy.length === 0
    && failingPods === 0
    && (status === 'healthy' || status === 'ok' || !status)
    && signals.every((s) => !isHighSeverity(s.severity || s.level))
  ) {
    return 'quiet'
  }
  return 'active'
}

function isHighSeverity(sev) {
  const r = severityRankTL(sev)
  return r >= 2
}

function buildStats(view, rows, evidence, patterns, signals) {
  const snap = getSnapshot(view)
  const failingPods = (snap.pods || []).filter((p) => {
    const h = podHealthLabel(p)
    return h === 'critical' || h === 'warning'
  }).length
  const unhealthyRows = rows.filter((r) => r.status && r.status !== 'healthy').length
  const logTpl = patterns?.templates?.length || 0
  const eventTpl = patterns?.eventTemplates?.length || 0
  const patternCount = logTpl + eventTpl

  const affectedSet = new Set()
  for (const r of rows) {
    if (r.status && r.status !== 'healthy') affectedSet.add(r.key)
  }
  for (const p of snap.pods || []) {
    if (podHealthLabel(p) !== 'healthy') affectedSet.add(`Pod/${p.name}`)
  }

  return {
    signals: signals.length,
    failures: Math.max(unhealthyRows, failingPods),
    patterns: patternCount,
    resources: affectedSet.size || rows.length,
    evidence: evidence.length,
  }
}

function buildVerdict(view, rows, worstRow, phase, opts) {
  const state = getState(view)
  const summary = view?.summary || {}
  const verdict = state.verdict || {}
  const conf = summary.confidence || verdict.confidence || 0

  let statusLabel = 'Active investigation'
  let statusTone = 'investigating'

  if (phase === 'empty') {
    return {
      statusLabel: '',
      statusTone: 'neutral',
      headline: '',
      summary: '',
      confidence: null,
      confidenceText: null,
      startedAt: null,
      affectedCount: 0,
      evidenceCount: 0,
    }
  }

  if (phase === 'quiet') {
    statusLabel = 'Monitoring live signals'
    statusTone = 'quiet'
  } else if (String(verdict.status).toLowerCase() === 'critical' || worstRow?.status === 'critical') {
    statusLabel = 'Degradation detected'
    statusTone = 'critical'
  } else {
    statusLabel = 'Investigation active'
    statusTone = 'active'
  }

  let headline = ''
  let summaryText = ''

  if (phase === 'quiet') {
    headline = 'No significant failures detected'
    summaryText = buildQuietSummary(view, rows)
  } else {
    headline = firstNonEmpty(
      view?.hypothesis,
      state.hypothesisLabel,
      verdict.summary,
      summary.leadingSignal,
      buildRowHeadline(worstRow),
      'Investigation in progress',
    )
    summaryText = buildActiveSummary(state, verdict, worstRow, rows)
  }

  const evidenceCount = (view?.evidence || state.liveEvidence || []).length
  const affectedCount = countAffected(rows, verdict)

  return {
    statusLabel,
    statusTone,
    headline,
    summary: summaryText,
    confidence: conf > 0 ? conf : null,
    confidenceText: conf > 0 ? confidenceLabel(conf) : null,
    startedAt: earliestTimestamp(view),
    affectedCount,
    evidenceCount,
    live: opts.live ?? summary.live ?? false,
    windowLabel: opts.timeWindowLabel || investigationWindowLabel(state),
  }
}

function buildQuietSummary(view, rows) {
  const state = getState(view)
  const snap = getSnapshot(view)
  const parts = []
  const resourceCount = rows.length || matchesCount(view)
  if (resourceCount > 0) parts.push(`${resourceCount} resource${resourceCount === 1 ? '' : 's'} in scope`)
  const analyzed = state.counters?.eventsIngested
  if (analyzed != null && analyzed > 0) parts.push(`${analyzed} events analyzed`)
  const sigCount = (view?.signals || []).length
  if (sigCount > 0) parts.push(`${sigCount} signal${sigCount === 1 ? '' : 's'} observed`)
  if (!parts.length) {
    return 'Klew is analyzing the current investigation window.'
  }
  return `Klew is analyzing the current investigation window — ${parts.join(', ')}.`
}

function buildActiveSummary(state, verdict, worstRow, rows) {
  const reasons = asStringArray(state.hypothesisReasons)
  if (reasons.length) return reasons.slice(0, 2).join(' ')

  const correlation = asStringArray(state.correlation)
  if (correlation.length) return correlation.slice(0, 2).join(' ')

  if (verdict.likelyTrigger) return verdict.likelyTrigger

  const unhealthy = rows.filter((r) => r.status && r.status !== 'healthy')
  if (unhealthy.length > 1 && worstRow) {
    return `${worstRow.kind}/${worstRow.name} shows ${worstRow.signal || worstRow.status}; ${unhealthy.length - 1} other resource${unhealthy.length === 2 ? '' : 's'} affected.`
  }
  if (worstRow?.signal) {
    return `${worstRow.kind}/${worstRow.name} — ${worstRow.signal}.`
  }
  return ''
}

function viewCorrelation(state) {
  return asStringArray(state.correlation)
}

function buildRowHeadline(row) {
  if (!row) return ''
  const signal = row.signal || row.status
  if (signal && signal !== 'healthy') {
    return `${humanizeKind(row.kind)} ${row.name} — ${signal}`
  }
  return `${humanizeKind(row.kind)} ${row.name} requires attention`
}

function selectFindings(view, rows, patterns, phase) {
  if (phase === 'empty') return []

  const candidates = []
  const seen = new Set()

  for (const sig of rankedVerdictSignals(getState(view).verdict || {})) {
    const id = sig.id || sig.label
    if (!id || seen.has(id)) continue
    seen.add(id)
    candidates.push({
      id,
      title: sig.label || sig.evidence || 'Signal detected',
      meta: formatSignalMeta(sig),
      chainSteps: extractChainFromSignal(sig),
      severity: sig.severity,
      strength: sig.strength,
      score: findingScore({
        severity: sig.severity,
        strength: sig.strength,
        count: sig.count,
        resourceCount: sig.objectRef?.name ? 1 : 0,
      }),
      nav: { tab: 'evidence', label: 'View evidence →' },
    })
  }

  for (const tpl of (patterns?.templates || []).slice(0, 3)) {
    const id = `log-${tpl.id || tpl.template}`
    if (seen.has(id)) continue
    seen.add(id)
    candidates.push({
      id,
      title: shortenTemplate(tpl.template),
      meta: formatPatternMeta(tpl, 'log'),
      chainSteps: templateToChain(tpl.template),
      severity: tpl.severity === 'error' ? 'error' : tpl.severity === 'warn' ? 'warning' : 'info',
      score: findingScore({ severity: tpl.severity, count: tpl.count, resourceCount: tpl.pods?.length || 0 }),
      nav: { tab: 'patterns', label: 'View pattern →' },
    })
  }

  for (const tpl of (patterns?.eventTemplates || []).slice(0, 3)) {
    const id = `evt-${tpl.id || tpl.template}`
    if (seen.has(id)) continue
    seen.add(id)
    candidates.push({
      id,
      title: shortenTemplate(tpl.template),
      meta: formatPatternMeta(tpl, 'event'),
      chainSteps: templateToChain(tpl.template),
      severity: tpl.severity === 'error' ? 'error' : 'warning',
      score: findingScore({ severity: tpl.severity, count: tpl.count }),
      nav: { tab: 'patterns', label: 'View pattern →' },
    })
  }

  for (const row of rows) {
    if (!row.status || row.status === 'healthy') continue
    const id = `row-${row.key}`
    if (seen.has(id)) continue
    seen.add(id)
    candidates.push({
      id,
      title: `${row.kind}/${row.name} — ${row.signal || row.status}`,
      meta: formatRowMeta(row),
      chainSteps: row.signal ? [row.signal] : [],
      severity: row.status === 'critical' ? 'critical' : 'warning',
      score: findingScore({
        severity: row.status,
        count: row.restarts || 1,
        resourceCount: 1,
      }),
      nav: { tab: 'failures', label: 'View failure →' },
    })
  }

  candidates.sort((a, b) => b.score - a.score || a.title.localeCompare(b.title))
  return candidates.slice(0, 5).map((f, i) => ({ ...f, rank: i + 1 }))
}

function findingScore({ severity, strength, count = 1, resourceCount = 0 }) {
  let score = severityRankTL(severity) * 100
  if (strength === 'strong') score += 40
  else if (strength === 'medium') score += 20
  score += Math.min(count, 50) * 2
  score += resourceCount * 8
  return score
}

function formatSignalMeta(sig) {
  const parts = []
  if (sig.count > 1) parts.push(`${sig.count} occurrences`)
  if (sig.source) parts.push(sig.source)
  if (sig.objectRef?.name) {
    const k = sig.objectRef.kind || 'Resource'
    parts.unshift(`${k}/${sig.objectRef.name}`)
  }
  return parts.join(' · ') || sig.evidence || ''
}

function formatPatternMeta(tpl, kind) {
  const parts = []
  if (tpl.count) parts.push(`${tpl.count} occurrence${tpl.count === 1 ? '' : 's'}`)
  if (tpl.pods?.length) parts.push(`${tpl.pods.length} pod${tpl.pods.length === 1 ? '' : 's'}`)
  if (kind === 'event' && tpl.pct) parts.push(`${Number(tpl.pct).toFixed(0)}% of events`)
  return parts.join(' · ')
}

function formatRowMeta(row) {
  const parts = []
  if (row.restarts) parts.push(`${row.restarts} restart${row.restarts === 1 ? '' : 's'}`)
  if (row.ready != null && row.total != null) parts.push(`${row.ready}/${row.total} ready`)
  return parts.join(' · ') || row.matchBy || ''
}

function extractChainFromSignal(sig) {
  const text = `${sig.label || ''} ${sig.evidence || ''}`
  const steps = splitChainText(text)
  return steps.length >= 2 ? steps : []
}

function templateToChain(template) {
  if (!template) return []
  const parts = String(template)
    .split(/\s*(?:→|->|\|)\s*|\s+then\s+/i)
    .map((s) => s.replace(/\*\*\*/g, '…').trim())
    .filter(Boolean)
  return parts.length >= 2 && parts.length <= 6 ? parts : []
}

function splitChainText(text) {
  const lowered = text.toLowerCase()
  for (const sep of [' → ', ' -> ', ', then ', ' followed by ', ' preceded by ']) {
    if (lowered.includes(sep.trim())) {
      return text.split(new RegExp(sep.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'i'))
        .map((s) => s.trim())
        .filter(Boolean)
        .slice(0, 6)
    }
  }
  return []
}

function buildInvestigationChain(state, findings) {
  const causal = asStringArray(state.causalChain)
  if (causal.length >= 2) {
    return { steps: causal.slice(0, 6), connector: '→', label: 'Investigation chain' }
  }

  const correlation = asStringArray(state.correlation)
  if (correlation.length >= 2) {
    return {
      steps: correlation.slice(0, 6).map((c) => String(c).trim()),
      connector: 'correlated with',
      label: 'Correlated sequence',
    }
  }

  const topChain = findings.find((f) => f.chainSteps?.length >= 2)
  if (topChain) {
    return {
      steps: topChain.chainSteps.slice(0, 6),
      connector: '→',
      label: 'Observed sequence',
    }
  }

  const leading = state.verdict?.leadingSignal
  if (leading) {
    const steps = splitChainText(leading)
    if (steps.length >= 2) {
      return { steps, connector: '→', label: 'Leading signal' }
    }
  }

  return null
}

function selectTimelineItems(view, evidence) {
  const state = getState(view)
  const timeline = state.timeline || []

  if (timeline.length) {
    const mapped = timeline.map((ev) => ({
      timestamp: ev.timestamp,
      type: timelineTypeLabel(ev),
      label: formatTimelineLabel(ev),
      severity: ev.severity,
      sourceKind: ev.sourceKind,
      sourceName: ev.sourceName,
      navTab: timelineNavTab(ev),
    }))

    const { runs } = buildTimelineRuns(mapped.map((m) => ({
      timestamp: m.timestamp,
      type: m.type,
      severity: m.severity,
      reason: m.label,
      message: m.label,
      sourceKind: m.sourceKind,
      sourceName: m.sourceName,
    })))

    const scored = runs.map((r) => ({
      run: r,
      score: timelineRunScore(r),
    }))
    scored.sort((a, b) => b.score - a.score)

    const picked = new Set()
    const items = []
    for (const { run } of scored) {
      if (items.length >= 8) break
      const key = run.key
      if (picked.has(key)) continue
      picked.add(key)
      items.push({
        timestamp: run.first,
        type: timelineTypeLabel(run.ev),
        label: formatTimelineLabel(run.ev),
        count: run.count,
        annotation: run.annotation,
        navTab: timelineNavTab(run.ev),
      })
    }

    items.sort((a, b) => new Date(a.timestamp) - new Date(b.timestamp))
    return items
  }

  return selectTimelineFromEvidence(evidence, 8)
}

function timelineRunScore(run) {
  let score = severityRankTL(run.ev?.severity) * 100
  if (run.inChain) score += 40
  if (run.annotation) score += 25
  score += Math.min(run.count, 20)
  return score
}

function selectAffectedResources(view, rows, verdict) {
  const affected = []
  const seen = new Set()

  for (const ref of verdict.affectedObjects || []) {
    const key = `${ref.kind}/${ref.name}`
    if (seen.has(key)) continue
    seen.add(key)
    affected.push({
      key,
      kind: ref.kind,
      name: ref.name,
      detail: 'affected',
      status: 'critical',
      score: 100,
    })
  }

  for (const row of rows) {
    if (seen.has(row.key)) continue
    if (row.status && row.status !== 'healthy') {
      seen.add(row.key)
      affected.push({
        key: row.key,
        kind: row.kind,
        name: row.name,
        detail: row.signal || row.status,
        status: row.status,
        score: (STATUS_RANK[row.status] || 0) * 10 + (row.score || 0),
        signalCount: countSignalsForResource(view, row),
      })
    }
  }

  for (const row of rows) {
    if (seen.has(row.key)) continue
    if ((row.score || 0) > 0) {
      seen.add(row.key)
      affected.push({
        key: row.key,
        kind: row.kind,
        name: row.name,
        detail: row.signal || 'in scope',
        status: row.status || 'unknown',
        score: row.score || 0,
      })
    }
  }

  affected.sort((a, b) => b.score - a.score)
  return affected.slice(0, 8)
}

function buildRelationshipPath(view, worstRow, rows) {
  const graph = getState(view).workloadGraph || {}
  const nodes = graph.nodes || []
  const edges = graph.edges || []
  if (!nodes.length || !edges.length) return null

  const byId = Object.fromEntries(nodes.map((n) => [n.id, n]))
  const anchorCandidates = []

  if (worstRow) anchorCandidates.push(worstRow)
  for (const n of nodes) {
    if (n.kind === 'Pod' && (n.health === 'critical' || n.health === 'warning')) {
      anchorCandidates.push({ kind: n.kind, name: n.name })
    }
  }
  for (const n of nodes) {
    if (n.kind === 'Pod') anchorCandidates.push({ kind: n.kind, name: n.name })
  }
  for (const row of rows || []) {
    anchorCandidates.push({ kind: row.kind, name: row.name })
  }

  const seen = new Set()
  for (const anchor of anchorCandidates) {
    const key = `${anchor.kind}/${anchor.name}`
    if (seen.has(key)) continue
    seen.add(key)

    const anchorId = findNodeId(nodes, anchor.kind, anchor.name)
    if (!anchorId) continue

    const path = walkOwnershipPath(anchorId, edges, byId)
    if (path.length >= 2) {
      return formatRelationshipPath(path, edges)
    }
  }

  return null
}

function formatRelationshipPath(path, edges) {
  return path.map((step, i) => ({
    kind: step.kind,
    name: step.name,
    health: step.health,
    via: i > 0 ? path[i - 1].via : null,
  }))
}

function findNodeId(nodes, kind, name) {
  const hit = nodes.find((n) => n.kind === kind && n.name === name)
  if (hit) return hit.id
  return nodes.find((n) => n.name === name)?.id || null
}

function walkOwnershipPath(targetId, edges, byId) {
  const parentMap = new Map()
  for (const e of edges) {
    if (e.relation === 'owns' || e.relation === 'selects' || e.relation === 'routesTo') {
      parentMap.set(e.to, { from: e.from, via: e.relation, annotation: e.annotation })
    }
  }

  const chain = []
  let cur = targetId
  const visited = new Set()
  while (cur && !visited.has(cur)) {
    visited.add(cur)
    const node = byId[cur]
    if (node) chain.unshift({ kind: node.kind, name: node.name, id: node.id, health: node.health })
    const parent = parentMap.get(cur)
    if (!parent) break
    cur = parent.from
  }

  if (chain.length >= 2) {
    for (let i = 0; i < chain.length - 1; i++) {
      const e = edges.find((x) => x.from === chain[i].id && x.to === chain[i + 1].id)
      chain[i].via = e?.relation || 'owns'
    }
  }

  return chain
}

function selectEvidencePreview(evidence) {
  return [...(evidence || [])]
    .sort((a, b) => {
      const sd = severityRankTL(b.severity) - severityRankTL(a.severity)
      if (sd) return sd
      return new Date(b.timestamp || 0) - new Date(a.timestamp || 0)
    })
    .slice(0, 5)
    .map((e) => ({
      id: e.id,
      timestamp: e.timestamp,
      type: evidenceTypeLabel(e.sourceType),
      headline: e.reason || e.sourceKind || evidenceTypeLabel(e.sourceType),
      detail: truncate(e.message || e.raw || '', 140),
      sourceName: e.sourceName || e.pod,
      severity: e.severity,
    }))
}

function buildVisibilityWarning(view) {
  const state = getState(view)
  const snap = getSnapshot(view)
  const perms = state.permissions?.length ? state.permissions : (snap.permissions || [])
  const denied = perms.filter((p) => p && p.allowed === false)
  const warnings = [
    ...(state.warnings || []),
    ...(state.verdict?.missingDataWarnings || []),
    ...(snap.warnings || []),
  ]

  if (!denied.length && !warnings.length) return null

  const deniedResources = [...new Set(denied.map((p) => p.resource).filter(Boolean))]
  let message = warnings[0] || ''
  if (denied.length) {
    const list = deniedResources.slice(0, 3).join(', ')
    message = `Klew cannot access ${denied.length} resource type${denied.length === 1 ? '' : 's'} with the current Kubernetes identity${list ? ` (${list}${deniedResources.length > 3 ? ', …' : ''})` : ''}.`
  }

  return {
    title: 'Investigation visibility is limited',
    message,
  }
}

function selectRecentObservations(view, evidence, timeline) {
  const items = []

  for (const ev of (timeline || []).slice(-6)) {
    items.push({
      text: formatTimelineLabel(ev),
      timestamp: ev.timestamp,
    })
  }

  if (items.length < 4) {
    for (const e of [...(evidence || [])].slice(-8)) {
      if (severityRankTL(e.severity) >= 2) continue
      items.push({
        text: formatEvidenceLabel(e),
        timestamp: e.timestamp,
      })
    }
  }

  return items
    .sort((a, b) => new Date(b.timestamp || 0) - new Date(a.timestamp || 0))
    .slice(0, 5)
}

function buildNextPaths(view, findings, phase) {
  if (phase === 'empty') {
    return [
      { tab: 'resources', label: 'Browse resources' },
    ]
  }

  const paths = []
  const tabs = new Set()

  for (const f of findings) {
    if (f.nav?.tab && !tabs.has(f.nav.tab)) {
      tabs.add(f.nav.tab)
      paths.push({ tab: f.nav.tab, label: f.nav.label.replace(' →', '') })
    }
  }

  if (!tabs.has('graph')) paths.push({ tab: 'graph', label: 'Open Graph' })
  if (!tabs.has('evidence')) paths.push({ tab: 'evidence', label: 'View all evidence' })

  return paths.slice(0, 5)
}

function selectTimelineFromEvidence(evidence, limit) {
  return [...(evidence || [])]
    .filter((e) => severityRankTL(e.severity) >= 1 || e.reason)
    .sort((a, b) => new Date(a.timestamp || 0) - new Date(b.timestamp || 0))
    .slice(-limit)
    .map((e) => ({
      timestamp: e.timestamp,
      type: evidenceTypeLabel(e.sourceType),
      label: formatEvidenceLabel(e),
      count: e.count,
      navTab: 'evidence',
    }))
}

function formatTimelineLabel(ev) {
  if (!ev) return '—'
  const reason = ev.reason || ev.type || ''
  const msg = ev.message || ''
  if (reason && msg && !msg.toLowerCase().startsWith(reason.toLowerCase())) {
    return `${reason} — ${truncate(msg, 80)}`
  }
  return reason || truncate(msg, 100) || `${ev.sourceKind || 'Event'}/${ev.sourceName || ''}`
}

function formatEvidenceLabel(e) {
  const reason = e.reason || ''
  const msg = e.message || e.raw || ''
  if (reason && msg) return `${reason} · ${truncate(msg, 70)}`
  return reason || truncate(msg, 90) || 'Observation'
}

function timelineTypeLabel(ev) {
  const t = String(ev?.type || '').toLowerCase()
  if (t.includes('metric')) return 'METRIC'
  if (t.includes('hypothesis')) return 'SIGNAL'
  if (t === 'event' || t === 'k8s_event') return 'EVENT'
  if (t.includes('failure')) return 'FAILURE'
  if (ev?.sourceKind === 'Pod') return 'RESOURCE'
  return (ev?.type || 'EVENT').toUpperCase().slice(0, 12)
}

function timelineNavTab(ev) {
  const t = String(ev?.type || '').toLowerCase()
  if (t.includes('metric')) return 'resources'
  const reason = `${ev?.reason || ''} ${ev?.message || ''}`.toLowerCase()
  if (reason.includes('oom') || reason.includes('crash')) return 'failures'
  return 'evidence'
}

function evidenceTypeLabel(sourceType) {
  const t = String(sourceType || '').toLowerCase()
  if (t === 'log') return 'LOG'
  if (t === 'k8s_event' || t === 'event') return 'EVENT'
  if (t === 'object_change' || t === 'change') return 'CHANGE'
  if (t === 'metric' || t === 'metrics') return 'METRIC'
  if (t === 'system') return 'STATUS'
  return (sourceType || 'EVIDENCE').toUpperCase().slice(0, 10)
}

function earliestTimestamp(view) {
  const state = getState(view)
  const candidates = []
  for (const ev of state.timeline || []) {
    if (ev.timestamp) candidates.push(new Date(ev.timestamp).getTime())
  }
  for (const ev of view?.evidence || state.liveEvidence || []) {
    if (ev.timestamp) candidates.push(new Date(ev.timestamp).getTime())
  }
  if (!candidates.length) return null
  return new Date(Math.min(...candidates)).toISOString()
}

function countSignalsForResource(view, row) {
  const signals = view?.signals || []
  let n = 0
  for (const s of signals) {
    const ref = s.objectRef
    if (ref?.name === row.name && (!ref.kind || ref.kind === row.kind)) n++
  }
  return n
}

function countAffected(rows, verdict) {
  const keys = new Set()
  for (const r of rows) {
    if (r.status && r.status !== 'healthy') keys.add(r.key)
  }
  for (const ref of verdict.affectedObjects || []) {
    keys.add(`${ref.kind}/${ref.name}`)
  }
  return keys.size || rows.filter((r) => r.status !== 'healthy').length
}

function matchesCount(view) {
  return getMatchedObjects(view).length
}

function firstNonEmpty(...vals) {
  for (const v of vals) {
    if (v != null && String(v).trim()) return String(v).trim()
  }
  return ''
}

function humanizeKind(kind) {
  if (!kind) return 'Resource'
  return kind
}

function shortenTemplate(tpl) {
  if (!tpl) return 'Pattern detected'
  const s = String(tpl).replace(/\*\*\*/g, '…')
  return s.length > 72 ? `${s.slice(0, 69)}…` : s
}

function truncate(s, n) {
  if (!s) return ''
  const t = String(s).replace(/\s+/g, ' ').trim()
  return t.length > n ? `${t.slice(0, n - 1)}…` : t
}

export { formatClock, evidenceTypeLabel, timelineTypeLabel }
