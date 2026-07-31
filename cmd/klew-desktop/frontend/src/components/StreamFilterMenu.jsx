import { useEffect, useRef } from 'react'

export function StreamFilterMenu({
  open,
  onToggle,
  active,
  disabled = false,
  selectedPods = [],
  matchedPods = [],
  otherPods = [],
  onTogglePod,
  onSelectMatched,
  onSelectAllPods,
  onReset,
}) {
  const rootRef = useRef(null)
  const selected = Array.isArray(selectedPods) ? selectedPods : []
  const matched = Array.isArray(matchedPods) ? matchedPods : []
  const other = Array.isArray(otherPods) ? otherPods : []

  useEffect(() => {
    if (!open) {
      return undefined
    }
    function onDocClick(e) {
      if (rootRef.current && !rootRef.current.contains(e.target)) {
        onToggle(false)
      }
    }
    function onKeyDown(e) {
      if (e.key === 'Escape') {
        onToggle(false)
      }
    }
    document.addEventListener('mousedown', onDocClick)
    document.addEventListener('keydown', onKeyDown)
    return () => {
      document.removeEventListener('mousedown', onDocClick)
      document.removeEventListener('keydown', onKeyDown)
    }
  }, [open, onToggle])

  return (
    <div className="stream-filter-anchor" ref={rootRef}>
      <button
        type="button"
        className={`stream-filter-btn ${active || open ? 'active' : ''}`}
        aria-label="Live tail filters"
        aria-expanded={open}
        disabled={disabled}
        title={disabled ? 'Start an investigation to filter the live tail' : 'Filter live logs and pods'}
        onClick={() => !disabled && onToggle(!open)}
      >
        <FilterIcon />
      </button>

      {open && (
        <div className="stream-filter-popover" role="dialog" aria-label="Live tail filters">
          <p className="stream-filter-intro">
            Logs only — by default every tailed pod is shown.
            Narrow the list here, or type in the search box. Templates are on the Patterns page.
          </p>

          <div className="stream-filter-section">
            <div className="stream-filter-label">
              Pods
              <span className="stream-filter-count">{matched.length + other.length}</span>
            </div>
            <div className="stream-filter-actions-inline">
              <button type="button" className="stream-filter-link" onClick={() => onSelectAllPods?.()}>
                Show all pods
              </button>
              <button type="button" className="stream-filter-link" onClick={() => onSelectMatched?.(matched)}>
                Only matched ({matched.length})
              </button>
            </div>

            {matched.length > 0 && (
              <>
                <div className="stream-filter-sublabel">Matched by search</div>
                <div className="stream-filter-pod-list">
                  {matched.map((name) => (
                    <label key={name} className="stream-filter-option">
                      <input
                        type="checkbox"
                        checked={selected.length === 0 || selected.includes(name)}
                        onChange={() => onTogglePod?.(name)}
                      />
                      <span className="mono stream-pod-name" title={name}>{name}</span>
                    </label>
                  ))}
                </div>
              </>
            )}

            {other.length > 0 && (
              <>
                <div className="stream-filter-sublabel">Other pods (optional)</div>
                <div className="stream-filter-pod-list">
                  {other.map((name) => (
                    <label key={name} className="stream-filter-option">
                      <input
                        type="checkbox"
                        checked={selected.length === 0 || selected.includes(name)}
                        onChange={() => onTogglePod?.(name)}
                      />
                      <span className="mono stream-pod-name" title={name}>{name}</span>
                    </label>
                  ))}
                </div>
              </>
            )}

            {!matched.length && !other.length && (
              <p className="stream-filter-hint">No pods in the investigation snapshot yet.</p>
            )}
          </div>

          <div className="stream-filter-actions">
            <button type="button" className="stream-filter-reset" onClick={onReset}>
              Clear filters (all pods)
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

function FilterIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden="true">
      <path d="M4 6h16M7 12h10M10 18h4" />
    </svg>
  )
}
