import { useMemo, useState } from 'react'
import { formatClock } from '../lib/investigationViews'
import { matchEvidenceToTemplate } from '../lib/streamView'
import { SEVERITY_ROWS, SeverityCountsBody } from './SeverityDonut'

/**
 * Patterns dashboard — ranked templates, words, and fields from tailed logs.
 * Pattern click expands matching lines; word/field click filters the log stream.
 */
export function StreamPatternsPanel({
  patterns = null,
  evidence = [],
  running = false,
  selectedPods = [],
  onFilterLogs,
}) {
  const [expandedId, setExpandedId] = useState(null)
  const [activeFilter, setActiveFilter] = useState(null)

  const templates = patterns?.templates || []
  const words = patterns?.words || []
  const attributes = patterns?.attributes || []
  const severity = patterns?.severity || {}
  const window = patterns?.window || {}

  const maxWord = words.length ? (words[0].score || words[0].count || 1) : 1
  const maxAttr = attributes.length ? (attributes[0].score || attributes[0].count || 1) : 1
  const sevTotal = SEVERITY_ROWS.reduce((n, r) => n + (severity[r.id] || 0), 0)

  const expanded = templates.find((t) => t.id === expandedId) || null
  const matchedLogs = useMemo(() => {
    if (!expanded?.template) return []
    // Prefer backend samples, then token match against live lines.
    return matchEvidenceToTemplate(evidence, expanded, selectedPods, 24)
  }, [evidence, expanded, selectedPods])

  const applyFilter = (term, kind) => {
    const next = String(term || '').trim()
    if (!next) return
    if (activeFilter === next) {
      setActiveFilter(null)
      onFilterLogs?.('')
      return
    }
    setActiveFilter(next)
    setExpandedId(null)
    onFilterLogs?.(next, kind)
  }

  if (!running) {
    return (
      <div className="empty-stream patterns-empty">
        <strong>No patterns yet</strong>
        <span>Start an investigation to collect logs.</span>
      </div>
    )
  }

  if (!templates.length && !words.length) {
    return (
      <div className="empty-stream patterns-empty">
        <strong>No patterns yet</strong>
        <span>Waiting for patterns from tailed logs…</span>
      </div>
    )
  }

  return (
    <div className="gp">
      <div className="gp-grid">
        <section className="gp-card">
          <header className="gp-card-h">
            Top Words
          </header>
          <ol className="gp-rank-list">
            {words.map((w) => (
              <li key={w.word}>
                <button
                  type="button"
                  className={`gp-rank-row ${activeFilter === w.word ? 'active' : ''}`}
                  onClick={() => applyFilter(w.word, 'word')}
                  title={`Filter logs for “${w.word}”`}
                >
                  <span className="gp-rank mono">{w.rank}</span>
                  <span className="gp-rank-label mono" title={w.word}>{w.word}</span>
                  <span className="gp-rank-count mono">{fmtCount(w.count)}</span>
                  <span className="gp-rank-bar" aria-hidden="true">
                    <span style={{ width: `${Math.round(((w.score || w.count) / maxWord) * 100)}%` }} />
                  </span>
                </button>
              </li>
            ))}
            {!words.length && <li className="gp-empty muted">No tokens</li>}
          </ol>
        </section>

        <section className="gp-card">
          <header className="gp-card-h">
            Top fields
          </header>
          <ol className="gp-rank-list">
            {attributes.map((a) => (
              <li key={a.key}>
                <button
                  type="button"
                  className={`gp-rank-row ${activeFilter === a.key ? 'active' : ''}`}
                  onClick={() => applyFilter(a.key, 'field')}
                  title={`Filter logs for “${a.key}”`}
                >
                  <span className="gp-rank mono">{a.rank}</span>
                  <span className="gp-rank-label mono" title={a.key}>{a.key}</span>
                  <span className="gp-rank-count mono">{fmtCount(a.count)}</span>
                  <span className="gp-rank-bar gp-rank-bar-attr" aria-hidden="true">
                    <span style={{ width: `${Math.round(((a.score || a.count) / maxAttr) * 100)}%` }} />
                  </span>
                </button>
              </li>
            ))}
            {!attributes.length && (
              <li className="gp-empty muted">No key=value fields in this window</li>
            )}
          </ol>
        </section>

        <section className="gp-card gp-card-patterns">
          <header className="gp-card-h">
            Log Patterns
            <span className="gp-card-sub muted">
              {window.patternCount || templates.length} pattern
              {(window.patternCount || templates.length) === 1 ? '' : 's'}
              {' from '}
              {fmtCount(window.lineCount)} logs
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
                    title="Expand matching log lines"
                  >
                    <span className={`gp-pattern-bar tone-${barTone(i, templates.length)}`} aria-hidden="true">
                      <span style={{ width: `${barPct}%` }} />
                    </span>
                    <span className="gp-pattern-pct mono">{Number(t.pct || 0).toFixed(1)}%</span>
                    <span className="gp-pattern-tpl mono" title={t.template}>{t.template}</span>
                  </button>
                  {open && (
                    <div className="gp-pattern-expand">
                      {!matchedLogs.length && (
                        <div className="gp-empty muted">No matching live log lines in the current window.</div>
                      )}
                      {matchedLogs.map((line) => (
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
          </ul>
        </section>

        <section className="gp-card gp-card-counts">
          <header className="gp-card-h">Log Counts</header>
          <SeverityCountsBody
            severity={severity}
            label="Log severity breakdown"
          />
          <div className="gp-counts-foot muted mono">
            TOTAL: {sevTotal}
          </div>
        </section>
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
