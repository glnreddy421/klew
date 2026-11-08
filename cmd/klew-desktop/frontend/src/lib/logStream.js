/** Container log stream — pure helpers for the Live logs panel. */

import { logMatchesSearch } from './streamView'

/** Lines kept in the panel; matches the backend ring buffer. */
export const MAX_STREAM_LINES = 5000

/** Number of pod colours defined in style.css (.log-pod-c0 … c7). */
export const POD_COLORS = 8

/** Mirrors MaxLogStreams in cmd/klew-desktop/log_stream.go. */
export const MAX_LOG_TABS = 8

/** Stable colour slot for a pod/container source, so a pod keeps its colour. */
export function podColor(key) {
  // FNV-1a with a final mix, so names differing only in a suffix spread out.
  const s = String(key || '')
  let h = 0x811c9dc5
  for (let i = 0; i < s.length; i += 1) {
    h = Math.imul(h ^ s.charCodeAt(i), 0x01000193)
  }
  h ^= h >>> 15
  h = Math.imul(h, 0x2c1b3c6d)
  h ^= h >>> 12
  return (h >>> 0) % POD_COLORS
}

/** Key identifying one pod's container in the stream. */
export function sourceKey(line) {
  return `${line?.pod || ''}/${line?.container || ''}`
}

/** Milliseconds since the epoch for a line, or 0 when it has no usable timestamp. */
export function lineTime(line) {
  const t = Date.parse(line?.timestamp || '')
  return Number.isNaN(t) ? 0 : t
}

function compareLines(a, b) {
  return (a.t - b.t) || (a.seq - b.seq)
}

/**
 * Merge a batch into prev, keeping lines in log-time order (ties by arrival),
 * and drop the oldest beyond max. Each line gets a cached `t` (ms).
 * Pods replay their tails as separate blocks, so a batch can be older than prev.
 */
export function appendLines(prev, batch, max = MAX_STREAM_LINES) {
  if (!batch?.length) return prev
  const sorted = batch.map((l) => (l.t == null ? { ...l, t: lineTime(l) } : l)).sort(compareLines)
  let next
  if (!prev.length || compareLines(prev[prev.length - 1], sorted[0]) <= 0) {
    next = prev.concat(sorted)
  } else {
    next = new Array(prev.length + sorted.length)
    let i = 0
    let j = 0
    let k = 0
    while (i < prev.length && j < sorted.length) {
      next[k++] = compareLines(prev[i], sorted[j]) <= 0 ? prev[i++] : sorted[j++]
    }
    while (i < prev.length) next[k++] = prev[i++]
    while (j < sorted.length) next[k++] = sorted[j++]
  }
  return next.length > max ? next.slice(next.length - max) : next
}

const ERROR_SEVERITIES = new Set(['critical', 'high'])

/**
 * Filter lines for display.
 * include and exclude are regex (or plain text when the regex is invalid), case-insensitive.
 */
export function filterLines(lines, { include = '', exclude = '', errorsOnly = false, muted = null } = {}) {
  const inc = String(include || '').trim()
  const exc = String(exclude || '').trim()
  const mutedSet = muted && muted.size ? muted : null
  if (!inc && !exc && !errorsOnly && !mutedSet) return lines
  return lines.filter((l) => {
    if (mutedSet && mutedSet.has(sourceKey(l))) return false
    if (errorsOnly && !ERROR_SEVERITIES.has(l.severity)) return false
    if (inc && !logMatchesSearch(l.text, inc)) return false
    if (exc && logMatchesSearch(l.text, exc)) return false
    return true
  })
}

/** Distinct pod/container sources in arrival order, with line counts. */
export function streamSources(lines) {
  const byKey = new Map()
  for (const l of lines) {
    const key = sourceKey(l)
    const cur = byKey.get(key)
    if (cur) cur.count += 1
    else byKey.set(key, { key, pod: l.pod, container: l.container, namespace: l.namespace, count: 1 })
  }
  return [...byKey.values()]
}

const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']

/**
 * Local time: HH:MM:SS.mmm for today, "Mon D HH:MM:SS" for earlier days,
 * or '' for a missing timestamp.
 */
