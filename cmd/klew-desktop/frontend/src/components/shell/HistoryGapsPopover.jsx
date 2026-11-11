import { useEffect, useId, useRef, useState } from 'react'
import { gapRows } from '../../lib/eventHistory'

/**
 * Hover/click popover listing history gaps with their reasons.
 * Pass `header` for a structured top row (status + toggle); `title` stays for
 * simple one-line headings (CoverageLine).
 */
export function HistoryGapsPopover({
  history,
  trigger,
  title,
  header = null,
  note = 'History covers only the time Klew was recording.',
  emptyText = 'No gaps in recorded history.',
  triggerTitle = '',
  className = '',
  align = 'start',
}) {
  const rows = gapRows(history)
  const [hover, setHover] = useState(false)
  const [pinned, setPinned] = useState(false)
  const ref = useRef(null)
  const panelId = useId()
  const open = hover || pinned

  useEffect(() => {
    if (!pinned) return undefined
    const onDoc = (e) => { if (ref.current && !ref.current.contains(e.target)) setPinned(false) }
    const onKey = (e) => { if (e.key === 'Escape') setPinned(false) }
    document.addEventListener('mousedown', onDoc)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDoc)
      document.removeEventListener('keydown', onKey)
    }
  }, [pinned])

  return (
    <span
      ref={ref}
      className={['history-gaps', className].filter(Boolean).join(' ')}
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
    >
      <button
        type="button"
        className="history-gaps-trigger"
        aria-expanded={open}
        aria-controls={panelId}
        title={triggerTitle || undefined}
        onClick={() => setPinned((v) => !v)}
        onFocus={() => setHover(true)}
        onBlur={() => setHover(false)}
      >
        {trigger}
      </button>
      {open && (
        <span id={panelId} role="dialog" className={`history-gaps-panel align-${align}`}>
          {header || (title ? <span className="history-gaps-title">{title}</span> : null)}
          {rows.length === 0 ? (
            <span className="history-gaps-empty">{emptyText}</span>
          ) : (
            <span className="history-gaps-list">
              {rows.map((g) => (
                <span key={g.key} className="history-gaps-row">
                  <span className="history-gaps-range mono">{g.range}</span>
                  <span className="history-gaps-reason">{g.label}</span>
                </span>
              ))}
            </span>
          )}
          {note ? <span className="history-gaps-note">{note}</span> : null}
        </span>
      )}
    </span>
  )
}

/** Compact coverage line; the "· N gaps" part opens the gap list. */
export function CoverageLine({ history, text, className = '' }) {
  if (!text) return null
  const hasGaps = gapRows(history).length > 0
  return (
    <p className={className}>
      {hasGaps ? (
        <HistoryGapsPopover history={history} trigger={text} title="Gaps in recorded history" />
      ) : (
        text
      )}
    </p>
  )
}
