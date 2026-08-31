import { getNodeMarker, nodeTypeClass } from '../../lib/investigationOverviewVisuals.js'
import { formatClock } from '../../lib/investigationOverview.js'

export function InvestigationTimeline({
  timelineVisual,
  onNavigate,
  onItemSelect,
}) {
  if (!timelineVisual?.items?.length) return null

  const { items, span, hasMore, moreNav } = timelineVisual

  return (
    <section className="inv-visual-section inv-timeline-section">
      <div className="inv-section-head">
        <h2 className="inv-section-title">Investigation timeline</h2>
        {hasMore && moreNav && (
          <button
            type="button"
            className="text-link-btn"
            onClick={() => onNavigate?.(moreNav.tab)}
          >
            {moreNav.label}
          </button>
        )}
      </div>

      <div className="inv-timeline-visual">
        <div className="inv-timeline-axis">
          {items.map((item) => (
            <span key={`${item.id}-axis`} className="inv-timeline-axis-time mono">
              {item.timeLabel}
            </span>
          ))}
        </div>

        <div className="inv-timeline-track">
          <div className="inv-timeline-spine" aria-hidden="true" />
          {items.map((item, i) => (
            <button
              key={item.id}
              type="button"
              className={`inv-timeline-node ${nodeTypeClass(item.nodeType)}`}
              style={span ? { left: `${positionPct(item.timestamp, span)}%` } : { flex: 1 }}
              onClick={() => {
                onItemSelect?.(item)
                onNavigate?.(item.navTab || 'evidence')
              }}
              title={buildHoverTitle(item)}
            >
              <span className="inv-timeline-marker" aria-hidden="true">
                {getNodeMarker(item.nodeType)}
              </span>
              {i < items.length - 1 && span && (
                <span className="inv-timeline-connector" aria-hidden="true" />
              )}
            </button>
          ))}
        </div>

        <div className="inv-timeline-labels">
          {items.map((item) => (
            <button
              key={`${item.id}-lbl`}
              type="button"
              className="inv-timeline-label-btn"
              style={span ? { left: `${positionPct(item.timestamp, span)}%` } : undefined}
              onClick={() => onNavigate?.(item.navTab || 'evidence')}
              title={buildHoverTitle(item)}
            >
              <span className="inv-timeline-label-text">{item.shortLabel}</span>
              {item.count > 1 && <span className="muted"> ×{item.count}</span>}
            </button>
          ))}
        </div>
      </div>

      <ul className="inv-timeline-list sr-only">
        {items.map((item) => (
          <li key={`${item.id}-sr`}>
            {formatClock(item.timestamp)} — {item.label}
          </li>
        ))}
      </ul>
    </section>
  )
}

function positionPct(ts, span) {
  if (!ts || !span) return 0
  const t = new Date(ts).getTime()
  const range = span.end - span.start
  if (range <= 0) return 0
  return Math.max(0, Math.min(100, ((t - span.start) / range) * 100))
}

function buildHoverTitle(item) {
  const lines = [item.hoverTitle || item.label]
  if (item.timestamp) lines.push(formatClock(item.timestamp))
  if (item.hoverMeta) lines.push(item.hoverMeta)
  if (item.hoverEvidenceCount > 0) {
    lines.push(`${item.hoverEvidenceCount} supporting evidence item${item.hoverEvidenceCount === 1 ? '' : 's'}`)
  }
  return lines.join('\n')
}
