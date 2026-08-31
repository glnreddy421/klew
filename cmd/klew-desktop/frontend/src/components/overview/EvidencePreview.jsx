import { formatClock } from '../../lib/investigationOverview.js'

export function EvidencePreview({
  items,
  totalCount,
  onNavigate,
  onOpenEvidence,
}) {
  if (!items?.length) return null

  const openEvidence = onOpenEvidence || onNavigate

  return (
    <section className="inv-visual-section inv-evidence-section">
      <div className="inv-section-head">
        <h2 className="inv-section-title">Supporting evidence</h2>
        <span className="muted inv-section-count">{totalCount} items</span>
      </div>
      <ul className="inv-evidence-list">
        {items.map((ev) => (
          <li key={ev.id || `${ev.timestamp}-${ev.headline}`}>
            <button
              type="button"
              className="inv-evidence-row"
              onClick={() => openEvidence?.('evidence')}
            >
              <span className="mono inv-evidence-time">{formatClock(ev.timestamp)}</span>
              <span className={`inv-evidence-type type-${String(ev.type || 'evidence').toLowerCase()}`}>{ev.type || 'EVIDENCE'}</span>
              <span className="inv-evidence-body">
                <strong>{ev.headline}</strong>
                {ev.sourceName && <span className="muted"> · {ev.sourceName}</span>}
              </span>
            </button>
          </li>
        ))}
      </ul>
      <button type="button" className="text-link-btn" onClick={() => openEvidence?.('evidence')}>
        View all evidence →
      </button>
    </section>
  )
}
