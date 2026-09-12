export function ClusterConnectionBanner({
  connection,
  onReconnect,
  onDismiss,
  onOpenSettings,
  reconnectBusy = false,
}) {
  if (!connection?.showBanner) return null

  const tone = connection.tone || 'info'
  const busy = tone === 'info' || reconnectBusy

  return (
    <div
      className={`cluster-connection-banner tone-${tone}`}
      role={tone === 'error' ? 'alert' : 'status'}
      aria-live="polite"
      aria-busy={busy || undefined}
    >
      <div className="cluster-connection-banner-main">
        <span className="cluster-connection-banner-icon" aria-hidden="true">
          {busy ? <span className="cluster-connection-spinner" /> : null}
        </span>
        <div className="cluster-connection-banner-copy">
          <strong className="cluster-connection-banner-title">{connection.title}</strong>
          {connection.message && (
            <span className="cluster-connection-banner-message">{connection.message}</span>
          )}
          {connection.detail && (
            <span className="cluster-connection-banner-detail muted">{connection.detail}</span>
          )}
        </div>
      </div>
      <div className="cluster-connection-banner-actions">
        {connection.showSettings && onOpenSettings && (
          <button type="button" className="text-link-btn" onClick={onOpenSettings}>
            Kubernetes settings
          </button>
        )}
        {connection.showDismiss && onDismiss && (
          <button
            type="button"
            className="text-link-btn cluster-connection-dismiss-btn"
            onClick={onDismiss}
            disabled={reconnectBusy}
          >
            Dismiss
          </button>
        )}
        {connection.showRetry && onReconnect && (
          <button
            type="button"
            className="btn btn-outline cluster-connection-retry-btn"
            onClick={onReconnect}
            disabled={reconnectBusy}
          >
            {reconnectBusy ? 'Connecting…' : (connection.retryLabel || 'Reconnect')}
          </button>
        )}
      </div>
    </div>
  )
}
