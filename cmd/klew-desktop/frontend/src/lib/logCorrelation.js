/**
 * Phase 2 — correlate log atoms with timeline/K8s events and emit Signals.
 * Heuristic join on time window + object/pod identity + word overlap.
 */

import { extractLogPatterns } from './logPatterns.js'
import { severityRankTL } from './investigationViews.js'

const WINDOW_MS = 120_000 // ±2 minutes around an event

/**
 * Correlate live log evidence with timeline events.
 *
 * @returns {{
 *   signals: Array<object>,
 *   correlations: Array<{ eventKey: string, templateId: string, label: string, score: number }>,
 *   causalExtras: string[],
 *   byEventKey: Record<string, Array<object>>
 * }}
 */
export function correlateLogPatternsToTimeline(evidence, timeline, opts = {}) {
  const patterns = opts.patterns || extractLogPatterns(evidence, {
    pods: opts.pods,
    errorOnly: true,
    maxTemplates: opts.maxTemplates || 10,
    maxWords: opts.maxWords || 15,
  })

  const templates = patterns.templates || []
  const events = Array.isArray(timeline) ? timeline : []
  const signals = []
  const correlations = []
  const byEventKey = {}
  const causalExtras = []

  if (!templates.length || !events.length) {
    // Still emit standalone log signals when we have strong templates
    for (const tpl of templates.slice(0, 3)) {
      if (tpl.count < 2 && tpl.severity !== 'critical') continue
      signals.push(logOnlySignal(tpl))
    }
    return { signals, correlations, causalExtras, byEventKey, patterns }
  }

  for (const ev of events) {
    const evTs = toMs(ev.timestamp)
    if (!evTs) continue
    const evSev = severityRankTL(ev.severity)
    // Skip pure info noise unless it's a change/deploy marker
    const isChange = /deploy|rollout|scal|creat|image|replicaset/i.test(
      `${ev.type || ''} ${ev.reason || ''}`,
    )
    if (evSev < 2 && !isChange) continue

    const objName = objectName(ev)
    const evText = `${ev.reason || ''} ${ev.message || ''}`.toLowerCase()
    const eventKey = eventKeyOf(ev)

    for (const tpl of templates) {
      const link = scoreLink(ev, evTs, evText, objName, isChange, tpl)
      if (!link) continue

      correlations.push({
        eventKey,
        softKey: eventSoftKey(ev),
        templateId: tpl.id,
        label: link.label,
        score: link.score,
        template: tpl.template,
        count: tpl.count,
        pods: tpl.pods,
      })

      const soft = eventSoftKey(ev)
      const attachment = {
        templateId: tpl.id,
        template: tpl.template,
        count: tpl.count,
        trend: tpl.trend,
        severity: tpl.severity,
        score: link.score,
        label: link.label,
        samples: tpl.samples?.slice(0, 2) || [],
      }
      if (!byEventKey[eventKey]) byEventKey[eventKey] = []
      byEventKey[eventKey].push(attachment)
      if (!byEventKey[soft]) byEventKey[soft] = []
      // Dedupe soft-key attachments by template
      if (!byEventKey[soft].some((a) => a.templateId === tpl.id)) {
        byEventKey[soft].push(attachment)
      }
    }
  }

  // Rank correlations and build Signals (dedupe by template+best event)
  const bestByTpl = new Map()
  for (const c of correlations) {
    const prev = bestByTpl.get(c.templateId)
    if (!prev || c.score > prev.score) bestByTpl.set(c.templateId, c)
  }

  const ranked = [...bestByTpl.values()].sort((a, b) => b.score - a.score)
  for (const c of ranked.slice(0, 8)) {
    const tpl = templates.find((t) => t.id === c.templateId)
    if (!tpl) continue
    const strength = c.score >= 8 ? 'strong' : c.score >= 4 ? 'medium' : 'weak'
    const sev = tpl.severity === 'critical' || tpl.severity === 'high'
      ? tpl.severity
      : 'warning'
    signals.push({
      id: `corr_${c.templateId}`,
      label: c.label,
      severity: sev,
      strength,
      source: 'LOG+EVENT',
      count: tpl.count,
      score: c.score,
      confidence: Math.min(0.95, 0.4 + c.score * 0.05),
      evidence: `${tpl.template} · ${tpl.count}× · near ${shortEvent(c.eventKey)}`,
      templateId: c.templateId,
      eventKey: c.eventKey,
      pods: tpl.pods,
    })
  }

  // Causal extras: top correlated log atoms as story steps
  for (const s of signals.slice(0, 3)) {
    if (s.strength === 'strong' || s.strength === 'medium') {
      causalExtras.push(`Log: ${truncate(s.label, 72)}`)
    }
  }

  // Sort attached patterns per event
  for (const k of Object.keys(byEventKey)) {
    byEventKey[k].sort((a, b) => b.score - a.score)
  }

  return { signals, correlations, causalExtras, byEventKey, patterns }
}

