import { useEffect, useId, useRef, useState } from 'react'
import { createPortal } from 'react-dom'

const SHOW_MS = 280
const HIDE_MS = 80

/**
 * Instant, styled hover tip. Prefer this over native `title` for chrome
 * controls so users see a clear explanation without the OS tooltip delay/overlap.
 *
 * Wrap a control: <HoverTip label="…" detail="…">…button…</HoverTip>
 */
export function HoverTip({
  label = '',
  detail = '',
  side = 'bottom',
  disabled = false,
  className = '',
  children,
}) {
  const [open, setOpen] = useState(false)
  const [coords, setCoords] = useState({ top: 0, left: 0 })
  const anchorRef = useRef(null)
  const showTimer = useRef(null)
  const hideTimer = useRef(null)
  const tipId = useId()
  const text = String(label || '').trim()
  const sub = String(detail || '').trim()

  function clearTimers() {
    if (showTimer.current) {
      clearTimeout(showTimer.current)
      showTimer.current = null
    }
    if (hideTimer.current) {
      clearTimeout(hideTimer.current)
      hideTimer.current = null
    }
  }

  function place() {
    const el = anchorRef.current
    if (!el) return
    const r = el.getBoundingClientRect()
    const gap = 8
    if (side === 'top') {
      setCoords({ top: Math.round(r.top - gap), left: Math.round(r.left + r.width / 2), place: 'top' })
    } else if (side === 'left') {
      setCoords({ top: Math.round(r.top + r.height / 2), left: Math.round(r.left - gap), place: 'left' })
    } else if (side === 'right') {
      setCoords({ top: Math.round(r.top + r.height / 2), left: Math.round(r.right + gap), place: 'right' })
    } else {
      setCoords({ top: Math.round(r.bottom + gap), left: Math.round(r.left + r.width / 2), place: 'bottom' })
    }
  }

  function show() {
    if (disabled || !text) return
    clearTimers()
    showTimer.current = setTimeout(() => {
      place()
      setOpen(true)
    }, SHOW_MS)
  }

  function hide() {
    clearTimers()
    hideTimer.current = setTimeout(() => setOpen(false), HIDE_MS)
  }

  useEffect(() => () => clearTimers(), [])

  useEffect(() => {
    if (!open) return undefined
    function onScroll() { setOpen(false) }
    window.addEventListener('scroll', onScroll, true)
    window.addEventListener('resize', onScroll)
    return () => {
      window.removeEventListener('scroll', onScroll, true)
      window.removeEventListener('resize', onScroll)
    }
  }, [open])

  if (!text) return children

  return (
    <span
      ref={anchorRef}
      className={['hover-tip-anchor', className].filter(Boolean).join(' ')}
      onMouseEnter={show}
      onMouseLeave={hide}
      onFocus={show}
      onBlur={hide}
      aria-describedby={open ? tipId : undefined}
    >
      {children}
      {open && createPortal(
        <span
          id={tipId}
          role="tooltip"
          className={`hover-tip hover-tip-${coords.place || side}`}
          style={{ top: coords.top, left: coords.left }}
        >
          <span className="hover-tip-label">{text}</span>
          {sub ? <span className="hover-tip-detail">{sub}</span> : null}
        </span>,
        document.body,
      )}
    </span>
  )
}
