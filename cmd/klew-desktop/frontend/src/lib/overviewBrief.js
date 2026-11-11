/**
 * Investigation Overview brief: the answer first (what is broken and why),
 * then what changed, the warning patterns and what to check next. Built from
 * the engine state only; routine lifecycle events never reach this view.
 */
import { getState, getSnapshot, isTerminalPodSuccess, podHealthLabel, confidenceLabel } from './investigationViews.js'
import { incidentRows } from './incidents.js'
import { buildInspectKey } from './matches.js'
import { isRoutine, reasonOf } from './findingRanking.js'

const MAX_FAILING = 8
const MAX_PATTERNS = 5
const MAX_CHECKS = 4

const SEVERITY_RANK = { critical: 0, high: 1, warning: 2, info: 3 }

/** Pattern trend glyphs from the miner, as words. */
const TREND_WORDS = { '↑': 'rising', '↓': 'falling', '·': 'steady', new: 'new' }

/** Reason a pod is unhealthy, most specific first: waiting reason, last crash, phase. */
export function podFailureReason(pod) {
  const containers = pod?.containers || []
  for (const c of containers) {
    if (c.state === 'waiting' && c.reason) {
      const last = c.lastReason && c.lastReason !== c.reason ? c.lastReason : ''
      return { reason: c.reason, detail: lastDetail(last, c.lastExitCode), container: c.name }
    }
  }
  for (const c of containers) {
    if (c.state === 'terminated' && c.reason && c.reason !== 'Completed') {
      return { reason: c.reason, detail: exitDetail(c.exitCode), container: c.name }
    }
  }
  for (const c of containers) {
    if (c.lastReason && c.lastReason !== 'Completed') {
      return { reason: 'Restarting', detail: lastDetail(c.lastReason, c.lastExitCode), container: c.name }
    }
  }
  if (pod?.unschedulable) return { reason: 'Pending', detail: pod.unschedulable, container: '' }
  const phase = String(pod?.phase || '')
  if (phase && phase !== 'Running') return { reason: phase, detail: '', container: '' }
  if (pod?.ready === false) {
    const notReady = containers.filter((c) => !c.ready).map((c) => c.name)
    return { reason: 'Not ready', detail: notReady.length ? `containers: ${notReady.join(', ')}` : '', container: '' }
  }
  return { reason: 'Unhealthy', detail: '', container: '' }
}

function exitDetail(code) {
  return code ? `exit ${code}` : ''
}

function lastDetail(reason, code) {
  if (!reason) return exitDetail(code)
  return code ? `last: ${reason}, exit ${code}` : `last: ${reason}`
}

/** Unhealthy pods, worst first: critical, then most restarts. Succeeded pods never count. */
export function failingPods(pods) {
  return (pods || [])
    .filter((p) => !isTerminalPodSuccess(p) && podHealthLabel(p) !== 'healthy')
    .map((p) => {
      const why = podFailureReason(p)
      return {
        key: buildInspectKey('Pod', p.name, p.namespace),
        kind: 'Pod',
        name: p.name,
        namespace: p.namespace,
        severity: podHealthLabel(p),
        reason: why.reason,
        detail: why.detail,
        restarts: p.restartCount || 0,
        createdAt: p.createdAt || null,
      }
    })
    .sort((a, b) => {
      const sa = a.severity === 'critical' ? 0 : 1
      const sb = b.severity === 'critical' ? 0 : 1
      if (sa !== sb) return sa - sb
      if (a.restarts !== b.restarts) return b.restarts - a.restarts
      return a.name.localeCompare(b.name)
    })
}

/** Heat map cell state: failing (critical), degraded (warning), healthy, or done (finished Job pod). */
function cellState(pod) {
  if (isTerminalPodSuccess(pod)) return 'done'
  const health = podHealthLabel(pod)
  if (health === 'critical') return 'failing'
  if (health === 'healthy') return 'healthy'
  return 'degraded'
}

