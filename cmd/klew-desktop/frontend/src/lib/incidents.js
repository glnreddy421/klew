/**
 * Incidents (backend-grouped Warning bursts) and chain recovery for Overview.
 */

function toDate(v) {
  if (!v) return null
  const d = v instanceof Date ? v : new Date(v)
  return Number.isNaN(d.getTime()) ? null : d
}

function clock(d, timeZone) {
  return new Intl.DateTimeFormat('en-US', {
    hour: '2-digit', minute: '2-digit', hour12: false, ...(timeZone ? { timeZone } : {}),
  }).format(d)
}

/** "20:15–20:33", or "20:15" when start and end share the minute. */
export function formatClockRange(from, to, { timeZone } = {}) {
  const a = toDate(from)
  const b = toDate(to)
  if (!a && !b) return ''
  if (!a || !b) return clock(a || b, timeZone)
  const left = clock(a, timeZone)
  const right = clock(b, timeZone)
  return left === right ? left : `${left}–${right}`
}

/** Overview rows: newest first. */
export function incidentRows(incidents, opts = {}) {
  return (incidents || [])
    .filter((i) => i && (i.start || i.end))
    .map((i, idx) => ({
      key: `${i.start}-${i.leadingReason}-${idx}`,
      range: formatClockRange(i.start, i.end, opts),
      reason: i.leadingReason || 'Warning',
      status: i.recovered ? 'recovered' : 'ongoing',
      count: i.count || 0,
      recorded: Boolean(i.recorded),
      end: toDate(i.end)?.getTime() || 0,
    }))
    .sort((a, b) => b.end - a.end)
}

/** Time window the chain covers, from its node timestamps. */
export function chainWindow(chain, opts = {}) {
  const times = (chain?.nodes || []).map((n) => toDate(n.timestamp)).filter(Boolean)
  if (!times.length) return ''
  const min = new Date(Math.min(...times.map((t) => t.getTime())))
  const max = new Date(Math.max(...times.map((t) => t.getTime())))
  return formatClockRange(min, max, opts)
}

/**
 * A chain is "recovered" when every incident has recovered and none of its
 * steps is newer than the last incident (allowing one minute of clock skew).
 */
export function chainRecovered(chain, incidents) {
  const list = incidents || []
  if (!list.length || list.some((i) => !i.recovered)) return false
  const lastEnd = Math.max(...list.map((i) => toDate(i.end)?.getTime() || 0))
  const times = (chain?.nodes || []).map((n) => toDate(n.timestamp)?.getTime()).filter(Boolean)
  if (!times.length || !lastEnd) return false
  return Math.max(...times) <= lastEnd + 60_000
}
