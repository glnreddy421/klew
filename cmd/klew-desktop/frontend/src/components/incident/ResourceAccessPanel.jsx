/**
 * Access-denied / unavailable state for a resource kind.
 */

function accessMeta(kindGroup) {
  const api = kindGroup?.apiVersion
    || (kindGroup?.group ? `${kindGroup.group}/${kindGroup.version || 'v1'}` : '')
  const resource = kindGroup?.resource || kindGroup?.kind?.toLowerCase?.() || ''
  const scope = kindGroup?.namespaced === false ? 'Cluster' : 'Namespace'
  return { api, resource, scope }
}

export function ResourceAccessPanel({ kindGroup }) {
  if (!kindGroup) return null

  const forbidden = kindGroup.accessState === 'forbidden'
    || kindGroup.countState?.state === 'forbidden'
  const unavailable = (!kindGroup.discovered && kindGroup.builtin && !kindGroup.discoveredOnly)
    || kindGroup.accessState === 'unavailable'
    || kindGroup.countState?.state === 'unavailable'

  if (!forbidden && !unavailable) return null

  const label = kindGroup.label || kindGroup.kind
  const meta = accessMeta(kindGroup)

  if (forbidden) {
    return (
      <div className="resource-access-panel">
        <div className="resource-access-icon" aria-hidden="true">🔒</div>
        <p className="resource-access-kicker">Access required</p>
        <h4 className="resource-access-title">{label}</h4>
        <p className="resource-access-body">
          Klew discovered this Kubernetes resource, but the current identity cannot list it.
        </p>
        <dl className="resource-access-meta">
          {meta.api && <><dt>API</dt><dd className="mono">{meta.api}</dd></>}
          {meta.resource && <><dt>Resource</dt><dd className="mono">{meta.resource}</dd></>}
          <dt>Scope</dt><dd>{meta.scope}</dd>
          <dt>Required capability</dt>
          <dd className="mono">list {meta.resource || label.toLowerCase()}</dd>
        </dl>
      </div>
    )
  }

  return (
    <div className="resource-access-panel">
      <p className="resource-access-kicker">Not available</p>
      <h4 className="resource-access-title">{label}</h4>
      <p className="resource-access-body">
        This API is not exposed by the connected cluster.
      </p>
      <dl className="resource-access-meta">
        <dt>API</dt><dd className="mono">{meta.api || '—'}</dd>
        <dt>Resource</dt><dd className="mono">{meta.resource || '—'}</dd>
      </dl>
    </div>
  )
}
