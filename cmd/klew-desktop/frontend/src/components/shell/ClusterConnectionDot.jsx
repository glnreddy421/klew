import { ClusterCloudIcon } from './ClusterCloudIcon.jsx'

/** Cluster reachability in the top bar — cloud online/offline (click to reconnect). */
export function ClusterConnectionDot({
  connection,
  onReconnect,
  reconnectBusy = false,
  contextLabel = '',
}) {
  if (!connection || connection.phase === 'idle') return null

  const { phase } = connection
  const cloudTone = phase === 'paused'
    ? 'muted'
    : phase === 'connected'
      ? 'ok'
      : (phase === 'connecting' || phase === 'retrying')
        ? 'warn'
        : 'crit'

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
        <ClusterCloudIcon
          tone={cloudTone}
          className={reconnectBusy ? 'is-busy' : ''}
          size={15}
        />
      </button>
    )
  }

  return (
    <span className="topbar-connection-dot" title={title} aria-label={label}>
      <ClusterCloudIcon tone={cloudTone} size={15} />
    </span>
  )
}
