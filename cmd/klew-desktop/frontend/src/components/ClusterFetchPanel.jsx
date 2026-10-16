import { LoadingState } from './LoadingSpinner.jsx'

/**
 * Loading placeholder with optional reconnect / proxy actions for slow cluster fetches.
 */
export function ClusterFetchPanel({
  loading = true,
  message = 'Loading…',
  detail = '',
  compact = false,
  className = '',
  onReconnect,
  onOpenProxySettings,
  reconnectBusy = false,
  showActions = true,
}) {
  const showActionRow = showActions && (onReconnect || onOpenProxySettings)

  return (
    <div
      className={[
        'cluster-fetch-panel',
        compact ? 'is-compact' : '',
        className,
      ].filter(Boolean).join(' ')}
      role={loading ? 'status' : undefined}
      aria-live="polite"
      aria-busy={loading || undefined}
    >
      {loading ? (
        <LoadingState message={message} compact={compact} />
      ) : null}
      {detail ? (
        <p className="cluster-fetch-detail muted">{detail}</p>
      ) : null}
      {showActionRow && (
        <ClusterFetchActions
          onReconnect={onReconnect}
          onOpenProxySettings={onOpenProxySettings}
          reconnectBusy={reconnectBusy}
        />
      )}
    </div>
  )
}

/** Reconnect / proxy actions for a slow or stuck cluster fetch. */
export function ClusterFetchActions({ onReconnect, onOpenProxySettings, reconnectBusy = false }) {
  if (!onReconnect && !onOpenProxySettings) return null
  return (
    <div className="cluster-fetch-actions">
      {onReconnect && (
        <button
          type="button"
          className="btn btn-outline btn-sm cluster-fetch-reconnect-btn"
          onClick={onReconnect}
          disabled={reconnectBusy}
        >
          {reconnectBusy ? 'Connecting…' : 'Reconnect'}
        </button>
      )}
      {onOpenProxySettings && (
        <button
          type="button"
          className="text-link-btn cluster-fetch-proxy-btn"
          onClick={onOpenProxySettings}
          disabled={reconnectBusy}
        >
          Proxy settings
        </button>
      )}
    </div>
  )
}
