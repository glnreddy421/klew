import { useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { columnPickerOrder } from '../../lib/entityTableColumns.js'

export function EntityTableColumnPicker({
  columns = [],
  visibleIds = [],
  onChange,
}) {
  const [open, setOpen] = useState(false)
  const [coords, setCoords] = useState({ top: 0, left: 0 })
  const anchorRef = useRef(null)
  const panelRef = useRef(null)

  const menuColumns = useMemo(
    () => columnPickerOrder(columns, visibleIds),
    [columns, visibleIds],
  )
  const firstHiddenIdx = menuColumns.findIndex((col) => !visibleIds.includes(col.id))

  useEffect(() => {
    if (!open) return undefined
    function onDoc(e) {
      if (anchorRef.current?.contains(e.target)) return
      if (panelRef.current?.contains(e.target)) return
      setOpen(false)
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

  function toggleColumn(id, required) {
    if (required) return
    if (visibleIds.includes(id)) {
      if (visibleIds.length <= 1) return
      onChange?.(visibleIds.filter((colId) => colId !== id))
      return
    }
    onChange?.([...visibleIds, id])
  }

  function moveColumn(id, delta) {
    const idx = visibleIds.indexOf(id)
    if (idx < 0) return
    const next = idx + delta
    if (next < 0 || next >= visibleIds.length) return
    const reordered = [...visibleIds]
    ;[reordered[idx], reordered[next]] = [reordered[next], reordered[idx]]
    onChange?.(reordered)
  }

  function openMenu() {
    const rect = anchorRef.current?.getBoundingClientRect()
    if (rect) {
      const menuWidth = 220
      const left = Math.max(12, Math.min(rect.right - menuWidth, window.innerWidth - menuWidth - 12))
      setCoords({ top: rect.bottom + 6, left })
    }
    setOpen(true)
  }

  const menu = open ? (
    <div
      ref={panelRef}
      className="entity-table-columns-menu entity-table-columns-menu-fixed"
      role="menu"
      aria-label="Table columns"
      style={{ top: coords.top, left: coords.left }}
    >
      {menuColumns.map((col, index) => {
        const checked = visibleIds.includes(col.id)
        const required = Boolean(col.required)
        const visibleIndex = visibleIds.indexOf(col.id)
        const canMoveUp = checked && !required && visibleIndex > 0
        const canMoveDown = checked && !required && visibleIndex >= 0 && visibleIndex < visibleIds.length - 1
        const showSeparator = firstHiddenIdx === index && firstHiddenIdx > 0

        return (
          <div key={col.id}>
            {showSeparator && <div className="entity-table-columns-sep" aria-hidden="true" />}
            <div className={`entity-table-columns-row ${checked ? 'is-checked' : ''}`}>
              <button
                type="button"
                role="menuitemcheckbox"
                className="entity-table-columns-option"
                aria-checked={checked}
                disabled={required}
                onClick={() => toggleColumn(col.id, required)}
              >
                <span className={`entity-table-columns-check ${checked ? 'checked' : ''}`} aria-hidden="true">
                  {checked ? '✓' : ''}
                </span>
                <span>{col.label}</span>
              </button>
              {checked && !required && (
                <span className="entity-table-columns-move">
                  <button
                    type="button"
                    className="entity-table-columns-move-btn"
                    aria-label={`Move ${col.label} up`}
                    disabled={!canMoveUp}
                    onClick={() => moveColumn(col.id, -1)}
                  >
                    ↑
                  </button>
                  <button
                    type="button"
                    className="entity-table-columns-move-btn"
                    aria-label={`Move ${col.label} down`}
                    disabled={!canMoveDown}
                    onClick={() => moveColumn(col.id, 1)}
                  >
                    ↓
                  </button>
                </span>
              )}
            </div>
          </div>
        )
      })}
    </div>
  ) : null

  return (
    <div className="entity-table-column-picker" ref={anchorRef}>
      <button
        type="button"
        className={`explorer-collapse-btn entity-table-columns-btn ${open ? 'is-open' : ''}`}
        onClick={() => (open ? setOpen(false) : openMenu())}
        title="Choose and arrange columns"
        aria-label="Choose and arrange columns"
        aria-expanded={open}
        aria-haspopup="menu"
      >
        <ColumnsIcon />
      </button>
      {menu && createPortal(menu, document.body)}
    </div>
  )
}

function ColumnsIcon() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="currentColor" aria-hidden="true">
      <circle cx="8" cy="3.5" r="1.15" />
      <circle cx="8" cy="8" r="1.15" />
      <circle cx="8" cy="12.5" r="1.15" />
    </svg>
  )
}
