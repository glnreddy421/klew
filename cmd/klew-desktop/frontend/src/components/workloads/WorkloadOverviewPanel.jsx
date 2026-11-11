import { useMemo } from 'react'
import { KindIcon } from '../KindIcon'
import { LoadingBar, ResourcePaneLoading } from '../LoadingSpinner.jsx'
import { DataAgeLabel } from '../DataAgeLabel.jsx'
import { RefreshButton } from '../RefreshButton.jsx'
import { useWorkloadOverview } from '../../hooks/useWorkloadOverview.js'
import { useRecentWarnings } from '../../hooks/useRecentWarnings.js'
import { AttentionList, RecentWarningsList } from './OverviewActivity.jsx'
import { summarizeBrowsePodMetrics } from '../../lib/browseMetrics.js'
import { buildOverviewMetricsSummary } from '../../lib/metricDisplay.js'
import { OverviewMetricCard, OverviewNodesCard } from '../metrics/ResourceMetricDisplay.jsx'
import { donutArcs, summarizeWorkloadCards } from '../../lib/workloadOverview.js'

function dominantTone(segments = []) {
  for (const tone of ['crit', 'warn', 'ok', 'info']) {
    if (segments.some((seg) => seg.tone === tone && seg.count > 0)) return tone
  }
  return 'muted'
}

function healthLabel(tone, hasData) {
  switch (tone) {
    case 'crit': return 'Needs attention'
    case 'warn': return 'Degraded'
    case 'ok': return 'Healthy'
    case 'info': return 'In progress'
    default: return hasData ? 'Inactive' : 'No data'
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
              <span className="workload-donut-legend-count">{seg.count}</span>
              {pct > 0 && <span className="workload-donut-legend-pct">{pct}%</span>}
            </span>
          </li>
        )
      })}
    </ul>
  )
}

