import { useMemo } from 'react'
import { KindIcon } from '../KindIcon'
import { InlineLoading, LoadingState } from '../LoadingSpinner.jsx'
import { useWorkloadOverview } from '../../hooks/useWorkloadOverview.js'
import { summarizeBrowsePodMetrics } from '../../lib/browseMetrics.js'
import { buildOverviewMetricsSummary } from '../../lib/metricDisplay.js'
import { OverviewMetricCard, OverviewNodesCard } from '../metrics/ResourceMetricDisplay.jsx'
import { donutGradient } from '../../lib/workloadOverview.js'

function dominantTone(segments = []) {
  if (segments.some((seg) => seg.tone === 'crit' && seg.count > 0)) return 'crit'
  if (segments.some((seg) => seg.tone === 'warn' && seg.count > 0)) return 'warn'
  if (segments.some((seg) => seg.tone === 'ok' && seg.count > 0)) return 'ok'
  return 'muted'
}

function healthLabel(tone) {
  switch (tone) {
    case 'crit': return 'Needs attention'
    case 'warn': return 'Degraded'
    case 'ok': return 'Healthy'
    default: return 'No data'
  }
}

function DonutLegend({ segments, total }) {
  return (
    <ul className="workload-donut-legend">
      {segments.map((seg) => {
        const pct = total > 0 ? Math.round((seg.count / total) * 100) : 0
        return (
          <li key={seg.label} className={`workload-donut-legend-row tone-${seg.tone}`}>
            <span className={`workload-donut-swatch tone-${seg.tone}`} aria-hidden="true" />
            <span className="workload-donut-legend-label">{seg.label}</span>
            <span className="workload-donut-legend-meta overview-num">
              {seg.count}
              {pct > 0 && <span className="workload-donut-legend-pct overview-num">{pct}%</span>}
            </span>
          </li>
        )
      })}
    </ul>
  )
}

function WorkloadKindDonut({ card, onSelectKind }) {
  const hasData = card.total > 0 && card.segments.length > 0
  const gradient = donutGradient(card.segments)
  const tone = dominantTone(card.segments)

  return (
    <article className={[
      'workload-kind-card',
      `tone-${tone}`,
      hasData ? '' : 'is-empty',
    ].filter(Boolean).join(' ')}>
      <button
        type="button"
        className="workload-kind-card-title"
        onClick={() => onSelectKind?.(card)}
      >
        <span className="workload-kind-card-icon">
          <KindIcon kind={card.kind} size={18} />
        </span>
        <span className="workload-kind-card-copy">
          <span className="workload-kind-card-name">{card.label}</span>
          <span className={`workload-kind-card-health tone-${tone}`}>{healthLabel(tone)}</span>
        </span>
        <span className="workload-kind-card-count overview-num">{card.total}</span>
      </button>

      {hasData ? (
        <>
          <div className="workload-donut-wrap" aria-hidden="true">
            <div className="workload-donut-glow" />
            <div className="workload-donut" style={{ background: gradient }}>
              <div className="workload-donut-hole">
                <span className="workload-donut-total overview-num is-display">{card.total}</span>
                <span className="workload-donut-caption overview-label">in scope</span>
              </div>
            </div>
          </div>
          <DonutLegend segments={card.segments} total={card.total} />
        </>
      ) : card.denied ? (
        <p className="workload-kind-empty workload-kind-denied" title="Access denied">
          Access denied
        </p>
      ) : (
        <p className="workload-kind-empty muted">No {card.label.toLowerCase()} in scope</p>
      )}
    </article>
  )
}

