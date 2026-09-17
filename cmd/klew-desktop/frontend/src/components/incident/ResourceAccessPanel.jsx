/**
 * Access-denied / unavailable / load-error state for a resource kind.
 */

/** @typedef {'list' | 'get'} RbacAction */

function accessMeta(kindGroup) {
  const api = kindGroup?.apiVersion
    || (kindGroup?.group ? `${kindGroup.group}/${kindGroup.version || 'v1'}` : '')
  const resource = kindGroup?.resource || kindGroup?.kind?.toLowerCase?.() || ''
  const scope = kindGroup?.namespaced === false ? 'Cluster' : 'Namespace'
  return { api, resource, scope }
}

function loadErrorMessage(kindGroup) {
  const fromCount = String(kindGroup?.countState?.error || '').trim()
  if (fromCount) return fromCount
  const fromAccess = String(kindGroup?.accessError || kindGroup?.error || '').trim()
  if (fromAccess) return fromAccess
  return 'The cluster API did not respond in time. Large clusters may need a retry or a longer network path (VPN / proxy).'
}

export function ResourceAccessPanel({
  kindGroup,
  action = 'list',
  onRetry,
  onReconnect,
  onOpenProxySettings,
  reconnectBusy = false,
}) {
  if (!kindGroup) return null

  const forbidden = kindGroup.accessState === 'forbidden'
    || kindGroup.countState?.state === 'forbidden'
  const loadError = kindGroup.accessState === 'error'
    || kindGroup.countState?.state === 'error'
  const unavailable = !forbidden && !loadError && (
    (!kindGroup.discovered && kindGroup.builtin && !kindGroup.discoveredOnly)
    || kindGroup.accessState === 'unavailable'
    || kindGroup.countState?.state === 'unavailable'
  )

  if (!forbidden && !unavailable && !loadError) return null

  const label = kindGroup.label || kindGroup.kind
  const meta = accessMeta(kindGroup)
  const capability = action === 'get' ? 'get' : 'list'
  const capabilityTarget = meta.resource || label.toLowerCase()
  const showActions = onRetry || onReconnect || onOpenProxySettings

  if (forbidden) {
    return (
      <div className="resource-access-panel">
        <div className="resource-access-icon" aria-hidden="true">🔒</div>
        <p className="resource-access-kicker">Access denied</p>
        <h4 className="resource-access-title">{label}</h4>
        <p className="resource-access-body">
          {action === 'get'
            ? 'The current identity cannot read this object.'
            : 'Klew discovered this Kubernetes resource, but the current identity cannot list it.'}
        </p>
        <dl className="resource-access-meta">
          {meta.api && <><dt>API</dt><dd className="mono">{meta.api}</dd></>}
          {meta.resource && <><dt>Resource</dt><dd className="mono">{meta.resource}</dd></>}
          <dt>Scope</dt><dd>{meta.scope}</dd>
          <dt>Required capability</dt>
          <dd className="mono">{capability} {capabilityTarget}</dd>
        </dl>
      </div>
    )
  }

  if (loadError) {
    return (
      <div className="resource-access-panel">
        <p className="resource-access-kicker">Could not load</p>
        <h4 className="resource-access-title">{label}</h4>
        <p className="resource-access-body">{loadErrorMessage(kindGroup)}</p>
        <dl className="resource-access-meta">
          <dt>API</dt><dd className="mono">{meta.api || '—'}</dd>
          <dt>Resource</dt><dd className="mono">{meta.resource || '—'}</dd>
          <dt>Scope</dt><dd>{meta.scope}</dd>
        </dl>
        {showActions && (
          <div className="resource-access-actions">
            {onRetry && (
              <button type="button" className="btn btn-outline btn-sm" onClick={onRetry}>
                Retry
              </button>
            )}
            {onReconnect && (
              <button
                type="button"
                className="btn btn-outline btn-sm"
                onClick={onReconnect}
                disabled={reconnectBusy}
              >
                {reconnectBusy ? 'Connecting…' : 'Reconnect'}
              </button>
            )}
            {onOpenProxySettings && (
              <button type="button" className="text-link-btn" onClick={onOpenProxySettings}>
                Proxy settings
              </button>
            )}
          </div>
        )}
      </div>
    )
  }

  return (
    <div className="resource-access-panel">
      <p className="resource-access-kicker">Not available</p>
      <h4 className="resource-access-title">{label}</h4>
      <p className="resource-access-body">
        This API group is not registered on the connected cluster (not found in API discovery).
      </p>
      <dl className="resource-access-meta">
        <dt>API</dt><dd className="mono">{meta.api || '—'}</dd>
        <dt>Resource</dt><dd className="mono">{meta.resource || '—'}</dd>
      </dl>
    </div>
  )
}
