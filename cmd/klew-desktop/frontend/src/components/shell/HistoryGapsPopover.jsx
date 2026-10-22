import { useEffect, useId, useRef, useState } from 'react'
import { gapRows } from '../../lib/eventHistory'

/**
 * Hover/click popover listing history gaps with their reasons.
 * `trigger` is the visible content (text or a dot); the list shows on hover,
 * focus, or click (click pins it until Escape or an outside click).
 */
export function HistoryGapsPopover({ history, trigger, title, className = '', align = 'start' }) {
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
        onClick={() => setPinned((v) => !v)}
        onFocus={() => setHover(true)}
        onBlur={() => setHover(false)}
      >
        {trigger}
      </button>
      {open && (
        <span id={panelId} role="dialog" className={`history-gaps-panel align-${align}`}>
          {title && <span className="history-gaps-title">{title}</span>}
          {rows.length === 0 ? (
            <span className="history-gaps-empty">No gaps in recorded history.</span>
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
          <span className="history-gaps-note">
            History covers only the time Klew was recording. A fresh start recovers
            events the cluster still holds (about one event TTL).
          </span>
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