export function formatLineTime(ts, now = new Date()) {
  if (!ts) return ''
  const d = new Date(ts)
  if (Number.isNaN(d.getTime())) return ''
  const pad = (n, w = 2) => String(n).padStart(w, '0')
  const clock = `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
  if (d.toDateString() !== now.toDateString()) {
    return `${MONTHS[d.getMonth()]} ${d.getDate()} ${clock}`
  }
  return `${clock}.${pad(d.getMilliseconds(), 3)}`
}

/** One-line description of what is streaming, for the panel header. */
export function describeSources(sources = []) {
  if (!sources.length) return ''
  const first = sources[0]
  const label = first.namespace ? `${first.label} in ${first.namespace}` : first.label
  if (sources.length === 1) return first.follows ? `${label} (follows new pods)` : label
  return `${label} and ${sources.length - 1} more`
}

/** Targets for StartLogStream from pod names in one namespace. */
export function podTargets(namespace, podNames) {
  const pods = [...new Set((podNames || []).filter(Boolean))]
  if (!namespace || !pods.length) return []
  return [{ namespace, pods }]
}

/** Kinds whose pods can be streamed; workloads and Services follow their selector. */
export const STREAMABLE_KINDS = new Set(['Pod', 'Deployment', 'StatefulSet', 'DaemonSet', 'ReplicaSet', 'Job', 'Service'])

/** Mirrors logstream.MaxTargets in Go. */
export const MAX_STREAM_TARGETS = 20

export function canStreamKind(kind) {
  return STREAMABLE_KINDS.has(kind)
}

/**
 * StartLogStream targets for resources ({ kind, name, namespace }).
 * Pods in one namespace share a target; each workload or Service gets its own.
 * Returns { targets, skipped } where skipped counts resources that cannot stream
 * (other kinds, no namespace, or past the source limit).
 */
export function streamTargetsFor(resources = []) {
  const podsByNs = new Map()
  const workloads = []
  const seen = new Set()
  let skipped = 0
  for (const r of resources) {
    const kind = r?.kind
    const name = r?.name
    const namespace = r?.namespace
    if (!canStreamKind(kind) || !name || !namespace) {
      skipped += 1
      continue
    }
    const key = `${kind}/${namespace}/${name}`
    if (seen.has(key)) continue
    seen.add(key)
    if (kind === 'Pod') {
      if (!podsByNs.has(namespace)) podsByNs.set(namespace, [])
      podsByNs.get(namespace).push(name)
    } else {
      workloads.push({ namespace, kind, name })
    }
  }
  const all = [
    ...[...podsByNs].map(([namespace, pods]) => ({ namespace, pods })),
    ...workloads,
  ]
  const targets = all.slice(0, MAX_STREAM_TARGETS)
  return { targets, skipped: skipped + (all.length - targets.length) }
}

/** Tooltip for a Stream logs action on one resource. */
export function streamLogsTitle(kind, name) {
  if (kind === 'Pod') return `Stream logs from ${name} in the Live logs panel`
  return `Stream logs from every pod of ${kind}/${name}, including new ones`
}

/**
 * Canonical key for a stream spec ({ targets, container }) so opening the
 * exact same selection focuses the existing log tab instead of duplicating it.
 */
export function logSpecKey(spec = {}) {
  const targets = Array.isArray(spec?.targets) ? spec.targets : []
  const podsByNs = new Map()
  const parts = []
  for (const t of targets) {
    const ns = String(t?.namespace || '').trim()
    if (!ns) continue
    const kind = String(t?.kind || '').trim()
    const name = String(t?.name || '').trim()
    const sel = String(t?.labelSelector || '').trim()
    const rawPods = Array.isArray(t?.pods) ? t.pods : []
    const pods = kind === 'Pod' && name ? [...rawPods, name] : rawPods
    const cleanPods = pods.map((p) => String(p || '').trim()).filter(Boolean)
    if (cleanPods.length) {
      if (!podsByNs.has(ns)) podsByNs.set(ns, new Set())
      const set = podsByNs.get(ns)
      for (const p of cleanPods) set.add(p)
    } else if (kind && name) {
      parts.push(`${ns}|${kind}/${name}`)
    } else if (sel) {
      parts.push(`${ns}|sel:${sel}`)
    }
  }
  for (const [ns, set] of podsByNs.entries()) {
    parts.push(`${ns}|pods:${[...set].sort().join(',')}`)
  }
  const unique = [...new Set(parts)].sort()
  if (!unique.length) return ''
  const container = String(spec?.container || '').trim()
  return container ? `${unique.join(';')}#c:${container}` : unique.join(';')
}

/**
 * Tab title for a log stream in the shared dock strip:
 * - One pod → the pod name (e.g. "api-7f9c-x2")
 * - One workload or Service → "Deployment/checkout"
 * - Multiple items → "3 sources"
 */
export function logTabTitle(spec = {}, sources = []) {
  const targets = Array.isArray(spec?.targets) ? spec.targets : []
  const items = []
  for (const t of targets) {
    const kind = String(t?.kind || '').trim()
    const name = String(t?.name || '').trim()
    const sel = String(t?.labelSelector || '').trim()
    const rawPods = Array.isArray(t?.pods) ? t.pods : []
    const pods = kind === 'Pod' && name ? [...rawPods, name] : rawPods
    const cleanPods = [...new Set(pods.map((p) => String(p || '').trim()).filter(Boolean))]
    if (cleanPods.length) {
      for (const p of cleanPods) items.push(p)
    } else if (kind && name) {
      items.push(`${kind}/${name}`)
    } else if (sel) {
      items.push(sel)
    }
  }
  if (items.length === 1) return items[0]
  if (items.length > 1) return `${items.length} sources`

  if (Array.isArray(sources) && sources.length === 1) {
    const lbl = String(sources[0]?.label || '').trim()
    if (lbl.startsWith('Pod/')) return lbl.slice(4)
    const m = lbl.match(/^(\d+)\s+pods$/)
    if (m) return `${m[1]} sources`
    if (lbl) return lbl
  }
  if (Array.isArray(sources) && sources.length > 1) {
    return `${sources.length} sources`
  }
  return 'Logs'
}
