import { useId, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { schedulingTableForRow } from '../../lib/schedulingTable.js'
import { tableCellValue } from '../../lib/entityTable.js'

function SchedulingPopover({ table, tipId, coords, above }) {
  return createPortal(
    <div
      className={[
        'entity-table-scheduling-popover',
        above ? 'is-above' : '',
      ].filter(Boolean).join(' ')}
      role="tooltip"
      id={tipId}
      style={{ left: coords.x, top: coords.y }}
    >
      <div className="entity-table-scheduling-popover-title">
        {table.title} {table.rows.length}
      </div>
      <div className="entity-table-scheduling-popover-scroll inspect-table-wrap">
        <table className="inspect-table">
          <thead>
            <tr>
              {table.columns.map((col) => (
                <th key={col}>{col}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {table.rows.map((row, index) => (
              <tr key={index}>
                {row.map((cell, cellIndex) => (
                  <td key={cellIndex} className="mono" title={String(cell || '')}>
                    {cell || ''}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>,
    document.body,
  )
}

export function SchedulingTableCell({ row, columnId }) {
  const table = schedulingTableForRow(row, columnId)
  const fallback = tableCellValue(row, columnId)
  const [open, setOpen] = useState(false)
  const [coords, setCoords] = useState({ x: 0, y: 0, above: false })
  const triggerRef = useRef(null)
  const tipId = useId()

  if (!table) {
    if (!fallback || fallback === '—') return <span>—</span>
    return <span className="mono entity-table-scheduling-detail" title={fallback}>{fallback}</span>
  }

  function showPopover() {
    const rect = triggerRef.current?.getBoundingClientRect()
    if (!rect) return
    const popoverWidth = Math.min(640, window.innerWidth - 24)
    const spaceBelow = window.innerHeight - rect.bottom
    const above = spaceBelow < 240 && rect.top > spaceBelow
    const x = Math.max(12, Math.min(rect.left, window.innerWidth - popoverWidth - 12))
    setCoords({
      x,
      y: above ? rect.top - 8 : rect.bottom + 8,
      above,
    })
    setOpen(true)
  }

  const label = table.summary

  return (
    <span
      ref={triggerRef}
      className="entity-table-scheduling-trigger"
      onMouseEnter={showPopover}
      onMouseLeave={() => setOpen(false)}
      onFocus={showPopover}
      onBlur={() => setOpen(false)}
      onClick={(e) => e.stopPropagation()}
      tabIndex={0}
      aria-describedby={open ? tipId : undefined}
    >
      <span className="entity-table-scheduling-count mono">{label}</span>
      {open && <SchedulingPopover table={table} tipId={tipId} coords={coords} above={coords.above} />}
    </span>
  )
}
