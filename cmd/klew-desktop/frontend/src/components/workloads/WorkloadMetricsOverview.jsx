import { Component, useMemo, useState } from 'react'
import { KindIcon } from '../KindIcon'
import { buildWorkloadMetrics } from '../../lib/workloadMetrics.js'

const COLLAPSE_KEY = 'klew.workloadMetrics.collapsed'

function loadCollapsed() {
  try {
    return localStorage.getItem(COLLAPSE_KEY) === '1'
  } catch {
    return false
  }
}

function saveCollapsed(value) {
  try {
    localStorage.setItem(COLLAPSE_KEY, value ? '1' : '0')
  } catch {
    // ignore
  }
}

class WorkloadMetricsBoundary extends Component {
  constructor(props) {
    super(props)
    this.state = { error: null }
  }

  static getDerivedStateFromError(error) {
    return { error }
  }

  render() {
    if (this.state.error) return null
    return this.props.children
  }
}

function MetricBar({ label, value, pct, tone = 'ok', sublabel }) {
  const safePct = pct == null ? 0 : Math.min(100, Math.max(0, pct))
  return (
    <div className={`wmo-metric wmo-metric-${tone}`}>
      <div className="wmo-metric-head">
        <span className="wmo-metric-label">{label}</span>
        <span className="wmo-metric-value mono">{value}</span>
      </div>
      <div className="wmo-metric-track" aria-hidden="true">
        <div className="wmo-metric-fill" style={{ width: `${safePct}%` }} />
      </div>
      {sublabel && <span className="wmo-metric-sub muted">{sublabel}</span>}
    </div>
  )
}

function HealthRing({ pct, tone, label }) {
  const value = pct == null ? 0 : Math.min(100, Math.max(0, pct))
  return (
    <div className={`wmo-health-ring tone-${tone}`} style={{ '--pct': value }}>
      <div className="wmo-health-ring-inner">
        <span className="wmo-health-ring-value mono">{pct == null ? '—' : `${value}%`}</span>
        <span className="wmo-health-ring-label">{label}</span>
      </div>
    </div>
  )
}

