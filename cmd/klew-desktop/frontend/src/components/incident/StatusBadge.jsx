export function StatusBadge({ status, label }) {
  const s = status || 'unknown'
  const text = label || s.toUpperCase()
  const prefix = s === 'healthy' ? '● ' : ''
  return (
    <span className={`severity-pill pill-${s === 'degraded' ? 'warning' : s}`}>
      {prefix}{text}
    </span>
  )
}

export function RowStatusBadge({ status }) {
  const s = status || 'unknown'
  const labels = { healthy: 'Ready', degraded: 'Degraded', critical: 'Critical', unknown: '—' }
  return (
    <span className={`match-status-pill status-${s === 'degraded' ? 'warning' : s}`}>
      {labels[s] || s}
    </span>
  )
}
