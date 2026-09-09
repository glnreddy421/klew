/** Cluster reachability dot in the top bar — green online, red offline (click to reconnect). */
export function ClusterConnectionDot({
  connection,
  onReconnect,
  reconnectBusy = false,
  contextLabel = '',
}) {
  if (!connection || connection.phase === 'idle') return null

  const { phase } = connection
  const dotClass = phase === 'connected'
    ? 'ok'
    : (phase === 'connecting' || phase === 'retrying')
      ? 'warn'
      : 'crit'

  const label = phase === 'connected'
    ? `Connected to ${contextLabel}`
    : phase === 'connecting'
      ? `Connecting to ${contextLabel}…`
      : phase === 'retrying'
        ? `Reconnecting to ${contextLabel}…`
        : `${contextLabel} disconnected`

  const detail = connection.message || connection.detail || ''
  const title = detail ? `${label} — ${detail}` : label
  const canReconnect = (phase === 'disconnected' || phase === 'retrying') && onReconnect

  if (canReconnect) {
    return (
      <button
        type="button"
        className="topbar-connection-dot-btn"
        onClick={onReconnect}
        disabled={reconnectBusy}
        title={reconnectBusy ? 'Connecting…' : `${title}. Click to reconnect.`}
        aria-label={reconnectBusy ? 'Connecting to cluster' : `${label}. Reconnect.`}
      >
        <span
          className={`connection-dot ${dotClass} ${reconnectBusy ? 'is-busy' : ''}`}
          aria-hidden="true"
        />
      </button>
    )
  }

  return (
    <span className="topbar-connection-dot" title={title} aria-label={label}>
      <span className={`connection-dot ${dotClass}`} aria-hidden="true" />
    </span>
  )
}