/** The workload a pod belongs to: Deployment (via its ReplicaSet), else its direct owner. */
export function podWorkload(pod) {
  const owner = (pod?.ownerRefs || [])[0]
  if (!owner) return { kind: 'Pod', name: 'Standalone pods' }
  const hash = pod?.labels?.['pod-template-hash']
  if (owner.kind === 'ReplicaSet' && hash && owner.name.endsWith(`-${hash}`)) {
    return { kind: 'Deployment', name: owner.name.slice(0, -(hash.length + 1)) }
  }
  return { kind: owner.kind, name: owner.name }
}

const STATE_ORDER = { failing: 0, degraded: 1, healthy: 2, done: 3 }

/**
 * Pod heat map for the investigated namespace: one cell per pod, grouped by
 * workload, worst groups first, plus totals per state for the impact bar.
 */
export function podHeatmap(pods) {
  const groups = new Map()
  const counts = { failing: 0, degraded: 0, healthy: 0, done: 0 }
  for (const p of pods || []) {
    const state = cellState(p)
    counts[state] += 1
    const wl = podWorkload(p)
    const id = `${wl.kind}/${wl.name}`
    if (!groups.has(id)) groups.set(id, { id, kind: wl.kind, name: wl.name, cells: [] })
    const why = state === 'healthy' || state === 'done' ? null : podFailureReason(p)
    groups.get(id).cells.push({
      key: buildInspectKey('Pod', p.name, p.namespace),
      name: p.name,
      state,
      reason: why ? [why.reason, why.detail].filter(Boolean).join(' · ') : (state === 'done' ? 'Completed' : 'Ready'),
      restarts: p.restartCount || 0,
    })
  }
  const worst = (g) => Math.min(...g.cells.map((c) => STATE_ORDER[c.state]))
  const list = [...groups.values()]
    .map((g) => ({ ...g, cells: [...g.cells].sort((a, b) => STATE_ORDER[a.state] - STATE_ORDER[b.state] || a.name.localeCompare(b.name)) }))
    .sort((a, b) => worst(a) - worst(b) || a.name.localeCompare(b.name))
  const live = counts.failing + counts.degraded + counts.healthy
  return {
    groups: list,
    counts,
    total: (pods || []).length,
    impactedPct: live ? Math.round(((counts.failing + counts.degraded) / live) * 100) : 0,
  }
}

/** Warning and worse event and log patterns, ranked by count; Normal/info noise is dropped. */
export function warningPatterns(logPatterns) {
  const lp = logPatterns || {}
  const tag = (list, source) => (list || []).map((t) => ({ ...t, source }))
  return [...tag(lp.eventTemplates, 'event'), ...tag(lp.templates, 'log')]
    .filter((t) => (SEVERITY_RANK[t.severity] ?? 3) < SEVERITY_RANK.info && t.count > 0)
    // Normal lifecycle events (Scheduled, Pulled, Started…) are not warnings, whatever their label.
    .filter((t) => t.source !== 'event' || !isRoutine(reasonOf(t.template), t.template))
    .sort((a, b) => {
      const sa = SEVERITY_RANK[a.severity] ?? 3
      const sb = SEVERITY_RANK[b.severity] ?? 3
      if (sa !== sb) return sa - sb
      return b.count - a.count
    })
    .slice(0, MAX_PATTERNS)
    .map((t) => ({
      id: `${t.source}-${t.id || t.template}`,
      text: t.template,
      count: t.count,
      trend: TREND_WORDS[t.trend] || t.trend || '',
      pods: (t.pods || []).length,
      source: t.source,
      severity: t.severity,
    }))
}

/** The engine's causal chain without routine lifecycle steps ("Completed +2h35m"). */
export function causalSteps(chain) {
  return stringList(chain).filter((step) => !isRoutine(reasonOf(step), step))
}