function WorkloadMetricsOverviewBody({
  view,
  clusterStatus,
  tree,
  onSelectKind,
  standalone = false,
}) {
  const [collapsed, setCollapsed] = useState(() => (standalone ? false : loadCollapsed()))

  const metrics = useMemo(() => {
    try {
      return buildWorkloadMetrics({ tree, view, clusterStatus })
    } catch (err) {
      console.error('WorkloadMetricsOverview failed:', err)
      return null
    }
  }, [tree, view, clusterStatus])

  if (!metrics?.kindTiles?.length) {
    return (
      <div className="workloads-page-empty muted">
        No workload data in the current browse scope yet.
      </div>
    )
  }

  const toggleCollapsed = () => {
    setCollapsed((prev) => {
      const next = !prev
      saveCollapsed(next)
      return next
    })
  }

  const handleSelectKind = (tile) => {
    if (!tile?.kind) return
    onSelectKind?.(tile)
  }

  const podValue = metrics.hasInvestigation
    ? `${metrics.pods.healthy}/${metrics.pods.total} ready`
    : `${metrics.pods.total || metrics.kindTiles.find((t) => t.kind === 'Pod')?.count || 0} pods`

  const replicaValue = metrics.replicas.desired > 0
    ? `${metrics.replicas.ready}/${metrics.replicas.desired} ready`
    : '—'

  const nodesReady = metrics.nodes?.ready ?? 0
  const nodesTotal = metrics.nodes?.total ?? 0
  const nodesNotReady = metrics.nodes?.notReady ?? 0
  const isCollapsed = !standalone && collapsed

  return (
    <section
      className={`workload-metrics-overview ${standalone ? 'is-standalone' : ''} ${isCollapsed ? 'is-collapsed' : ''}`}
      aria-label="Workload metrics"
    >
      <div className="wmo-header">
        <div className="wmo-header-main">
          <HealthRing
            pct={metrics.podHealthPct ?? metrics.replicaPct}
            tone={metrics.healthTone}
            label="Health"
          />
          <div className="wmo-header-copy">
            <h2 className="wmo-title">Workloads</h2>
            <p className={`wmo-health-label tone-${metrics.healthTone}`}>{metrics.healthLabel}</p>
            <p className="wmo-summary muted">
              {metrics.totalWorkloadCount} resource{metrics.totalWorkloadCount === 1 ? '' : 's'} in scope
            </p>
          </div>
        </div>
        {!standalone && (
          <button
            type="button"
            className="wmo-collapse-btn"
            onClick={toggleCollapsed}
            aria-expanded={!isCollapsed}
          >
            {isCollapsed ? 'Show metrics' : 'Hide metrics'}
          </button>
        )}
      </div>

      {!isCollapsed && (
        <>
          <div className="wmo-kind-grid">
            {metrics.kindTiles.map((tile) => (
              <button
                key={tile.kind}
                type="button"
                className={`wmo-kind-tile ${tile.denied ? 'is-denied' : ''}`}
                onClick={() => handleSelectKind(tile)}
                title={onSelectKind ? `Open ${tile.label} in Resources` : tile.label}
              >
                <KindIcon kind={tile.kind} size={26} />
                <span className="wmo-kind-count mono">{tile.denied ? '—' : tile.count}</span>
                <span className="wmo-kind-label">{tile.label}</span>
              </button>
            ))}
          </div>

          <div className="wmo-metrics-grid">
            <MetricBar
              label="Pod health"
              value={podValue}
              pct={metrics.podHealthPct ?? (metrics.pods.total > 0 ? 0 : null)}
              tone={metrics.pods.failing > 0 ? 'crit' : metrics.pods.pending > 0 ? 'warn' : 'ok'}
              sublabel={
                metrics.hasInvestigation && metrics.pods.failing > 0
                  ? `${metrics.pods.failing} failing · ${metrics.pods.pending} pending`
                  : metrics.hasInvestigation && metrics.pods.pending > 0
                    ? `${metrics.pods.pending} pending`
                    : undefined
              }
            />
            <MetricBar
              label="Replica readiness"
              value={replicaValue}
              pct={metrics.replicaPct}
              tone={
                metrics.replicaPct == null ? 'muted'
                  : metrics.replicaPct >= 100 ? 'ok'
                    : metrics.replicaPct >= 50 ? 'warn' : 'crit'
              }
            />
            {metrics.resources.available ? (
              <>
                <MetricBar
                  label="CPU usage"
                  value={`${metrics.resources.cpu.label} / ${metrics.resources.cpu.denomLabel}`}
                  pct={metrics.resources.cpu.pct}
                  tone={(metrics.resources.cpu.pct ?? 0) >= 80 ? 'warn' : 'ok'}
                />
                <MetricBar
                  label="Memory usage"
                  value={`${metrics.resources.memory.label} / ${metrics.resources.memory.denomLabel}`}
                  pct={metrics.resources.memory.pct}
                  tone={(metrics.resources.memory.pct ?? 0) >= 80 ? 'warn' : 'ok'}
                />
              </>
            ) : (
              <MetricBar
                label="Resource usage"
                value="Metrics unavailable"
                pct={0}
                tone="muted"
                sublabel={metrics.resources.note || 'Install metrics-server for live usage'}
              />
            )}
            <div className="wmo-stat-cards">
              <div className={`wmo-stat-card tone-${nodesNotReady > 0 ? 'warn' : 'ok'}`}>
                <span className="wmo-stat-value mono">{nodesReady}/{nodesTotal || '—'}</span>
                <span className="wmo-stat-label">Nodes ready</span>
              </div>
              {(metrics.blast?.critical ?? 0) + (metrics.blast?.warning ?? 0) > 0 && (
                <div className="wmo-stat-card tone-warn">
                  <span className="wmo-stat-value mono">
                    {(metrics.blast?.critical ?? 0) + (metrics.blast?.warning ?? 0)}
                  </span>
                  <span className="wmo-stat-label">In-scope issues</span>
                </div>
              )}
            </div>
          </div>
        </>
      )}
    </section>
  )
}

export function WorkloadMetricsOverview({
  view,
  clusterStatus,
  tree,
  onSelectKind,
  standalone = false,
}) {
  return (
    <WorkloadMetricsBoundary>
      <WorkloadMetricsOverviewBody
        view={view}
        clusterStatus={clusterStatus}
        tree={tree}
        onSelectKind={onSelectKind}
        standalone={standalone}
      />
    </WorkloadMetricsBoundary>
  )
}
