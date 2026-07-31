import { StatusBadge } from './StatusBadge'
import { focusMetrics } from '../../lib/focusScope'

export function StatusStrip({ scope, summary, rows, drillDown, focusRow, view }) {
  const s = summary || {}
  const totalPods = (s.readyPods ?? 0) + (s.unreadyPods ?? 0)
  const { status: scopeStatus, label: scopeLabel, row: worst } = scope
  const n = scope.matchCount ?? (rows || []).length
  const drilled = Boolean(drillDown?.active && focusRow)

  const status = drilled ? (focusRow.status || scopeStatus) : scopeStatus
  const label = drilled
    ? (focusRow.status === 'healthy' ? 'HEALTHY' : focusRow.status === 'critical' ? 'CRITICAL' : 'DEGRADED')
    : scopeLabel
  const healthy = status === 'healthy'

  const headline = drilled
    ? buildDrillHeadline(focusRow, drillDown)
    : buildScopeHeadline({
      status: scopeStatus,
      worst,
      summary: s,
      n,
      unhealthyCount: scope.unhealthyCount ?? 0,
    })

  const metrics = drilled ? focusMetrics(view, drillDown) : null
  const showMetrics = !healthy || (drilled && focusRow.status !== 'healthy')

  const stripClass = status === 'healthy'
    ? 'incident-strip incident-strip-healthy'
    : status === 'critical'
      ? 'incident-strip incident-strip-critical'
      : 'incident-strip incident-strip-degraded'

  return (
    <div className={stripClass}>
      <div className="strip-left">
        <StatusBadge status={status} label={label} />
        <span className="strip-headline">{headline}</span>
      </div>
      {showMetrics && !drilled && (
        <div className="strip-metrics">
          <span>Ready {s.readyPods ?? 0}/{totalPods || '—'}</span>
          <span className="strip-dot">·</span>
          <span>Restarts {s.restarts ?? 0}</span>
          <span className="strip-dot">·</span>
          <span>Endpoints {s.endpointsReady ?? '—'}/{s.endpointsTotal || '—'}</span>
        </div>
      )}
      {showMetrics && drilled && (
        <div className="strip-metrics">
          <span>Ready {metrics.ready ?? '—'}/{metrics.total || '—'}</span>
          <span className="strip-dot">·</span>
          <span>Restarts {metrics.restarts ?? 0}</span>
          {metrics.endpointsTotal != null && (
            <>
              <span className="strip-dot">·</span>
              <span>Endpoints {metrics.endpointsReady ?? '—'}/{metrics.endpointsTotal || '—'}</span>
            </>
          )}
        </div>
      )}
    </div>
  )
}

function buildDrillHeadline(focusRow, drillDown) {
  const kind = focusRow.kind || 'Workload'
  const name = focusRow.name || '—'
  const pods = drillDown.relatedPodCount || 0
  if (focusRow.status === 'healthy') {
    const podBit = pods ? ` · ${pods} related pod${pods === 1 ? '' : 's'}` : ''
    return `Focused: ${kind}/${name} — ready${podBit}`
  }
  const signal = focusRow.signal || 'investigating'
  return `Focused: ${kind}/${name} — ${signal}`
}

function buildScopeHeadline({ status, worst, summary, n, unhealthyCount }) {
  if (!n) {
    return status === 'healthy' ? 'No matches in scope' : 'Investigating…'
  }

  if (status === 'healthy') {
    return n === 1 ? 'Ready — no anomalies' : `All ${n} matches ready — no anomalies`
  }

  const kind = worst?.kind || 'Workload'
  const name = worst?.name || summary.query || '—'
  const signal = worst?.signal || summary.leadingSignal || 'investigating'
  const worstLine = `${kind}/${name} — ${signal}`
  if (unhealthyCount > 1) {
    return `${worstLine} · ${unhealthyCount} of ${n} unhealthy`
  }
  return worstLine
}
