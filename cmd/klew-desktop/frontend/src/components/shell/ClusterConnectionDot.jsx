/** Dot tone for a connection phase: ok, warn, crit, or muted when paused. */
export function connectionTone(phase) {
  if (phase === 'paused') return 'muted'
  if (phase === 'connected') return 'ok'
  if (phase === 'connecting' || phase === 'retrying') return 'warn'
  return 'crit'
}

/** Cluster reachability in the top bar — a status dot (click to reconnect when offline). */
export function ClusterConnectionDot({
  connection,
  onReconnect,
  reconnectBusy = false,
  contextLabel = '',
}) {
  if (!connection || connection.phase === 'idle') return null

  const { phase } = connection
  const cloudTone = connectionTone(phase)

  const label = phase === 'paused'
    ? `${contextLabel} — monitoring paused`
    : phase === 'connected'
      ? `Connected to ${contextLabel}`
      : phase === 'connecting'
        ? `Connecting to ${contextLabel}…`
        : phase === 'retrying'
          ? `Reconnecting to ${contextLabel}…`
          : `${contextLabel} disconnected`

  const detail = connection.message || connection.detail || ''
  const title = detail ? `${label} — ${detail}` : label
  const canReconnect = (phase === 'disconnected' || phase === 'retrying' || phase === 'paused') && onReconnect

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
        <span className={`topbar-status-dot tone-${cloudTone}${reconnectBusy ? ' is-busy' : ''}`} aria-hidden="true" />
      </button>
    )
  }

  return (
    <span className="topbar-connection-dot" title={title} aria-label={label}>
      <span className={`topbar-status-dot tone-${cloudTone}`} aria-hidden="true" />
    </span>
  )
}