function WorkloadKindDonut({ card, onSelectKind }) {
  const hasData = card.total > 0 && card.segments.length > 0
  const arcs = useMemo(() => donutArcs(card.segments), [card.segments])
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
          <KindIcon kind={card.kind} size={16} />
        </span>
        <span className="workload-kind-card-copy">
          <span className="workload-kind-card-name">{card.label}</span>
          <span className={`workload-kind-card-health tone-${tone}`}>{healthLabel(tone, hasData)}</span>
        </span>
        <span className="workload-kind-card-count overview-num">{card.total}</span>
      </button>

      {hasData ? (
        <div className="workload-donut-body">
          <div className="workload-donut-wrap" aria-hidden="true">
            <svg className="workload-donut" viewBox="0 0 100 100" width="100" height="100">
              <circle className="workload-donut-track" cx="50" cy="50" r="36" />
              {arcs.map((arc) => (
                arc.full ? (
                  <circle
                    key={arc.label}
                    className={`workload-donut-arc tone-${arc.tone}`}
                    cx="50"
                    cy="50"
                    r="36"
                    stroke={arc.color}
                  />
                ) : (
                  <path
                    key={arc.label}
                    className={`workload-donut-arc tone-${arc.tone}`}
                    d={arc.d}
                    stroke={arc.color}
                  />
                )
              ))}
            </svg>
            <div className="workload-donut-center">
              <span className="workload-donut-total overview-num is-display">{card.total}</span>
              <span className="workload-donut-caption overview-label">total</span>
            </div>
          </div>
          <DonutLegend segments={card.segments} total={card.total} />
        </div>
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
  clusterStatus = null,
  clusterMonitoringEnabled = true,
  onSelectKind,
  onInspect,
}) {
  const {
    cards,
    attentionItems,
    podEntities,
    loading,
    refreshing,
    error,
    deniedCount,
    hasData,
    updatedAt,
    live,
    refresh,
  } = useWorkloadOverview({
    cluster,
    browseScope,
    kindGroups,
    enabled: kindGroups.length > 0 && clusterMonitoringEnabled,
  })
  const warnings = useRecentWarnings({
    cluster,
    browseScope,
    enabled: clusterMonitoringEnabled,
  })

  const hasOverview = hasData || cards.some((c) => c.total > 0)
  const initialLoad = loading && !hasOverview
  // Only show background refresh while overview data is actively revalidating.
  const backgroundRefresh = refreshing

  const resourceMetrics = useMemo(
    () => buildOverviewMetricsSummary(summarizeBrowsePodMetrics(podEntities)),
    [podEntities],
  )

  const nodes = clusterStatus?.nodes || {}

  const summary = useMemo(() => summarizeWorkloadCards(cards), [cards])

  if (!kindGroups.length) {
    return (
      <div className="workload-overview-panel">
        {loading && !hasOverview ? (
          <ResourcePaneLoading label="Loading overview" />
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
          {!initialLoad && cards.length > 0 && (
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
          <RefreshButton
            onClick={() => {
              refresh?.()
              warnings.refresh?.()
            }}
            spinning={backgroundRefresh}
            disabled={initialLoad}
            title="Refresh overview"
            className="icon-refresh-btn workload-overview-refresh-btn"
          />
        </div>
        {backgroundRefresh && (
          <LoadingBar className="workload-overview-loading-bar" label="Updating overview" />
        )}
        {!initialLoad && (live || updatedAt > 0) && (
          <span className="workload-overview-status">
            {live && <span className="brief-live">● Live</span>}
            {live && updatedAt > 0 && ' · '}
            {updatedAt > 0 && (
              <DataAgeLabel updatedAt={updatedAt} />
            )}
          </span>
        )}
        {deniedCount > 0 && !initialLoad && (
          <p className="workload-overview-status scope-toolbar-warn">
            {deniedCount === 1
              ? 'One workload type is not listable with the current identity.'
              : `${deniedCount} workload types are not listable with the current identity.`}
          </p>
        )}
        {error && !initialLoad && (
          <p className="workload-overview-status scope-toolbar-warn" title={error}>
            Some workload lists could not be loaded.
          </p>
        )}
      </header>

      {!initialLoad && (
        <section className="workload-overview-metrics" aria-label="Resource usage">
          <div className="workload-overview-section-head">
            <h3 className="workload-overview-section-title">Resource usage</h3>
            <p className="workload-overview-section-lead muted">
              {resourceMetrics.cpu.available || resourceMetrics.hasAlloc
                ? resourceMetrics.note
                : 'Pod requests, limits and live usage when metrics-server is available.'}
            </p>
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
            <OverviewNodesCard
              ready={nodes.ready ?? 0}
              total={nodes.total}
              notReady={nodes.notReady ?? 0}
              pressured={nodes.pressured ?? 0}
            />
          </div>
        </section>
      )}

      <section className="workload-overview-workloads" aria-label="Workload types">
        <div className="workload-overview-section-head">
          <h3 className="workload-overview-section-title">Workloads</h3>
          <p className="workload-overview-section-lead muted">Health breakdown by kind in the current scope.</p>
        </div>
      <div className="workload-overview-grid-wrap">
        {initialLoad ? (
          <ResourcePaneLoading label="Loading overview" />
        ) : (
          <div className="workload-overview-grid">
            {cards.map((card) => (
              <WorkloadKindDonut
                key={card.kind}
                card={card}
                onSelectKind={onSelectKind}
              />
            ))}
          </div>
        )}
      </div>
      </section>

      {!initialLoad && (
        <section className="overview-activity" aria-label="Activity">
          <AttentionList items={attentionItems} loading={loading} onInspect={onInspect} />
          <RecentWarningsList
            events={warnings.events}
            total={warnings.total}
            accessState={warnings.accessState}
            error={warnings.error}
            loading={warnings.loading}
            onInspect={onInspect}
          />
        </section>
      )}
    </div>
  )
}
