/** Human-readable data freshness labels for browse surfaces. */

export function formatDataAge(updatedAtMs, { now = Date.now() } = {}) {
  if (!updatedAtMs || updatedAtMs <= 0) return ''
  const sec = Math.max(0, Math.floor((now - updatedAtMs) / 1000))
  if (sec < 5) return 'Live'
  if (sec < 60) return `Updated ${sec}s ago`
  const min = Math.floor(sec / 60)
  if (min < 60) return `Updated ${min}m ago`
  const hr = Math.floor(min / 60)
  return `Updated ${hr}h ago`
}

export function isDataStale(updatedAtMs, thresholdMs, { now = Date.now() } = {}) {
  if (!updatedAtMs || updatedAtMs <= 0) return false
  return now - updatedAtMs > thresholdMs
}
