/**
 * Local event history (recorded Kubernetes Events) — display helpers.
 * History is never complete: always say when it starts and where the gaps are.
 */

export const RECORDING_STATES = new Set(['recording', 'other_window'])

export const RETENTION_DAY_OPTIONS = [1, 3, 7, 14, 30]
export const MAX_MB_OPTIONS = [100, 250, 500, 1000, 2000]

const DEFAULT_SETTINGS = { enabled: true, retentionDays: 7, maxMB: 500, eventTTLMinutes: 60 }

export const EVENT_TTL_MINUTE_OPTIONS = [15, 30, 60, 120, 180, 360, 720, 1440]

function toDate(v) {
  if (!v) return null
  const d = v instanceof Date ? v : new Date(v)
  return Number.isNaN(d.getTime()) ? null : d
}

function dateTimeFormat(opts, timeZone) {
  return new Intl.DateTimeFormat('en-US', { hour12: false, ...opts, ...(timeZone ? { timeZone } : {}) })
}

/** "Oct 18, 14:02" */
export function formatHistoryTime(value, { timeZone } = {}) {
  const d = toDate(value)
  if (!d) return ''
  const date = dateTimeFormat({ month: 'short', day: 'numeric' }, timeZone).format(d)
  const time = dateTimeFormat({ hour: '2-digit', minute: '2-digit' }, timeZone).format(d)
  return `${date}, ${time}`
}

function sameDay(a, b, timeZone) {
  const f = dateTimeFormat({ year: 'numeric', month: 'numeric', day: 'numeric' }, timeZone)
  return f.format(a) === f.format(b)
}

/** "Oct 18 16:00–19:00" or "Oct 18 16:00–Oct 19 09:00" */
export function formatGap(gap, { timeZone } = {}) {
  const from = toDate(gap?.from)
  const to = toDate(gap?.to)
  if (!from || !to) return ''
  const day = (d) => dateTimeFormat({ month: 'short', day: 'numeric' }, timeZone).format(d)
  const time = (d) => dateTimeFormat({ hour: '2-digit', minute: '2-digit' }, timeZone).format(d)
  if (sameDay(from, to, timeZone)) return `${day(from)} ${time(from)}–${time(to)}`
  return `${day(from)} ${time(from)}–${day(to)} ${time(to)}`
}

/** "3d history", "5h history", "<1h history", or "" when nothing is recorded. */
export function historyDepthLabel(since, now = new Date()) {
  const d = toDate(since)
  if (!d) return ''
  const hours = (toDate(now).getTime() - d.getTime()) / 3_600_000
  if (hours < 1) return '<1h history'
  if (hours < 48) return `${Math.floor(hours)}h history`
  return `${Math.floor(hours / 24)}d history`
}

export function isRecordingStatus(status) {
  return RECORDING_STATES.has(status?.state)
}

function gapCountText(n) {
  if (!n) return ''
  return n === 1 ? '1 gap' : `${n} gaps`
}

/** Short reason for "Not recording — <reason>". */
export function notRecordingReason(status) {
  if (!status) return 'event history unavailable'
  switch (status.state) {
    case 'paused':
      return 'paused'
    case 'off':
      return status.reason || 'turned off'
    case 'not_recordable':
      return status.reason || 'RBAC denied'
    case 'reconnecting':
      return status.reason ? `reconnecting (${status.reason})` : 'reconnecting'
    case 'starting':
      return 'starting'
    default:
      return status.reason || 'not recording'
  }
}

/**
 * Top-bar tooltip:
 *   "Recording events · history since Oct 18, 14:02 · 2 gaps"
 *   "Not recording — paused"
 */
export function recordingTooltip(status, { timeZone } = {}) {
  if (!isRecordingStatus(status)) {
    const base = `Not recording — ${notRecordingReason(status)}`
    if (status?.since) return `${base} · history since ${formatSince(status.since, { timeZone })}`
    return base
  }
  const parts = [status.state === 'other_window' ? 'Recording events (another Klew window)' : 'Recording events']
  if (status.since) parts.push(`history since ${formatSince(status.since, { timeZone })}`)
  const gaps = gapCountText(status.gaps?.length || 0)
  if (gaps) parts.push(gaps)
  if (status.highVolume) parts.push('high event volume — recording Warnings only')
  if (status.scope === 'namespaces' && status.reason) parts.push(status.reason)
  return parts.join(' · ')
}

/** Dot tone for the top bar indicator. */
export function recordingTone(status) {
  if (isRecordingStatus(status)) return status.highVolume ? 'warn' : 'ok'
  if (status?.state === 'reconnecting' || status?.state === 'starting') return 'warn'
  return 'muted'
}

/** Compact per-context label for Home list and context popover: "Recording · 3d history". */
export function contextRecordingLabel(status, now = new Date()) {
  if (!status) return ''
  const head = isRecordingStatus(status) ? 'Recording' : 'Not recording'
  const depth = historyDepthLabel(status.since, now)
  return depth ? `${head} · ${depth}` : head
}

/** Hide the label for contexts that are neither recorded nor have history. */
export function shouldShowContextRecording(status) {
  if (!status) return false
  return isRecordingStatus(status) || Boolean(status.active || status.favorite || status.since)
}