export function WorkloadOverviewPanel({
  cluster,
  browseScope,
  kindGroups = [],
  catalogLoading = false,
  clusterStatus = null,
  onSelectKind,
}) {
  const { cards, podEntities, loading, error, deniedCount } = useWorkloadOverview({
    cluster,
    browseScope,
    kindGroups,
    enabled: kindGroups.length > 0,
  })

  const busy = catalogLoading || loading

  const resourceMetrics = useMemo(
    () => buildOverviewMetricsSummary(summarizeBrowsePodMetrics(podEntities)),
    [podEntities],
  )

  const nodes = clusterStatus?.nodes || {}

  const summary = useMemo(() => {
    let total = 0
    let healthy = 0
    let attention = 0
    for (const card of cards) {
      total += card.total
      for (const seg of card.segments) {
        if (seg.tone === 'ok') healthy += seg.count
        if (seg.tone === 'warn' || seg.tone === 'crit') attention += seg.count
      }
    }
    return { total, healthy, attention }
  }, [cards])

  if (!kindGroups.length) {
    return (
      <div className="workload-overview-panel">
        {catalogLoading ? (
          <LoadingState message="Loading workload types…" />
        ) : (
          <p className="muted">No workload types discovered in this scope.</p>
        )}
      </div>
    )
  }

  return (
    <div className="workload-overview-panel">
      <header className="workload-overview-header">
        <div className="workload-overview-header-main">
          <div>
            <h2 className="workload-overview-title">Overview</h2>
            <p className="workload-overview-lead muted">
              Status breakdown for workload types in the current browse scope.
            </p>
          </div>
          {!busy && cards.length > 0 && (
            <div className="workload-overview-summary" aria-label="Scope summary">
              <div className="workload-overview-stat">
                <span className="workload-overview-stat-value overview-num">{summary.total}</span>
                <span className="workload-overview-stat-label overview-label">Resources</span>
              </div>
              <div className="workload-overview-stat tone-ok">
                <span className="workload-overview-stat-value overview-num">{summary.healthy}</span>
                <span className="workload-overview-stat-label overview-label">Healthy</span>
              </div>
              <div className={`workload-overview-stat ${summary.attention > 0 ? 'tone-warn' : 'tone-muted'}`}>
                <span className="workload-overview-stat-value overview-num">{summary.attention}</span>
                <span className="workload-overview-stat-label overview-label">Attention</span>
              </div>
            </div>
          )}
        </div>
        {busy && (
          <InlineLoading message="Loading overview…" className="workload-overview-status muted" />
        )}
        {deniedCount > 0 && !busy && (
          <p className="workload-overview-status scope-toolbar-warn">
            {deniedCount === 1
              ? 'One workload type is not listable with the current identity.'
              : `${deniedCount} workload types are not listable with the current identity.`}
          </p>
        )}
        {error && !busy && (
          <p className="workload-overview-status scope-toolbar-warn" title={error}>
            Some workload lists could not be loaded.
          </p>
        )}
      </header>

      {!busy && (
        <section className="workload-overview-metrics" aria-label="Resource usage">
          <div className="workload-overview-section-head">
            <h3 className="workload-overview-section-title">Resource usage</h3>
            {!resourceMetrics.cpu.available && !resourceMetrics.hasAlloc && (
              <p className="workload-overview-metrics-hint muted">{resourceMetrics.note}</p>
            )}
          </div>
          <div className="workload-overview-metrics-grid">
            {(resourceMetrics.cpu.available || resourceMetrics.hasAlloc) ? (
              <>
                <OverviewMetricCard metric={resourceMetrics.cpu} note={resourceMetrics.note} />
                <OverviewMetricCard metric={resourceMetrics.memory} note={resourceMetrics.note} />
              </>
            ) : (
              <article className="resource-metric-card resource-metric-card-empty tone-muted">
                <div className="resource-metric-card-head">
                  <span className="resource-metric-card-label">Resource usage</span>
                </div>
                <p className="resource-metric-card-value overview-num">Unavailable</p>
                <p className="resource-metric-card-foot muted">{resourceMetrics.note}</p>
              </article>
            )}
            <OverviewNodesCard ready={nodes.ready ?? 0} total={nodes.total} />
          </div>
        </section>
      )}

      <section className="workload-overview-workloads" aria-label="Workload types">
        <div className="workload-overview-section-head">
          <h3 className="workload-overview-section-title">Workloads</h3>
          <p className="workload-overview-section-lead muted">Health breakdown by kind in the current scope.</p>
        </div>
      <div className="workload-overview-grid-wrap">
        {busy && (
          <div className="workload-overview-grid-loading" aria-hidden="true">
            <LoadingState message="Loading overview…" />
          </div>
        )}
        <div className={`workload-overview-grid ${busy ? 'is-loading' : ''}`}>
          {cards.map((card) => (
            <WorkloadKindDonut
              key={card.kind}
              card={card}
              onSelectKind={onSelectKind}
            />
          ))}
        </div>
      </div>
      </section>
    </div>
  )
}
