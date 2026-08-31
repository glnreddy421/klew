import { formatClock } from '../../lib/investigationOverview.js'

export function InvestigationHeader({
  verdict,
  stats,
  timeWindowLabel,
  visibilityWarning,
  onNavigate,
}) {
  return (
    <header className="brief-verdict inv-header">
      <div className="brief-verdict-top">
        <span className="brief-kicker">Investigation</span>
        <span className="brief-window-context muted">
          {verdict.live && <span className="brief-live">● Live</span>}
          {verdict.live && ' · '}
          {verdict.windowLabel || timeWindowLabel}
          {visibilityWarning && (
            <span className="inv-limited-badge" title={visibilityWarning.message}>
              {' · '}Limited visibility
            </span>
          )}
        </span>
      </div>

      <div className="brief-verdict-body">
        <div className="brief-verdict-main">
          {verdict.statusLabel && (
            <span className={`brief-status tone-${verdict.statusTone}`}>{verdict.statusLabel}</span>
          )}
          <h1 className="brief-headline">{verdict.headline}</h1>
          {verdict.summary && (
            <p className="brief-summary muted">{verdict.summary}</p>
          )}
        </div>

        {(verdict.startedAt || verdict.confidenceText) && (
          <dl className="brief-verdict-meta">
            {verdict.startedAt && (
              <div className="brief-meta-item">
                <dt>Started</dt>
                <dd className="mono">{formatClock(verdict.startedAt)}</dd>
              </div>
            )}
            {verdict.confidenceText && (
              <div className="brief-meta-item">
                <dt>Confidence</dt>
                <dd>{verdict.confidenceText}</dd>
              </div>
            )}
          </dl>
        )}
      </div>

      <InvestigationStats stats={stats} onNavigate={onNavigate} />
    </header>
  )
}

function InvestigationStats({ stats, onNavigate }) {
  const items = [
    { key: 'signals', label: 'Signals', tab: 'evidence', count: stats.signals },
    { key: 'failures', label: 'Failures', tab: 'failures', count: stats.failures },
    { key: 'patterns', label: 'Patterns', tab: 'patterns', count: stats.patterns },
    { key: 'resources', label: 'Resources', tab: 'resources', count: stats.resources },
    { key: 'evidence', label: 'Evidence', tab: 'evidence', count: stats.evidence },
  ]

  return (
    <div className="brief-stats" role="list">
      {items.map((item, i) => (
        <span key={item.key} className="brief-stat-wrap">
          {i > 0 && <span className="brief-stat-sep muted" aria-hidden="true">·</span>}
          <button
            type="button"
            className="brief-stat"
            onClick={() => onNavigate?.(item.tab)}
            role="listitem"
          >
            <span className="brief-stat-count mono">{item.count}</span>
            <span className="brief-stat-label">{item.label}</span>
          </button>
        </span>
      ))}
    </div>
  )
}