export function statusByContext(list) {
  const out = {}
  for (const s of list || []) {
    if (s?.context) out[s.context] = s
  }
  return out
}

/** Why recording stopped before a gap (backend coverage end reasons). */
export const GAP_REASON_LABELS = {
  klew_closed: 'Klew closed',
  monitoring_paused: 'monitoring paused',
  cluster_unreachable: 'cluster unreachable',
  recording_off: 'recording off',
  rbac_denied: 'RBAC denied',
  context_inactive: 'context not recorded',
  system_sleep: 'Mac asleep',
}

/** "Not recorded · Mac asleep", or plain "Not recorded" when the reason is unknown. */
export function gapReasonLabel(reason) {
  const why = GAP_REASON_LABELS[reason]
  return why ? `Not recorded · ${why}` : 'Not recorded'
}

/** "09:09" for today, "Oct 18, 14:02" otherwise. */
export function formatSince(value, { now = new Date(), timeZone } = {}) {
  const d = toDate(value)
  if (!d) return ''
  if (sameDay(d, toDate(now), timeZone)) {
    return dateTimeFormat({ hour: '2-digit', minute: '2-digit' }, timeZone).format(d)
  }
  return formatHistoryTime(d, { timeZone })
}

/** Popover rows: { key, range: "Oct 21 15:09–15:25", label: "Not recorded · Klew closed" }. */
export function gapRows(history, { timeZone } = {}) {
  return (history?.gaps || [])
    .map((g, i) => ({ key: `${g.from}-${i}`, range: formatGap(g, { timeZone }), label: gapReasonLabel(g.reason) }))
    .filter((g) => g.range)
}

/**
 * One compact coverage line:
 *   "Includes recorded events · history since 09:09"
 *   "Includes recorded events · history since 09:09 · 2 gaps"
 * With no prefix it starts "History since 09:09". The gap list goes in a popover.
 */
export function coverageSummary(history, { prefix = '', now = new Date(), timeZone } = {}) {
  if (!history?.since) return ''
  const since = formatSince(history.since, { now, timeZone })
  const parts = prefix ? [prefix, `history since ${since}`] : [`History since ${since}`]
  const gaps = gapCountText(gapRows(history, { timeZone }).length)
  if (gaps) parts.push(gaps)
  return parts.join(' · ')
}

/** Overview / Failures / Evidence line — only when the investigation used recorded events. */
export function evidenceHistoryLine(eventHistory, opts = {}) {
  if (!eventHistory?.recordedEvents) return ''
  return coverageSummary(eventHistory, { ...opts, prefix: 'Includes recorded events' })
}

/** Inspector Events header: "History since 09:09 · 1 gap". */
export function historySinceLine(history, opts = {}) {
  return coverageSummary(history, opts)
}

export function formatBytes(n) {
  const v = Number(n) || 0
  if (v < 1024) return `${v} B`
  if (v < 1024 * 1024) return `${(v / 1024).toFixed(0)} KB`
  if (v < 1024 * 1024 * 1024) return `${(v / (1024 * 1024)).toFixed(1)} MB`
  return `${(v / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

/** Settings form state from the backend payload (tolerates missing fields). */
export function normalizeHistorySettings(raw) {
  const s = { ...DEFAULT_SETTINGS, available: false, sizeBytes: 0, ...(raw || {}) }
  const days = Number(s.retentionDays)
  const mb = Number(s.maxMB)
  const ttl = Number(s.eventTTLMinutes)
  return {
    available: Boolean(s.available),
    enabled: s.enabled !== false,
    retentionDays: Number.isFinite(days) && days > 0 ? Math.min(90, Math.round(days)) : DEFAULT_SETTINGS.retentionDays,
    maxMB: Number.isFinite(mb) && mb > 0 ? Math.round(mb) : DEFAULT_SETTINGS.maxMB,
    eventTTLMinutes: Number.isFinite(ttl) && ttl > 0
      ? Math.min(1440, Math.max(5, Math.round(ttl)))
      : DEFAULT_SETTINGS.eventTTLMinutes,
    sizeBytes: Number(s.sizeBytes) || 0,
    error: s.error || '',
  }
}

/** Options for a select, always including the current value. */
export function optionsWithValue(options, value) {
  return options.includes(value) ? options : [...options, value].sort((a, b) => a - b)
}

/** Settings → per-context rows: "docker-desktop — Recording · since Oct 18, 14:02 · 1 gap". */
export function settingsContextRows(statuses, { timeZone } = {}) {
  return (statuses || [])
    .filter((s) => s?.context)
    .map((s) => {
      const parts = [isRecordingStatus(s) ? 'Recording' : `Not recording — ${notRecordingReason(s)}`]
      parts.push(s.since ? `since ${formatHistoryTime(s.since, { timeZone })}` : 'no history yet')
      const gaps = gapCountText(s.gaps?.length || 0)
      if (gaps) parts.push(gaps)
      return { context: s.context, text: parts.join(' · '), recording: isRecordingStatus(s) }
    })
}

/** "1 hour", "90 minutes", "6 hours". */
export function formatTTL(minutes) {
  const m = Number(minutes) || 0
  if (m >= 60 && m % 60 === 0) return m === 60 ? '1 hour' : `${m / 60} hours`
  return `${m} minutes`
}
