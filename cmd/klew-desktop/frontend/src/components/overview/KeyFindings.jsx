import { useState } from 'react'

function severityTone(sev) {
  const r = String(sev || '').toLowerCase()
  if (r === 'critical' || r === 'error' || r === 'fatal') return 'crit'
  if (r === 'warning' || r === 'warn' || r === 'degraded') return 'warn'
  return 'neutral'
}

export function KeyFindings({
  findings,
  onNavigate,
  onHighlightNodes,
}) {
  const [hoveredId, setHoveredId] = useState(null)

  if (!findings.length) return null

  function handleHover(finding, active) {
    setHoveredId(active ? finding.id : null)
    onHighlightNodes?.(active ? (finding.chainNodeIds || []) : [])
  }

  return (
    <section className="inv-visual-section inv-findings-section">
      <div className="inv-section-head">
        <h2 className="inv-section-title">Key findings</h2>
      </div>
      <ol className="inv-findings">
        {findings.map((f) => (
          <li
            key={f.id}
            className={`inv-finding tone-${severityTone(f.severity)} ${hoveredId === f.id ? 'is-hovered' : ''}`}
            onMouseEnter={() => handleHover(f, true)}
            onMouseLeave={() => handleHover(f, false)}
          >
            <span className="inv-finding-rank mono">{String(f.rank).padStart(2, '0')}</span>
            <div className="inv-finding-body">
              <h3 className="inv-finding-title">{f.title}</h3>
              {f.meta && <p className="inv-finding-meta muted">{f.meta}</p>}
              {f.nav && (
                <button
                  type="button"
                  className="text-link-btn inv-finding-link"
                  onClick={() => onNavigate?.(f.nav.tab)}
                >
                  {f.nav.label}
                </button>
              )}
            </div>
          </li>
        ))}
      </ol>
    </section>
  )
}
