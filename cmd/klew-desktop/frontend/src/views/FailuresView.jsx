import { useMemo, useState } from 'react'
import { StatusBadge } from '../components/incident/StatusBadge'
import {
  containerHealthLabel,
  formatClock,
  formatClockDate,
  getSnapshot,
  getState,
  normalizeHealth,
  podHealthLabel,
  rankPodsForTriage,
  severityRankTL,
  worstContainer,
} from '../lib/investigationViews'

/**
 * Failures — severity-ranked pod triage + investigation detail.
 */
export function FailuresView({ view, explorerFilter }) {
  const state = getState(view)
  const snap = getSnapshot(view)
  const pods = useMemo(() => {
    let list = rankPodsForTriage(snap.pods || [])
    const f = explorerFilter || {}
    if (f.severity === 'critical') {
      list = list.filter((p) => podHealthLabel(p) === 'critical')
    } else if (f.severity === 'warning') {
      list = list.filter((p) => {
        const h = podHealthLabel(p)
        return h === 'warning' || h === 'degraded'
      })
    } else if (f.severity === 'stable') {
      list = list.filter((p) => podHealthLabel(p) === 'healthy')
    }
    if (f.type) {
      list = list.filter((p) => {
        const c = worstContainer(p)
        const r = String(c?.lastReason || c?.reason || '').toLowerCase()
        switch (f.type) {
          case 'oom': return r.includes('oom')
          case 'probe': return r.includes('probe') || r.includes('unhealthy')
          case 'image': return r.includes('image')
          case 'crash': return r.includes('crash') || r.includes('backoff')
          default: return true
        }
      })
    }
    return list
  }, [snap.pods, explorerFilter])
  const [selected, setSelected] = useState(null)

  const activeKey = selected && pods.some((p) => p.name === selected)
    ? selected
    : (pods[0]?.name || null)
  const active = pods.find((p) => p.name === activeKey) || null

  const failing = pods.filter((p) => podHealthLabel(p) !== 'healthy').length
  const signal = state.verdict?.leadingSignal
    || dominantReason(pods)
    || 'None'
  const signalDesc = failureSignalDescription(signal)

  if (!pods.length) {
    return (
      <div className="inv-page">
        <div className="inv-empty muted">No pods in investigation scope.</div>
      </div>
    )
  }

  const podEvents = (active ? relatedEvents(view, active.name) : []).slice(0, 12)

  return (
    <div className="inv-page failures-page">
      <section className="card inv-card">
        <h3>Failures</h3>
        <div className="card-body failures-summary">
          <div className="failures-signal">
            <StatusBadge
              status={failing === 0 ? 'healthy' : (signal.toLowerCase().includes('oom') ? 'critical' : 'warning')}
              label={signal}
            />
            <span className="muted">{signalDesc}</span>
          </div>
          <span className={failing ? 'tone-warn' : 'tone-ok'}>
            {failing === 0 ? 'All pods stable' : `${failing} of ${pods.length} pods failing`}
          </span>
        </div>
      </section>

      <div className="failures-split">
        <section className="card inv-card">
          <h3>Pod triage</h3>
          <div className="card-body card-body-flush">
            <ul className="pod-triage-list" role="listbox" aria-label="Failing pods">
              {pods.map((p) => {
                const health = podHealthLabel(p)
                const worst = worstContainer(p)
                const activeRow = p.name === activeKey
                return (
                  <li key={p.name}>
                    <button
                      type="button"
                      role="option"
                      aria-selected={activeRow}
                      className={`pod-triage-row ${activeRow ? 'selected' : ''} health-${health}`}
                      onClick={() => setSelected(p.name)}
                    >
                      <span className={`match-led status-${health === 'warning' ? 'warning' : health}`} />
                      <span className="pod-triage-main">
                        <span className="mono pod-name" title={p.name}>{p.name}</span>
                        <span className="muted">
                          {worst?.lastReason || worst?.reason || p.phase || '—'}
                          {p.node ? ` · ${p.node}` : ''}
                        </span>
                      </span>
                      <span className="pod-triage-meta">
                        <span>{p.ready ? 'Ready' : 'Not ready'}</span>
                        <span className="mono">{p.restartCount || 0}↻</span>
                      </span>
                    </button>
                  </li>
                )
              })}
            </ul>
          </div>
        </section>

        <section className="card inv-card">
          <h3>Pod investigation</h3>
          <div className="card-body">
            {!active ? (
              <p className="muted">Select a pod to inspect runtime detail.</p>
            ) : (
              <PodDetail pod={active} events={podEvents} />
            )}
          </div>
        </section>
      </div>
    </div>
  )
}

