import { useMemo, useState } from 'react'
import { formatClock, getState } from '../lib/investigationViews'
import { matchEvidenceToTemplate } from '../lib/streamView'
import { SEVERITY_ROWS, SeverityCountsBody } from '../components/SeverityDonut'

/**
 * Event Patterns 2×2 from Pod / Node / PVC events.
 * Used standalone (legacy) or embedded inside PatternsView.
 */
export function TimelineView({ view, embedded = false }) {
  const state = getState(view)
  const patterns = view?.logPatterns || state.logPatterns || null
  const evidence = view?.evidence || state.liveEvidence || []

  const templates = patterns?.eventTemplates || []
  const words = patterns?.eventWords || []
  const reasons = patterns?.eventReasons || []
  const severity = patterns?.eventSeverity || {}
  const window = patterns?.eventWindow || {}

  const [expandedId, setExpandedId] = useState(null)
  const expanded = templates.find((t) => t.id === expandedId) || null

  const maxWord = words.length ? (words[0].score || words[0].count || 1) : 1
  const maxReason = reasons.length ? (reasons[0].score || reasons[0].count || 1) : 1
  const sevTotal = SEVERITY_ROWS.reduce((n, r) => n + (severity[r.id] || 0), 0)

  const matched = useMemo(() => {
    if (!expanded) return []
    return matchEvidenceToTemplate(evidence, expanded, [], 24, {
      sourceTypes: ['k8s_event'],
    })
  }, [evidence, expanded])

  const empty = !templates.length && !words.length && !reasons.length
  const shellClass = embedded ? 'event-patterns-embed' : 'inv-page infra-patterns-page'

  if (empty) {
    return (
      <div className={shellClass}>
        <div className="inv-empty muted">
          No Pod, Node, or PVC events in this investigation yet.
        </div>
      </div>
    )
  }

  return (
    <div className={shellClass}>
      <div className="gp">
        <div className="gp-grid">
          <section className="gp-card">
            <header className="gp-card-h">
              Top Words
            </header>
            <ol className="gp-rank-list">
              {words.map((w) => (
                <li key={w.word}>
                  <div className="gp-rank-row static" title={w.word}>
                    <span className="gp-rank mono">{w.rank}</span>
                    <span className="gp-rank-label mono" title={w.word}>{w.word}</span>
                    <span className="gp-rank-count mono">{fmtCount(w.count)}</span>
                    <span className="gp-rank-bar" aria-hidden="true">
                      <span style={{ width: `${Math.round(((w.score || w.count) / maxWord) * 100)}%` }} />
                    </span>
                  </div>
                </li>
              ))}
              {!words.length && <li className="gp-empty muted">No tokens</li>}
            </ol>
          </section>

          <section className="gp-card">
            <header className="gp-card-h">
              Top Reasons
            </header>
            <ol className="gp-rank-list">
              {reasons.map((a) => (
                <li key={a.key}>
                  <div className="gp-rank-row static" title={a.key}>
                    <span className="gp-rank mono">{a.rank}</span>
                    <span className="gp-rank-label mono" title={a.key}>{a.key}</span>
                    <span className="gp-rank-count mono">{fmtCount(a.count)}</span>
                    <span className="gp-rank-bar gp-rank-bar-attr" aria-hidden="true">
                      <span style={{ width: `${Math.round(((a.score || a.count) / maxReason) * 100)}%` }} />
                    </span>
                  </div>
                </li>
              ))}
              {!reasons.length && <li className="gp-empty muted">No reasons yet</li>}
            </ol>
          </section>

          <section className="gp-card gp-card-patterns">
            <header className="gp-card-h">
              Event Patterns
              <span className="gp-card-sub muted">
                {window.patternCount || templates.length} pattern
                {(window.patternCount || templates.length) === 1 ? '' : 's'}
                {' · Pod / Node / PVC'}
              </span>
            </header>
            <ul className="gp-pattern-list">
              {templates.map((t, i) => {
                const open = expandedId === t.id
                const barPct = Math.max(2, Math.min(100, t.pct || 0))
                return (
                  <li key={t.id} className={open ? 'expanded' : ''}>
                    <button
                      type="button"
                      className={`gp-pattern-row ${open ? 'active' : ''}`}
                      onClick={() => setExpandedId(open ? null : t.id)}
                      aria-expanded={open}
                    >
                      <span className={`gp-pattern-bar tone-${barTone(i, templates.length)}`} aria-hidden="true">
                        <span style={{ width: `${barPct}%` }} />
                      </span>
                      <span className="gp-pattern-pct mono">{Number(t.pct || 0).toFixed(1)}%</span>
                      <span className="gp-pattern-tpl mono" title={t.template}>{t.template}</span>
                    </button>
                    {open && (
                      <div className="gp-pattern-expand">
                        {!matched.length && (
                          <div className="gp-empty muted">No matching event samples in the current window.</div>
                        )}
                        {matched.map((line) => (
                          <div key={line.id} className={`gp-expand-line sev-${line.severity}`}>
                            <span className="muted mono">{line.time || formatClock(null)}</span>
                            <span className="muted mono gp-expand-pod" title={line.pod}>{line.pod}</span>
                            <span className="gp-expand-msg mono">{line.message}</span>
                          </div>
                        ))}
                      </div>
                    )}
                  </li>
                )
              })}
              {!templates.length && <li className="gp-empty muted">No event clusters yet</li>}
            </ul>
          </section>

          <section className="gp-card gp-card-counts">
            <header className="gp-card-h">Event Counts</header>
            <SeverityCountsBody
              severity={severity}
              label="Event severity breakdown"
            />
            <div className="gp-counts-foot muted mono">
              TOTAL: {sevTotal}
              {' · '}
              objects: {window.podCount || 0}
            </div>
          </section>
        </div>
      </div>
    </div>
  )
}

function fmtCount(n) {
  const x = Number(n) || 0
  if (x >= 1_000_000) return `${(x / 1_000_000).toFixed(1)}M`
  if (x >= 10_000) return `${Math.round(x / 1000)}k`
  if (x >= 1000) return `${(x / 1000).toFixed(1)}k`
  return String(x)
}

function barTone(i, n) {
  const t = n <= 1 ? 0 : i / (n - 1)
  if (t < 0.2) return 'hot'
  if (t < 0.45) return 'warm'
  if (t < 0.7) return 'mid'
  return 'cool'
}
