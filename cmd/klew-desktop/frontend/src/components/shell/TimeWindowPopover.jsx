import { useEffect, useRef, useState } from 'react'
import { WINDOW_MIN_OPTIONS } from '../../lib/preferences.js'

export function TimeWindowPopover({
  live,
  windowMin = 15,
  autoRefresh,
  running,
  onWindowChange,
  onAutoRefreshChange,
}) {
  const [open, setOpen] = useState(false)
  const rootRef = useRef(null)
  const options = WINDOW_MIN_OPTIONS || [5, 15, 30, 60]

  useEffect(() => {
    if (!open) return
    function onDoc(e) {
      if (rootRef.current && !rootRef.current.contains(e.target)) setOpen(false)
    }
    function onKey(e) {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', onDoc)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDoc)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  const label = windowMin >= 60 ? '1h' : `${windowMin}m`

  return (
    <div className="shell-popover-anchor" ref={rootRef}>
      <button
        type="button"
        className="shell-time-btn"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        aria-haspopup="listbox"
        title="Investigation time window"
        disabled={!running}
      >
        {live && autoRefresh !== false && <span className="shell-live-dot" aria-hidden="true">◉</span>}
        <span>{live && autoRefresh !== false ? 'Live' : 'Window'} · {label}</span>
      </button>
      {open && running && (
        <div className="shell-popover shell-time-popover" role="listbox" aria-label="Time window">
          <label className="shell-popover-check-row shell-popover-live-row">
            <input
              type="checkbox"
              checked={autoRefresh !== false}
              onChange={(e) => onAutoRefreshChange?.(e.target.checked)}
            />
            <span>Live refresh</span>
          </label>
          <div className="shell-popover-divider" />
          {options.map((m) => (
            <button
              key={m}
              type="button"
              role="option"
              aria-selected={windowMin === m}
              className={`shell-popover-item ${windowMin === m ? 'active' : ''}`}
              onClick={() => {
                onWindowChange?.(m)
                setOpen(false)
              }}
            >
              {windowMin === m && <span className="shell-popover-check" aria-hidden="true">✓</span>}
              <span>{m >= 60 ? '1 hour' : `${m} minutes`}</span>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