function PodDetail({ pod, events }) {
  const health = podHealthLabel(pod)
  const containers = pod.containers || []

  return (
    <div className="pod-detail">
      <div className="pod-detail-head">
        <div>
          <div className="inspect-category">Pod</div>
          <h4 className="inspect-name mono">{pod.name}</h4>
        </div>
        <StatusBadge status={normalizeHealth(health)} />
      </div>

      <dl className="inspect-fields">
        <div className="inspect-field"><dt>Phase</dt><dd>{pod.phase || '—'}</dd></div>
        <div className="inspect-field"><dt>Ready</dt><dd>{pod.ready ? 'yes' : 'no'}</dd></div>
        <div className="inspect-field"><dt>Restarts</dt><dd>{pod.restartCount || 0}</dd></div>
        <div className="inspect-field"><dt>Node</dt><dd className="mono">{pod.node || '—'}</dd></div>
        <div className="inspect-field"><dt>Created</dt><dd>{formatClockDate(pod.createdAt)}</dd></div>
      </dl>

      <h5 className="inv-section-title">Containers</h5>
      {containers.length === 0 ? (
        <p className="muted">No container status.</p>
      ) : (
        <ul className="container-list">
          {containers.map((c) => (
            <li key={c.name} className={`container-row health-${containerHealthLabel(c)}`}>
              <div className="container-head">
                <strong className="mono">{c.name}</strong>
                <StatusBadge status={normalizeHealth(containerHealthLabel(c))} label={c.state || containerHealthLabel(c)} />
              </div>
              <div className="container-meta muted">
                {[
                  c.reason || c.lastReason,
                  c.restartCount != null ? `${c.restartCount} restarts` : null,
                  c.lastExitCode != null ? `exit ${c.lastExitCode}` : null,
                  c.image ? truncate(c.image, 48) : null,
                ].filter(Boolean).join(' · ')}
              </div>
              {(c.requestsMem || c.limitsMem || c.requestsCPU || c.limitsCPU) && (
                <div className="container-res muted">
                  CPU {c.requestsCPU || '—'}/{c.limitsCPU || '—'} · Mem {c.requestsMem || '—'}/{c.limitsMem || '—'}
                </div>
              )}
            </li>
          ))}
        </ul>
      )}

      <h5 className="inv-section-title">Related events</h5>
      {events.length === 0 ? (
        <p className="muted">No recent events for this pod.</p>
      ) : (
        <ul className="inspect-events">
          {events.map((ev, i) => (
            <li key={`${ev.time}-${i}`} className={`inspect-event sev-${ev.severity}`}>
              <span className="inspect-event-time mono">{ev.time}</span>
              <span className="inspect-event-reason">{ev.reason}</span>
              <span className="inspect-event-msg" title={ev.message}>{ev.message}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

function dominantReason(pods) {
  const counts = {}
  for (const p of pods || []) {
    const c = worstContainer(p)
    const r = c?.lastReason || c?.reason
    if (!r) continue
    counts[r] = (counts[r] || 0) + 1
  }
  return Object.entries(counts).sort((a, b) => b[1] - a[1])[0]?.[0] || ''
}

function failureSignalDescription(signal) {
  switch (String(signal || '').toLowerCase()) {
    case 'oomkilled':
    case 'oomkilling':
      return 'container exceeded memory limit'
    case 'crashloopbackoff':
    case 'backoff':
      return 'container repeatedly crashing'
    case 'errimagepull':
    case 'imagepullbackoff':
      return 'image pull failure'
    case 'failedmount':
      return 'volume mount failure'
    case 'none':
    case '':
      return 'no dominant failure signal'
    default:
      return 'leading runtime signal'
  }
}

function relatedEvents(view, podName) {
  const out = []
  const snap = getSnapshot(view)
  for (const e of snap.events || []) {
    const obj = e.involvedObject || {}
    const name = obj.name || e.pod || ''
    if (name !== podName && !String(name).startsWith(`${podName}-`)) continue
    out.push({
      time: formatClock(e.timestamp),
      reason: e.reason || e.type || '',
      message: e.message || '',
      severity: severityRankTL(e.type === 'Warning' ? 'warning' : 'info') >= 2 ? 'warning' : 'info',
    })
  }
  for (const e of view?.evidence || []) {
    if (e.sourceType === 'log') continue
    if (e.pod !== podName && e.sourceName !== podName) continue
    out.push({
      time: formatClock(e.timestamp),
      reason: e.reason || e.sourceType || '',
      message: e.message || '',
      severity: e.severity || 'info',
    })
  }
  return out
}

function truncate(s, n) {
  const t = String(s || '')
  return t.length > n ? `${t.slice(0, n - 1)}…` : t
}
