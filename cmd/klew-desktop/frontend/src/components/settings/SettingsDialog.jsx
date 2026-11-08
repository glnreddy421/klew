import { useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { SettingsView } from '../../views/SettingsView'

const CLOSE_MS = 140

/**
 * Settings as a centred dialog over the current page (backdrop, pop-in).
 * Closes on Esc, the close button or a click on the backdrop.
 */
export function SettingsDialog({ open, onClose, ...viewProps }) {
  const [mounted, setMounted] = useState(open)
  const [closing, setClosing] = useState(false)
  const dialogRef = useRef(null)
  const returnFocusRef = useRef(null)

  useEffect(() => {
    if (open) {
      returnFocusRef.current = document.activeElement
      setMounted(true)
      setClosing(false)
      return undefined
    }
    if (!mounted) return undefined
    setClosing(true)
    const t = setTimeout(() => {
      setMounted(false)
      setClosing(false)
      returnFocusRef.current?.focus?.()
    }, CLOSE_MS)
    return () => clearTimeout(t)
  }, [open])

  useEffect(() => {
    if (!open) return undefined
    dialogRef.current?.focus()
    function onKey(e) {
      if (e.key === 'Escape') {
        e.stopPropagation()
        onClose?.()
      }
    }
    window.addEventListener('keydown', onKey, true)
    return () => window.removeEventListener('keydown', onKey, true)
  }, [open, onClose])

  if (!mounted) return null

  return createPortal(
    <div
      className={`settings-dialog-backdrop ${closing ? 'is-closing' : ''}`}
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose?.()
      }}
    >
      <div
        ref={dialogRef}
        className={`settings-dialog ${closing ? 'is-closing' : ''}`}
        role="dialog"
        aria-modal="true"
        aria-label="Settings"
        tabIndex={-1}
      >
        <SettingsView {...viewProps} onClose={onClose} />
      </div>
    </div>,
    document.body,
  )
}
