import { useMemo } from 'react'
import { KindIcon } from '../components/KindIcon'
import {
  fmtCpu,
  fmtMem,
  getSnapshot,
  nodePressureFlags,
} from '../lib/investigationViews'
import { formatNodesVersionLead } from '../lib/clusterVersion'

/**
 * Nodes — cluster inventory and investigation-scoped node context.
 */
export function NodesView({ view, clusterStatus, focus = 'cluster' }) {
  const snap = getSnapshot(view)
  const scopedNodes = snap.nodes || []
  const pods = snap.pods || []
  const clusterItems = clusterStatus?.nodeItems || []

  const podsByNode = useMemo(() => {
    const map = new Map()
    for (const p of pods) {
      if (!p.node) continue
      if (!map.has(p.node)) map.set(p.node, [])
      map.get(p.node).push(p)
    }
    return map
  }, [pods])

  const scopedNames = useMemo(
    () => new Set(scopedNodes.map((n) => n.name)),
    [scopedNodes],
  )

  const showCluster = focus !== 'scope'
  const showScope = focus !== 'cluster'

  const clusterRows = useMemo(() => {
    return [...clusterItems].sort((a, b) => {
      const aBad = !a.ready || a.memoryPressure || a.diskPressure || a.pidPressure
      const bBad = !b.ready || b.memoryPressure || b.diskPressure || b.pidPressure
      if (aBad !== bBad) return aBad ? -1 : 1
      return String(a.name).localeCompare(String(b.name))
    })
  }, [clusterItems])

  const scopedRows = useMemo(() => {
    const list = scopedNodes.length
      ? scopedNodes
      : clusterItems.filter((n) => scopedNames.has(n.name) || podsByNode.has(n.name))
    return [...list].sort((a, b) => String(a.name).localeCompare(String(b.name)))
  }, [scopedNodes, clusterItems, scopedNames, podsByNode])

  return (
    <div className="inv-page nodes-page">
      {showScope && (
        <section className="card inv-card">
          <header className="nodes-section-head">
            <h3>Nodes in investigation scope</h3>
            <p className="muted nodes-section-lead">
              Nodes hosting workloads matched by the current investigation query.
            </p>
          </header>
          <div className="card-body card-body-flush">
            {scopedRows.length === 0 ? (
              <p className="muted inv-pad">
                No scoped node data yet. Start or widen an investigation to see nodes
                hosting matched pods.
              </p>
            ) : (
              <NodeTable
                rows={scopedRows}
                podsByNode={podsByNode}
                detailed
                inScope
              />
            )}
          </div>
        </section>
      )}

      {showCluster && (
        <section className="card inv-card">
          <header className="nodes-section-head">
            <h3>Cluster node inventory</h3>
            <p className="muted nodes-section-lead">
              {formatNodesVersionLead(clusterStatus) || 'Cluster node inventory'}
            </p>
          </header>
          <div className="card-body card-body-flush">
            {clusterRows.length === 0 ? (
              <p className="muted inv-pad">
                {clusterStatus?.error
                  ? clusterStatus.error
                  : 'Cluster node inventory is not available yet.'}
              </p>
            ) : (
              <NodeTable
                rows={clusterRows}
                podsByNode={podsByNode}
                scopedNames={scopedNames}
              />
            )}
          </div>
        </section>
      )}
    </div>
  )
}

function NodeTable({ rows, podsByNode, scopedNames, detailed = false, inScope = false }) {
  return (
    <table className="nodes-table">
      <thead>
        <tr>
          <th>Node</th>
          <th>Role</th>
          <th>Kubelet</th>
          <th>Status</th>
          <th>Signals</th>
          {detailed && <th>Capacity</th>}
          <th>Scoped pods</th>
          {!inScope && <th>Scope</th>}
        </tr>
      </thead>
      <tbody>
        {rows.map((n) => {
          const flags = pressureFlagsForRow(n)
          const nodePods = podsByNode.get(n.name) || []
          const scoped = inScope || scopedNames?.has(n.name) || nodePods.length > 0
          return (
            <tr key={n.name} className={flags.length ? 'is-pressured' : ''}>
              <td>
                <span className="nodes-name-cell">
                  <KindIcon kind="Node" />
                  <span className="mono">{n.name}</span>
                </span>
              </td>
              <td className="muted">{formatNodeRole(n.role)}</td>
              <td className="mono nodes-kubelet-cell">{n.kubeletVersion || '—'}</td>
              <td>
                <span className={n.ready ? 'tone-ok' : 'tone-crit'}>
                  {n.ready ? 'Ready' : 'Not ready'}
                </span>
              </td>
              <td>
                {flags.length === 0 ? (
                  <span className="muted">—</span>
                ) : (
                  <div className="node-flags">
                    {flags.map((f) => (
                      <span key={f} className="node-flag">{f}</span>
                    ))}
                  </div>
                )}
              </td>
              {detailed && (
                <td className="muted mono nodes-cap-cell">
                  {n.allocatableCpuMillicores || n.allocatableMemMi
                    ? `${fmtCpu(n.allocatableCpuMillicores)} · ${fmtMem(n.allocatableMemMi)}`
                    : n.kubeletVersion || '—'}
                </td>
              )}
              <td className="mono">
                {nodePods.length > 0 ? nodePods.length : '—'}
              </td>
              {!inScope && (
                <td>
                  {scoped ? (
                    <span className="nodes-scope-badge">In scope</span>
                  ) : (
                    <span className="muted">—</span>
                  )}
                </td>
              )}
            </tr>
          )
        })}
      </tbody>
    </table>
  )
}

function formatNodeRole(role) {
  if (role === 'control-plane') return 'Control plane'
  if (role === 'worker') return 'Worker'
  return 'Unknown'
}

function pressureFlagsForRow(node) {
  if (node.allocatableCpuMillicores != null || node.allocatableMemMi != null) {
    return nodePressureFlags(node)
  }
  const flags = []
  if (!node.ready) flags.push('NotReady')
  if (node.memoryPressure) flags.push('MemoryPressure')
  if (node.diskPressure) flags.push('DiskPressure')
  if (node.pidPressure) flags.push('PIDPressure')
  if (node.unschedulable) flags.push('Unschedulable')
  return flags
}
