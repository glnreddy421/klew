import { useMemo } from 'react'
import { KindIcon } from '../components/KindIcon'
import {
  fmtCpu,
  fmtMem,
  getSnapshot,
  nodePressureFlags,
  podHealthLabel,
  resourceFindings,
  utilPct,
} from '../lib/investigationViews'

/**
 * Resources — CPU/memory capacity, nodes, investigation pods, findings.
 */
export function ResourcesView({ view }) {
  const snap = getSnapshot(view)
  const m = snap.metrics || {}
  const pods = snap.pods || []
  const nodes = snap.nodes || []
  const findings = useMemo(() => resourceFindings(view), [view])

  const memDenom = Math.max(m.memLimitMi || 0, m.memRequestMi || 0, 1)
  const cpuDenom = Math.max(m.cpuLimitMillicores || 0, m.cpuRequestMillicores || 0, 1)
  const memPct = utilPct(m.memUsageMi, memDenom)
  const cpuPct = utilPct(m.cpuUsageMillicores, cpuDenom)

  const invNodes = useMemo(() => {
    const names = new Set(pods.map((p) => p.node).filter(Boolean))
    return nodes.filter((n) => names.has(n.name))
  }, [pods, nodes])

  const colocated = useMemo(() => {
    const invNames = new Set(pods.map((p) => p.name))
    const nodeSet = new Set(pods.map((p) => p.node).filter(Boolean))
    // Co-located = pods on same nodes that aren't in investigation list
    // Snapshot may only include investigation pods; still show by node grouping.
    return pods.filter((p) => nodeSet.has(p.node) && !invNames.has(`__none__`))
  }, [pods])

  return (
    <div className="inv-page resources-page">
      <div className="resources-top">
        <section className="card inv-card">
          <h3>Workload capacity</h3>
          <div className="card-body">
            <CapacityBlock
              title="CPU"
              available={m.available}
              usage={m.cpuUsageMillicores}
              request={m.cpuRequestMillicores}
              limit={m.cpuLimitMillicores}
              pct={cpuPct}
              fmt={fmtCpu}
              unit="cores / millicores"
            />
            <CapacityBlock
              title="Memory"
              available={m.available}
              usage={m.memUsageMi}
              request={m.memRequestMi}
              limit={m.memLimitMi}
              pct={memPct}
              fmt={fmtMem}
              unit="Mi"
              warnHigh
            />
            {!m.available && (
              <p className="muted resources-note">
                {m.note || 'Usage unavailable (metrics-server). Showing requests/limits from pod specs.'}
              </p>
            )}
          </div>
        </section>

        <section className="card inv-card">
          <h3>Node footprint</h3>
          <div className="card-body">
            {(invNodes.length ? invNodes : nodes).length === 0 ? (
              <p className="muted">No node data in snapshot.</p>
            ) : (
              <ul className="node-list">
                {(invNodes.length ? invNodes : nodes).slice(0, 8).map((n) => {
                  const flags = nodePressureFlags(n)
                  return (
                    <li key={n.name} className={`node-row ${flags.length ? 'pressured' : ''}`}>
                      <div className="node-head">
                        <span className="mono">{n.name}</span>
                        <span className={n.ready ? 'tone-ok' : 'tone-crit'}>
                          {n.ready ? 'Ready' : 'NotReady'}
                        </span>
                      </div>
                      {flags.length > 0 && (
                        <div className="node-flags">
                          {flags.map((f) => (
                            <span key={f} className="node-flag">{f}</span>
                          ))}
                        </div>
                      )}
                      <div className="muted node-cap">
                        Alloc CPU {fmtCpu(n.allocatableCpuMillicores)} · Mem {fmtMem(n.allocatableMemMi)}
                      </div>
                    </li>
                  )
                })}
              </ul>
            )}
          </div>
        </section>
      </div>

      <div className="resources-mid">
        <section className="card inv-card">
          <h3>Investigation pods</h3>
          <div className="card-body card-body-flush">
            {pods.length === 0 ? (
              <p className="muted inv-pad">No pods in scope.</p>
            ) : (
              <ul className="res-pod-list">
                {pods.map((p) => (
                  <li key={p.name} className={`res-pod-row health-${podHealthLabel(p)}`}>
                    <span className={`match-led status-${podHealthLabel(p) === 'warning' ? 'warning' : podHealthLabel(p)}`} />
                    <span className="mono res-pod-name" title={p.name}>{p.name}</span>
                    <span className="muted">{p.node || '—'}</span>
                    <span className="mono">{podResLine(p)}</span>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </section>

        <section className="card inv-card">
          <h3>Pods on shared nodes</h3>
          <div className="card-body card-body-flush">
            {colocated.length === 0 ? (
              <p className="muted inv-pad">No co-located pods listed in snapshot.</p>
            ) : (
              <ul className="res-pod-list">
                {colocated.slice(0, 24).map((p) => (
                  <li key={`co-${p.name}`} className="res-pod-row">
                    <KindIcon kind="Pod" />
                    <span className="mono res-pod-name" title={p.name}>{p.name}</span>
                    <span className="muted">{p.node || '—'}</span>
                    <span className="mono">{p.restartCount || 0}↻</span>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </section>
      </div>

      <section className="card inv-card">
        <h3>Resource investigation</h3>
        <div className="card-body">
          <ul className="resource-findings">
            {findings.map((f, i) => (
              <li key={i} className={`finding level-${f.level}`}>
                <span className="finding-mark" aria-hidden="true">
                  {f.level === 'crit' ? '!' : f.level === 'warn' ? '▲' : f.level === 'ok' ? '✓' : '·'}
                </span>
                <span>{f.text}</span>
              </li>
            ))}
          </ul>
        </div>
      </section>
    </div>
  )
}

function CapacityBlock({ title, available, usage, request, limit, pct, fmt, warnHigh }) {
  const denom = Math.max(limit || 0, request || 0, usage || 0, 1)
  const hasSpec = (request || 0) > 0 || (limit || 0) > 0
  if (!hasSpec && !(available && usage)) {
    return (
      <div className="capacity-block">
        <h5 className="inv-section-title">{title}</h5>
        <p className="muted">No {title.toLowerCase()} data</p>
      </div>
    )
  }

  const reqPct = Math.min(100, Math.round(((request || 0) / denom) * 100))
  const limPct = limit ? Math.min(100, Math.round((limit / denom) * 100)) : 0
  const usePct = usage != null && available ? Math.min(100, Math.round((usage / denom) * 100)) : null

  return (
    <div className="capacity-block">
      <h5 className="inv-section-title">{title}</h5>
      {available && usage != null && usage > 0 ? (
        <p className="capacity-numbers">
          <strong>{fmt(usage)}</strong>
          <span className="muted"> / {fmt(denom)}</span>
          {pct != null && (
            <span className={`capacity-pct ${warnHigh && pct >= 80 ? 'high' : ''}`}>
              {pct}%{warnHigh && pct >= 80 ? ' ⚠' : ''}
            </span>
          )}
        </p>
      ) : (
        <p className="muted capacity-numbers">
          request {fmt(request)} · limit {fmt(limit)}
        </p>
      )}
      <div className="resource-track capacity-track" title={`${title} usage`}>
        {limPct > 0 && <div className="resource-fill limit" style={{ width: `${limPct}%` }} />}
        <div className="resource-fill request" style={{ width: `${reqPct}%` }} />
        {usePct != null && <div className="resource-fill usage" style={{ width: `${usePct}%` }} />}
      </div>
      <div className="resource-legend">
        <span><i className="swatch request" /> request</span>
        {limit > 0 && <span><i className="swatch limit" /> limit</span>}
        {usePct != null && <span><i className="swatch usage" /> usage</span>}
      </div>
    </div>
  )
}

function podResLine(p) {
  const c = (p.containers || [])[0]
  if (!c) return '—'
  const bits = []
  if (c.requestsCPU || c.limitsCPU) bits.push(`cpu ${c.requestsCPU || '—'}/${c.limitsCPU || '—'}`)
  if (c.requestsMem || c.limitsMem) bits.push(`mem ${c.requestsMem || '—'}/${c.limitsMem || '—'}`)
  return bits.join(' · ') || `${p.restartCount || 0}↻`
}