function stringList(v) {
  return Array.isArray(v) ? v.map((s) => String(s || '').trim()).filter(Boolean) : []
}

/** What changed before the trouble: the pinned change, then the most recent rollout. */
function recentChanges(state) {
  const out = []
  const pinned = state.pinnedChange
  if (pinned?.text) {
    const c = pinned.change || {}
    out.push({
      key: `pinned-${c.kind}-${c.name}-${c.revision}`,
      text: pinned.text,
      kind: c.kind,
      name: c.name,
      namespace: c.namespace,
      areas: c.areas || [],
      beforeIncident: Boolean(pinned.beforeIncident),
      at: c.time || null,
    })
  }
  const rc = state.recentChange
  if (rc && (rc.revisionTo || rc.image || rc.helmRelease)) {
    const parts = []
    if (rc.revisionFrom && rc.revisionTo) parts.push(`revision ${rc.revisionFrom} → ${rc.revisionTo}`)
    else if (rc.revisionTo) parts.push(`revision ${rc.revisionTo}`)
    if (rc.image) parts.push(`image ${rc.image}`)
    if (rc.helmRelease) parts.push(`Helm ${rc.helmRelease}${rc.helmRevision ? ` rev ${rc.helmRevision}` : ''}`)
    if (rc.rolloutStatus) parts.push(rc.rolloutStatus)
    const text = parts.join(' · ')
    if (text && !out.some((c) => c.text.includes(text))) {
      out.push({ key: 'recent', text, beforeIncident: false, at: rc.deployedAt || null })
    }
  }
  return out
}

/**
 * The Overview brief. `status` is failing (something is broken now), recovered
 * (healthy now, incidents earlier) or healthy.
 */
export function buildOverviewBrief(view) {
  const state = getState(view)
  const snapshot = getSnapshot(view)
  const verdict = state.verdict || {}
  const pods = snapshot.pods || []
  const failing = failingPods(pods)
  const live = pods.filter((p) => !isTerminalPodSuccess(p))
  const readyPods = live.filter((p) => p.ready).length

  const incidents = incidentRows(state.incidents)
  const ongoing = incidents.filter((i) => i.status === 'ongoing')
  const earlier = incidents.filter((i) => i.status !== 'ongoing')

  let status = 'healthy'
  if (failing.length || ongoing.length) status = 'failing'
  else if (earlier.length) status = 'recovered'

  const lead = failing[0]
  let headline
  if (lead) {
    headline = `${lead.name} · ${lead.reason}`
    if (failing.length > 1) headline += ` · ${failing.length - 1} more failing`
  } else if (ongoing.length) {
    headline = `${ongoing[0].reason} · ongoing`
  } else {
    headline = `Healthy now · ${readyPods}/${live.length} pods ready`
    if (earlier.length) headline += ` · ${earlier.length} recovered incident${earlier.length === 1 ? '' : 's'}`
  }

  const trigger = status === 'failing'
    ? String(verdict.likelyTrigger || verdict.leadingSignal || '').trim()
    : ''
  const conf = verdict.confidence || 0

  return {
    status,
    headline,
    trigger,
    confidenceText: status === 'failing' && conf > 0 ? confidenceLabel(conf) : '',
    heatmap: podHeatmap(pods),
    failing: failing.slice(0, MAX_FAILING),
    failingTotal: failing.length,
    podsTotal: live.length,
    podsReady: readyPods,
    chain: status === 'failing' ? causalSteps(state.causalChain) : [],
    changes: recentChanges(state),
    patterns: warningPatterns(state.logPatterns || view?.logPatterns),
    // Checks are written for a problem; on a healthy namespace they would contradict the verdict.
    nextChecks: status === 'failing'
      ? (stringList(state.nextChecks).length ? stringList(state.nextChecks) : stringList(verdict.recommendedNextChecks))
        .slice(0, MAX_CHECKS)
      : [],
    earlier,
    ongoing,
  }
}
