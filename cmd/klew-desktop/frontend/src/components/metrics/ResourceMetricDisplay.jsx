import { metricToneFromPct } from '../../lib/metricDisplay.js'

function MetricIcon({ kind }) {
  if (kind === 'memory') {
    return (
      <svg className="resource-metric-icon" viewBox="0 0 24 24" aria-hidden="true">
        <rect x="4" y="6" width="16" height="12" rx="2" fill="none" stroke="currentColor" strokeWidth="1.6" />
        <path d="M8 10h8M8 14h5" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
      </svg>
    )
  }
  if (kind === 'nodes') {
    return (
      <svg className="resource-metric-icon" viewBox="0 0 24 24" aria-hidden="true">
        <rect x="3" y="4" width="18" height="6" rx="1.5" fill="none" stroke="currentColor" strokeWidth="1.6" />
        <rect x="3" y="14" width="18" height="6" rx="1.5" fill="none" stroke="currentColor" strokeWidth="1.6" />
        <circle cx="7" cy="7" r="1" fill="currentColor" />
        <circle cx="7" cy="17" r="1" fill="currentColor" />
      </svg>
    )
  }
  return (
    <svg className="resource-metric-icon" viewBox="0 0 24 24" aria-hidden="true">
      <rect x="5" y="5" width="14" height="14" rx="2.5" fill="none" stroke="currentColor" strokeWidth="1.6" />
      <path d="M9 9h6v6H9z" fill="currentColor" opacity="0.35" />
      <path d="M9 3v2M15 3v2M9 19v2M15 19v2M3 9h2M3 15h2M19 9h2M19 15h2" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" />
    </svg>
  )
}

export function ResourceMetricBar({
  usagePct,
  requestPct,
  limitPct,
  tone = 'ok',
  compact = false,
  showAlloc = true,
}) {
  const hasUsage = usagePct != null && usagePct > 0
  const hasRequest = showAlloc && requestPct != null && requestPct > 0
  const hasLimit = showAlloc && limitPct != null && limitPct > 0

  return (
    <div
      className={[
        'resource-metric-bar',
        compact ? 'is-compact' : '',
        `tone-${tone}`,
        hasUsage ? 'has-usage' : '',
      ].filter(Boolean).join(' ')}
      aria-hidden="true"
    >
      <div className="resource-metric-bar-track">
        {hasLimit && (
          <div className="resource-metric-bar-fill is-limit" style={{ width: `${limitPct}%` }} />
        )}
        {hasRequest && (
          <div className="resource-metric-bar-fill is-request" style={{ width: `${requestPct}%` }} />
        )}
        {hasUsage && (
          <div className="resource-metric-bar-fill is-usage" style={{ width: `${usagePct}%` }} />
        )}
      </div>
    </div>
  )
}

export function PodMetricCell({ metric, fallback = '—' }) {
  if (!metric?.hasLive && !metric?.hasAlloc) {
    return <span className="resource-metric-cell-empty">{fallback}</span>
  }

  const tone = metric.hasLive ? metricToneFromPct(metric.pct) : 'muted'

  return (
    <div className={`resource-metric-cell tone-${tone}`} title={metric.title}>
      <ResourceMetricBar
        compact
        tone={tone}
        usagePct={metric.usagePct}
        requestPct={metric.requestPct}
        limitPct={metric.limitPct}
        showAlloc={!metric.hasLive || metric.hasAlloc}
      />
      <span className="resource-metric-cell-value mono">{metric.label}</span>
      {metric.hasLive && <span className="resource-metric-live-dot" title="Live usage" aria-hidden="true" />}
    </div>
  )
}

export function OverviewMetricCard({ metric, note }) {
  const kind = metric.kind === 'memory' ? 'memory' : 'cpu'
  const title = kind === 'memory' ? 'Memory' : 'CPU'
  const live = metric.available && metric.usageLabel

  return (
    <article className={`resource-metric-card tone-${metric.tone}`}>
      <div className="resource-metric-card-head">
        <span className={`resource-metric-card-icon-wrap tone-${metric.tone}`}>
          <MetricIcon kind={kind} />
        </span>
        <div className="resource-metric-card-copy">
          <span className="resource-metric-card-label">{title}</span>
          {live ? (
            <span className="resource-metric-live-badge">Live</span>
          ) : metric.hasAlloc ? (
            <span className="resource-metric-alloc-badge">Allocated</span>
          ) : null}
        </div>
        {live && metric.pct != null && (
          <span className={`resource-metric-pct overview-num tone-${metric.tone}`}>{metric.pct}%</span>
        )}
      </div>

      <div className="resource-metric-card-value-row">
        <span className="resource-metric-card-value overview-num is-display">
          {live ? metric.usageLabel : (metric.hasAlloc ? metric.denomLabel : '—')}
        </span>
        {live && (
          <span className="resource-metric-card-denom muted overview-num is-secondary">/ {metric.denomLabel}</span>
        )}
      </div>

      <ResourceMetricBar
        tone={metric.tone}
        usagePct={live ? metric.usagePct : null}
        requestPct={metric.requestPct}
        limitPct={metric.limitPct}
        showAlloc={metric.hasAlloc}
      />

      <p className="resource-metric-card-foot muted">
        {live ? (
          <>
            {metric.requestLabel && <>req {metric.requestLabel}</>}
            {metric.requestLabel && metric.limitLabel && ' · '}
            {metric.limitLabel && <>lim {metric.limitLabel}</>}
            {!metric.requestLabel && !metric.limitLabel && 'Usage vs requests/limits in scope'}
          </>
        ) : metric.hasAlloc ? (
          <>
            {metric.requestLabel && <>request {metric.requestLabel}</>}
            {metric.requestLabel && metric.limitLabel && ' · '}
            {metric.limitLabel && <>limit {metric.limitLabel}</>}
          </>
        ) : (
          note || 'Install metrics-server for live usage'
        )}
      </p>
    </article>
  )
}

export function OverviewNodesCard({ ready = 0, total }) {
  const hasTotal = total != null && total > 0
  const pct = hasTotal ? Math.min(100, Math.round((ready / total) * 100)) : null
  const tone = !hasTotal ? 'muted' : ready === total ? 'ok' : ready === 0 ? 'crit' : 'warn'

  return (
    <article className={`resource-metric-card resource-metric-card-nodes tone-${tone}`}>
      <div className="resource-metric-card-head">
        <span className={`resource-metric-card-icon-wrap tone-${tone}`}>
          <MetricIcon kind="nodes" />
        </span>
        <div className="resource-metric-card-copy">
          <span className="resource-metric-card-label">Nodes</span>
          <span className="resource-metric-alloc-badge">Cluster</span>
        </div>
        {pct != null && (
          <span className={`resource-metric-pct overview-num tone-${tone}`}>{pct}%</span>
        )}
      </div>

      <div className="resource-metric-card-value-row">
        <span className="resource-metric-card-value overview-num is-display">{ready}</span>
        <span className="resource-metric-card-denom muted overview-num is-secondary">/ {hasTotal ? total : '—'} ready</span>
      </div>

      <div className="resource-metric-node-ring" style={{ '--pct': pct ?? 0 }} aria-hidden="true">
        <div className="resource-metric-node-ring-inner" />
      </div>
    </article>
  )
}
