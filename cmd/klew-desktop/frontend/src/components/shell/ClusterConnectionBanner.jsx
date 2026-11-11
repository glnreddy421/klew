import { useEffect, useState } from 'react'

const CONNECTING_BANNER_DELAY_MS = 2000

export function ClusterConnectionBanner({
  connection,
  onReconnect,
  onDismiss,
  onOpenSettings,
  onOpenProxySettings,
  reconnectBusy = false,
}) {
  // A quick connect (a context switch, a status check) shows no banner at all:
  // the tab's status dot covers it. The banner appears only if it drags on, so
  // it does not pop in and out and push the page around.
  const connecting = connection?.phase === 'connecting'
  const [slow, setSlow] = useState(false)
  useEffect(() => {
    if (!connecting) {
      setSlow(false)
      return undefined
    }
    const timer = window.setTimeout(() => setSlow(true), CONNECTING_BANNER_DELAY_MS)
    return () => window.clearTimeout(timer)
  }, [connecting])

  if (!connection?.showBanner || (connecting && !slow)) return null

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
        {onOpenProxySettings && (
          <button
            type="button"
            className="text-link-btn cluster-connection-proxy-btn"
            onClick={onOpenProxySettings}
            disabled={reconnectBusy}
          >
            Proxy settings
          </button>
        )}
      </div>
    </div>
  )
}
