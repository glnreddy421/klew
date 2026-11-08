/** Log line helpers shared by the Live logs panel and Patterns. */

export const StreamMode = {
  Logs: 'logs',
  ErrorsOnly: 'errors',
  /** @deprecated Patterns live on the Patterns nav page */
  Patterns: 'patterns',
}

export const STREAM_MODES = [
  { id: StreamMode.Logs, label: 'All logs', hint: 'Container log lines from selected pods' },
  { id: StreamMode.ErrorsOnly, label: 'Errors only', hint: 'High/critical log lines only' },
]

const SOURCE_LOG = 'log'

/** Normalize selected pod names from filter state. */
export function normalizePodSelection(pods) {
  if (pods == null || pods === '') return []
  if (typeof pods === 'string') return [pods.trim()].filter(Boolean)
  if (!Array.isArray(pods)) return []
  return pods.map((p) => String(p || '').trim()).filter(Boolean)
}

/**
 * Resolve the pod name for a log/evidence row.
 * Prefers explicit fields; falls back to Go log_streamer message prefix "pod/container: …".
 */
export function resolveEventPod(e) {
  if (!e || typeof e !== 'object') return ''
  if (e.pod) return String(e.pod)
  if (e.Pod) return String(e.Pod)
  const sourceKind = e.sourceKind || e.SourceKind || ''
  const sourceName = e.sourceName || e.SourceName || ''
  if (sourceKind === 'Pod' && sourceName) return String(sourceName)
  const msg = String(e.message || e.Message || e.raw || e.Raw || '')
  const m = msg.match(/^([a-z0-9][a-z0-9.-]*)\/([a-z0-9][a-z0-9.-]*):\s/i)
  if (m) return m[1]
  const m2 = msg.match(/\bpod\/([a-z0-9][a-z0-9.-]*)\b/i)
  if (m2) return m2[1]
  return ''
}

/** Log line matches the live-tail text filter (regex if valid). */
export function logMatchesSearch(text, search) {
  const q = String(search || '').trim()
  if (!q) return true
  const hay = String(text || '')
  try {
    return new RegExp(q, 'i').test(hay)
  } catch {
    return hay.toLowerCase().includes(q.toLowerCase())
  }
}

/** Strip CSI/OSC color codes from klog/kyverno-style container logs. */
export function stripAnsi(s) {
  if (!s) return ''
  const str = String(s)
  if (!str.includes('\u001b') && !str.includes('\u009b')) return str
  return str
    .replace(/\u001b\[[0-9;?]*[a-zA-Z]/g, '')
    .replace(/\u009b[[0-9;?]*[a-zA-Z]/g, '')
    .replace(/\u001b\][^\u0007\u001b]*(?:\u0007|\u001b\\)/g, '')
}

/**
 * Static tokens from a Drain3 template (masks/wildcards removed).
 * Used instead of a full-line regex because Drain3 joins tokens with spaces
 * while live lines still contain `:` / `=` (e.g. reflector.go:446 vs reflector.go <*>).
 */
export function significantTemplateTokens(template) {
  return String(template || '')
    .split(/<[^>\s]+>/)
    .join(' ')
    .split(/[\s,;=()[\]{}|"']+/)
    .map((t) => t.trim())
    .filter((t) => t.length >= 3 && t !== '*' && t !== '***')
}

/** True when message contains enough static template tokens. */
export function messageMatchesTemplate(message, template) {
  const tokens = significantTemplateTokens(template)
  if (!tokens.length) return false
  const hay = String(message || '').toLowerCase()
  let hits = 0
  for (const t of tokens) {
    if (hay.includes(t.toLowerCase())) hits++
  }
  const need = Math.min(tokens.length, Math.max(2, Math.ceil(tokens.length * 0.6)))
  return hits >= need
}

function normalizeLogMessage(e) {
  const pod = resolveEventPod(e)
  let msg = stripAnsi(e.message || e.Message || e.raw || e.Raw || '')
  if (pod) {
    const container = e.container || e.Container
    if (container) msg = msg.replace(`${pod}/${container}: `, '')
    msg = msg.replace(`${pod}: `, '')
  }
  return { pod: pod || '—', message: msg.trim() }
}

/**
 * Prefer backend Drain samples (exact mined lines), then live evidence token-match.
 */
export function matchEvidenceToTemplate(evidence, templateOrRow, selectedPods = [], limit = 24, opts = {}) {
  const template = typeof templateOrRow === 'string'
    ? templateOrRow
    : (templateOrRow?.template || '')
  const samples = typeof templateOrRow === 'object' && templateOrRow
    ? (templateOrRow.samples || [])
    : []
  // Default: container logs. Pass sourceTypes: ['k8s_event'] for Event Patterns.
  const allowTypes = opts.sourceTypes
    ? new Set(opts.sourceTypes)
    : new Set([SOURCE_LOG])

  const out = []
  const seen = new Set()

  const push = (id, time, pod, message, severity) => {
    const key = String(message || '').trim()
    if (!key || seen.has(key)) return
    seen.add(key)
    out.push({
      id,
      time: time || '—',
      pod: pod || '—',
      message: key,
      severity: severity || 'info',
    })
  }

  for (let i = 0; i < samples.length && out.length < limit; i++) {
    const s = samples[i]
    const msg = stripAnsi(s.message || s.Message || '')
    if (!msg) continue
    push(
      `sample-${i}-${s.timestamp || s.Timestamp || ''}`,
      formatTime(s.timestamp || s.Timestamp),
      s.pod || s.Pod || '—',
      msg,
      s.severity || s.Severity,
    )
  }
  if (out.length >= limit) return out

  const selected = normalizePodSelection(selectedPods)
  const podSet = selected.length ? new Set(selected) : null
  const sorted = [...(Array.isArray(evidence) ? evidence : [])].sort(
    (a, b) => new Date(b.timestamp || b.Timestamp || 0) - new Date(a.timestamp || a.Timestamp || 0),
  )

  for (const e of sorted) {
    if (out.length >= limit) break
    const t = e.sourceType || e.SourceType || SOURCE_LOG
    if (!allowTypes.has(t)) continue
    let pod = resolveEventPod(e)
    let message = stripAnsi(e.message || e.Message || e.raw || e.Raw || '')
    if (t === SOURCE_LOG) {
      const n = normalizeLogMessage(e)
      pod = n.pod
      message = n.message
    } else {
      // Events: compound [Reason] Message for token match against Drain templates.
      const reason = e.reason || e.Reason || ''
      if (reason && !message.startsWith('[')) {
        message = `[${reason}] ${message}`.trim()
      }
      if (!pod || pod === '') {
        pod = e.sourceName || e.SourceName || '—'
      }
    }
    if (podSet && (pod === '—' || !podSet.has(pod))) continue
    if (!messageMatchesTemplate(message, template)) continue
    push(
      e.id || e.ID || `${e.timestamp || ''}-${out.length}`,
      formatTime(e.timestamp || e.Timestamp),
      pod,
      message,
      e.severity || e.Severity,
    )
  }
  return out
}

function formatTime(ts) {
  if (!ts) return '—'
  return new Date(ts).toLocaleTimeString(undefined, {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  })
}

