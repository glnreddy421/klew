import { KindIcon } from '../KindIcon'

export function NodeNameCell({ row }) {
  const roles = row.nodeResources?.roles || row.table?.roles || ''
  const isControlPlane = roles.includes('control-plane') || roles.includes('master')
  return (
    <span className="entity-table-node-name">
      <KindIcon kind="Node" size={16} className={isControlPlane ? 'entity-table-node-icon-cp' : ''} />
      <span className="entity-table-name" title={row.name}>{row.name}</span>
    </span>
  )
}

export function NodeResourceCell({ capacity, allocatable, used }) {
  const cap = Number(capacity) || 0
  const use = used == null ? null : Number(used)
  const pct = cap > 0 && use != null ? Math.min(100, Math.round((use / cap) * 100)) : null

  return (
    <div className="entity-table-node-resource" title={formatResourceTitle(cap, allocatable, use)}>
      <div className="entity-table-node-resource-track" aria-hidden="true">
        {pct != null && pct > 0 ? (
          <div className="entity-table-node-resource-fill" style={{ width: `${pct}%` }} />
        ) : null}
      </div>
      {pct == null ? <span className="entity-table-node-resource-empty">—</span> : null}
    </div>
  )
}

function formatResourceTitle(capacity, allocatable, used) {
  const parts = []
  if (used != null) parts.push(`used ${used}`)
  if (allocatable != null) parts.push(`allocatable ${allocatable}`)
  if (capacity != null) parts.push(`capacity ${capacity}`)
  return parts.join(' · ') || 'No metrics'
}

export function NodeConditionsCell({ row }) {
  const ready = row.nodeResources?.ready ?? row.table?.nodeReady
  if (ready == null) {
    const label = row.table?.conditions || row.table?.statusLabel || '—'
    return <span className="entity-table-node-condition">{label}</span>
  }
  const label = ready ? 'Ready' : 'NotReady'
  const tone = ready ? 'healthy' : 'critical'
  return (
    <span className={`entity-table-node-condition status-${tone === 'healthy' ? 'healthy' : 'warning'}`}>
      <span className="entity-table-status-led" aria-hidden="true" />
      {label}
    </span>
  )
}

export function NodeTaintsCell({ row }) {
  const count = row.nodeResources?.taintCount ?? row.table?.taints
  if (count == null || count === '') return <span>—</span>
  return <span className="mono">{count}</span>
}
