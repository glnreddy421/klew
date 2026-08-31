import { useCallback, useEffect, useRef, useState } from 'react'

export const TERMINAL_CLOSED = 'closed'
export const TERMINAL_MINIMIZED = 'minimized'
export const TERMINAL_NORMAL = 'normal'
export const TERMINAL_MAXIMIZED = 'maximized'

const DEFAULT_HEIGHT = 280
const MIN_HEIGHT = 140

export function useTerminalPanel() {
  const [open, setOpen] = useState(false)
  const [panelState, setPanelState] = useState(TERMINAL_CLOSED)
  const [height, setHeight] = useState(DEFAULT_HEIGHT)
  const dragRef = useRef(null)

  const maxHeight = () => Math.max(MIN_HEIGHT, Math.floor(window.innerHeight * 0.72))

  const openPanel = useCallback(() => {
    setOpen(true)
    setPanelState(TERMINAL_NORMAL)
  }, [])

  const close = useCallback(() => {
    setOpen(false)
    setPanelState(TERMINAL_CLOSED)
  }, [])

  const toggle = useCallback(() => {
    setOpen((v) => {
      const next = !v
      setPanelState(next ? TERMINAL_NORMAL : TERMINAL_CLOSED)
      return next
    })
  }, [])

  const minimize = useCallback(() => setPanelState(TERMINAL_MINIMIZED), [])
  const maximize = useCallback(() => setPanelState(TERMINAL_MAXIMIZED), [])
  const restore = useCallback(() => setPanelState(TERMINAL_NORMAL), [])

  const startResize = useCallback((e) => {
    if (panelState !== TERMINAL_NORMAL) return
    e.preventDefault()
    const startY = e.clientY
    const startH = height
    dragRef.current = { startY, startH }

    function onMove(ev) {
      if (!dragRef.current) return
      const delta = dragRef.current.startY - ev.clientY
      const next = Math.min(maxHeight(), Math.max(MIN_HEIGHT, dragRef.current.startH + delta))
      setHeight(next)
    }

    function onUp() {
      dragRef.current = null
      window.removeEventListener('mousemove', onMove)
      window.removeEventListener('mouseup', onUp)
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
    }

    document.body.style.cursor = 'row-resize'
    document.body.style.userSelect = 'none'
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', onUp)
  }, [height, panelState])

  useEffect(() => {
    function onResize() {
      setHeight((h) => Math.min(h, maxHeight()))
    }
    window.addEventListener('resize', onResize)
    return () => window.removeEventListener('resize', onResize)
  }, [])

  return {
    open,
    panelState,
    height,
    openPanel,
    toggle,
    close,
    minimize,
    maximize,
    restore,
    startResize,
  }
}
