/** Shared pattern explorer filters — used by sidebar and Patterns view. */

export const PATTERN_SIGNALS = [
  { id: 'restart', label: 'Restarts', re: /restart|backoff|crash/i },
  { id: 'memory', label: 'Memory', re: /oom|memory|alloc/i },
  { id: 'connect', label: 'Connectivity', re: /connect|refused|timeout|dial/i },
  { id: 'probe', label: 'Probes', re: /probe|readiness|liveness/i },
  { id: 'sched', label: 'Scheduling', re: /schedul|pending|affinity/i },
]

export function isEmergingTemplate(t) {
  if (!t) return false
  if (t.trend === '↑' || t.trend === 'up' || t.trend === 'rising') return true
  return typeof t.trendPct === 'number' && t.trendPct >= 50
}

export function isRecurringTemplate(t) {
  return (t?.count || 0) >= 3
}

export function matchesPatternSignal(t, signalId) {
  if (!signalId) return true
  const sig = PATTERN_SIGNALS.find((s) => s.id === signalId)
  if (!sig) return true
  return sig.re.test(String(t?.template || ''))
}

/**
 * @param {object[]} templates
 * @param {{ kind?: string|null, signal?: string|null }} [filter]
 */
export function filterPatternTemplates(templates, filter) {
  const list = templates || []
  if (!filter) return list

  let out = list
  if (filter.kind === 'recurring') {
    out = out.filter(isRecurringTemplate)
  } else if (filter.kind === 'emerging') {
    out = out.filter(isEmergingTemplate)
  }
  if (filter.signal) {
    out = out.filter((t) => matchesPatternSignal(t, filter.signal))
  }
  return out
}

export function resolvePatternTabKind(filter) {
  if (filter?.kind === 'events') return 'events'
  return 'logs'
}

export function patternExplorerCounts(patterns) {
  const logTpl = patterns?.templates || []
  const eventTpl = patterns?.eventTemplates || []
  const all = [...logTpl, ...eventTpl]
  return {
    total: logTpl.length + eventTpl.length,
    recurring: all.filter(isRecurringTemplate).length,
    emerging: all.filter(isEmergingTemplate).length,
    logs: logTpl.length,
    events: eventTpl.length,
  }
}

export function derivePatternSignalCounts(logTpl, eventTpl) {
  const all = [...(logTpl || []), ...(eventTpl || [])]
  return PATTERN_SIGNALS
    .map((k) => ({
      ...k,
      count: all.filter((t) => k.re.test(String(t.template || ''))).length,
    }))
    .filter((k) => k.count > 0)
}

export function patternFilterDescription(filter, kind) {
  if (!filter) return null
  const parts = []
  if (filter.kind === 'recurring') parts.push('Recurring')
  else if (filter.kind === 'emerging') parts.push('Emerging')
  else if (filter.kind === 'logs') parts.push('Log patterns')
  else if (filter.kind === 'events') parts.push('Event patterns')

  if (filter.signal) {
    const sig = PATTERN_SIGNALS.find((s) => s.id === filter.signal)
    if (sig) parts.push(sig.label)
  }

  if (!parts.length) return null
  return parts.join(' · ')
}

export function applyPatternFilterToPayload(patterns, filter, kind) {
  if (!patterns) return null
  const tab = kind || resolvePatternTabKind(filter)
  if (tab === 'events') {
    return {
      ...patterns,
      eventTemplates: filterPatternTemplates(patterns.eventTemplates, filter),
    }
  }
  return {
    ...patterns,
    templates: filterPatternTemplates(patterns.templates, filter),
  }
}