function scoreLink(ev, evTs, evText, objName, isChange, tpl) {
  const times = (tpl.samples || [])
    .map((s) => toMs(s.timestamp))
    .filter(Boolean)
  // Use sample midpoint or assume "recent" if no timestamps
  let nearest = Infinity
  if (times.length) {
    for (const t of times) {
      const d = Math.abs(t - evTs)
      if (d < nearest) nearest = d
    }
  } else {
    nearest = WINDOW_MS / 2 // weak time match
  }
  if (nearest > WINDOW_MS) return null

  let score = 0
  // Closer in time → higher
  score += (1 - nearest / WINDOW_MS) * 4

  // Object / pod overlap
  const pods = tpl.pods || []
  if (objName && pods.some((p) => p === objName || p.startsWith(`${objName}-`) || objName.startsWith(p))) {
    score += 4
  } else if (objName && pods.some((p) => sharesToken(p, objName))) {
    score += 2
  }

  // Word overlap between event reason/message and template
  const overlap = wordOverlap(evText, tpl.template.toLowerCase())
  score += Math.min(4, overlap * 1.5)

  // Severity alignment
  if (severityRankTL(ev.severity) >= 2 && (tpl.severity === 'high' || tpl.severity === 'critical')) {
    score += 1.5
  }
  if (isChange && tpl.trend === '↑') score += 1

  // Volume
  if (tpl.count >= 10) score += 1
  if (tpl.count >= 50) score += 1

  if (score < 3.5) return null

  const label = buildLabel(ev, tpl)
  return { score, label }
}

function buildLabel(ev, tpl) {
  const reason = ev.reason || ev.type || 'event'
  const shortTpl = truncate(tpl.template, 56)
  return `${reason} ↔ ${shortTpl} (${tpl.count}×)`
}

function logOnlySignal(tpl) {
  return {
    id: `log_${tpl.id}`,
    label: truncate(tpl.template, 72),
    severity: tpl.severity === 'critical' || tpl.severity === 'high' ? tpl.severity : 'warning',
    strength: tpl.count >= 20 || tpl.severity === 'critical' ? 'strong' : 'medium',
    source: 'LOG',
    count: tpl.count,
    score: tpl.score || tpl.count,
    confidence: 0.55,
    evidence: `${tpl.template} · ${tpl.count}× across ${tpl.pods?.length || 0} pod(s)`,
    templateId: tpl.id,
    pods: tpl.pods,
  }
}

function objectName(ev) {
  const involved = ev.involvedObject?.name || ev.InvolvedObject?.name
  if (involved) return String(involved)
  if (ev.sourceName) return String(ev.sourceName)
  return ''
}

function eventKeyOf(ev) {
  return [
    ev.timestamp || '',
    ev.type || '',
    ev.reason || '',
    ev.sourceKind || '',
    ev.sourceName || '',
    (ev.message || '').slice(0, 40),
  ].join('|')
}

/** Soft key for attaching patterns to folded timeline runs (ignores timestamp). */
export function eventSoftKey(ev) {
  return [
    ev?.type || '',
    ev?.reason || '',
    ev?.sourceKind || '',
    ev?.sourceName || '',
    (ev?.message || '').slice(0, 40),
  ].join('|')
}

function shortEvent(eventKey) {
  const parts = String(eventKey || '').split('|')
  // full key: ts|type|reason|…  soft key: type|reason|…
  return parts.length > 3 ? (parts[2] || parts[1] || 'event') : (parts[1] || parts[0] || 'event')
}

function toMs(ts) {
  if (!ts) return 0
  const n = new Date(ts).getTime()
  return Number.isFinite(n) ? n : 0
}

function wordOverlap(a, b) {
  const wa = new Set(tokenize(a))
  const wb = tokenize(b)
  if (!wa.size || !wb.length) return 0
  let n = 0
  for (const w of wb) if (wa.has(w)) n++
  return n
}

function tokenize(s) {
  return String(s || '')
    .toLowerCase()
    .match(/[a-z][a-z0-9_-]{2,}/g) || []
}

function sharesToken(a, b) {
  const ta = new Set(String(a).toLowerCase().split(/[^a-z0-9]+/).filter((x) => x.length > 2))
  for (const t of String(b).toLowerCase().split(/[^a-z0-9]+/)) {
    if (t.length > 2 && ta.has(t)) return true
  }
  return false
}

function truncate(s, n) {
  const t = String(s || '')
  return t.length <= n ? t : `${t.slice(0, n - 1)}…`
}
