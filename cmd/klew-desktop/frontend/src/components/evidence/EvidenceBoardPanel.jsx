import { useState } from 'react'

/**
 * Correlated signals — infra event patterns linked to log patterns by
 * minute-overlap similarity (≥ threshold from backend).
 */
export function EvidenceBoardPanel({ board, onFilterLogs, compactEmpty = false }) {
  const cards = board?.cards || []
  const thresholdPct = Math.round((board?.threshold ?? 0.6) * 100)

  return (
    <section className="ev-chains" aria-labelledby="ev-chains-title">
      <header className="ev-chains-head">
        <div>
          <h3 id="ev-chains-title">Correlated signals</h3>
          <p className="ev-chains-sub muted">
            Infrastructure events linked to log templates by minute overlap
          </p>
        </div>
        <div className="ev-chains-stats">
          <span className="ev-stat">
            <strong>{cards.length}</strong>
            <em>linked</em>
          </span>
          <span className="ev-stat muted">≥{thresholdPct}%</span>
        </div>
      </header>

      {!cards.length ? (
        <div className={`ev-chains-empty ${compactEmpty ? 'compact' : ''}`}>
          <div className="ev-chains-empty-viz" aria-hidden="true">
            <span className="ev-slot on" />
            <span className="ev-slot" />
            <span className="ev-slot on" />
            <span className="ev-slot on" />
            <span className="ev-slot" />
            <span className="ev-slot on" />
            <span className="ev-empty-op">overlap</span>
            <span className="ev-slot on" />
            <span className="ev-slot" />
            <span className="ev-slot on" />
            <span className="ev-slot" />
            <span className="ev-slot on" />
            <span className="ev-slot on" />
          </div>
          <p>
            No linked patterns yet. When an event template and a log template share active minutes
            (≥{thresholdPct}% overlap), they appear here.
          </p>
        </div>
      ) : (
        <ul className="ev-chains-list">
          {cards.map((card) => (
            <CorrelatedSignal
              key={card.evidenceId || card.rootEvent?.id}
              card={card}
              onFilterLogs={onFilterLogs}
            />
          ))}
        </ul>
      )}
    </section>
  )
}

function CorrelatedSignal({ card, onFilterLogs }) {
  const [open, setOpen] = useState(true)
  const root = card.rootEvent || {}
  const logs = card.triggeredLogs || []
  const confPct = Math.round((card.confidence || 0) * 100)
  const tone = confPct >= 80 ? 'high' : confPct >= 60 ? 'mid' : 'low'

  return (
    <li className={`ev-signal tone-${tone}`}>
      <button
        type="button"
        className="ev-signal-main"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
      >
        <span className={`ev-signal-score tone-${tone}`} title="Time overlap">
          {confPct}
          <small>%</small>
        </span>
        <div className="ev-signal-flow">
          <div className="ev-signal-node">
            <span className="ev-tag event">Event</span>
            <span className="mono ev-tpl" title={root.template}>{root.template || '—'}</span>
            <MiniSpark values={volumeOf(root)} />
          </div>
          <span className="ev-signal-arrow" aria-hidden="true">→</span>
          <div className="ev-signal-node">
            <span className="ev-tag log">Log ×{logs.length}</span>
            <span className="mono ev-tpl" title={logs[0]?.template}>
              {logs[0]?.template || '—'}
              {logs.length > 1 ? ` +${logs.length - 1}` : ''}
            </span>
            <MiniSpark values={volumeOf(logs[0])} />
          </div>
        </div>
        <span className="ev-signal-chevron" aria-hidden="true">{open ? '▾' : '▸'}</span>
      </button>

      {open && (
        <div className="ev-signal-detail">
          <div className="ev-overlap-row" title="Active minutes (event vs first log)">
            <OverlapBars a={volumeOf(root)} b={volumeOf(logs[0])} />
            <span className="muted">minute overlap</span>
          </div>
          {(root.pods || []).length > 0 && (
            <p className="ev-signal-pods muted">
              {(root.pods || []).slice(0, 8).join(' · ')}
              {(root.pods || []).length > 8 ? ` +${root.pods.length - 8}` : ''}
            </p>
          )}
          <ul className="ev-signal-logs">
            {logs.map((log) => (
              <li key={log.id || log.template}>
                <button
                  type="button"
                  className="ev-log-row"
                  title="Filter Live tail"
                  disabled={!onFilterLogs}
                  onClick={() => {
                    const term = pickFilterTerm(log)
                    if (term && onFilterLogs) onFilterLogs(term)
                  }}
                >
                  <span className="ev-tag log">Log</span>
                  <span className="mono ev-tpl" title={log.template}>{log.template || '—'}</span>
                  <MiniSpark values={volumeOf(log)} />
                  <span className="muted">{log.count ?? 0}×</span>
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}
    </li>
  )
}

function OverlapBars({ a, b }) {
  const n = Math.max(a?.length || 0, b?.length || 0, 1)
  const cells = []
  for (let i = 0; i < n; i++) {
    const aa = (a?.[i] || 0) > 0
    const bb = (b?.[i] || 0) > 0
    let cls = 'none'
    if (aa && bb) cls = 'both'
    else if (aa) cls = 'a'
    else if (bb) cls = 'b'
    cells.push(cls)
  }
  return (
    <span className="ev-overlap" aria-hidden="true">
      {cells.map((cls, i) => (
        <span key={i} className={`ev-overlap-cell ${cls}`} />
      ))}
    </span>
  )
}

function volumeOf(tpl) {
  if (Array.isArray(tpl?.volumeHistory) && tpl.volumeHistory.length) return tpl.volumeHistory
  if (Array.isArray(tpl?.sparkline) && tpl.sparkline.length) return tpl.sparkline
  return []
}

function pickFilterTerm(log) {
  const kw = log?.keywords?.[0]?.word
  if (kw) return kw
  const tpl = String(log?.template || '')
    .replace(/<\*>/g, ' ')
    .replace(/<[A-Za-z0-9_]+>/g, ' ')
    .replace(/[^\w./:-]+/g, ' ')
    .trim()
  const parts = tpl.split(/\s+/).filter((w) => w.length >= 4)
  return parts[0] || tpl.slice(0, 40) || ''
}

function MiniSpark({ values }) {
  const v = Array.isArray(values) ? values : []
  if (!v.length) return <span className="mini-spark empty" aria-hidden="true" />
  const max = Math.max(1, ...v.map((n) => Number(n) || 0))
  return (
    <span className="mini-spark" title="Per-minute volume" aria-hidden="true">
      {v.map((n, i) => {
        const num = Number(n) || 0
        const h = Math.max(2, Math.round((num / max) * 14))
        return (
          <span
            key={i}
            className={`mini-spark-bar ${num > 0 ? 'on' : ''}`}
            style={{ height: `${h}px` }}
          />
        )
      })}
    </span>
  )
}

/** Compact teaser for Patterns → Evidence. */
export function EvidenceBoardTeaser({ board, onOpen }) {
  const n = board?.cardCount ?? board?.cards?.length ?? 0
  if (n < 1 || !onOpen) return null
  return (
    <button type="button" className="evidence-board-teaser" onClick={onOpen}>
      <span>{n} correlated signal{n === 1 ? '' : 's'}</span>
      <span className="muted">View on Evidence →</span>
    </button>
  )
}
