import { useCallback, useEffect, useRef, useState } from 'react'

export const TERMINAL_CLOSED = 'closed'
export const TERMINAL_MINIMIZED = 'minimized'
export const TERMINAL_NORMAL = 'normal'
export const TERMINAL_MAXIMIZED = 'maximized'

const MIN_HEIGHT = 140
const DEFAULT_HEIGHT_RATIO = 0.5
const MAX_HEIGHT_RATIO = 0.85

function defaultTerminalHeight() {
  return Math.max(MIN_HEIGHT, Math.floor(window.innerHeight * DEFAULT_HEIGHT_RATIO))
}

function maxTerminalHeight() {
  return Math.max(MIN_HEIGHT, Math.floor(window.innerHeight * MAX_HEIGHT_RATIO))
}

export function useTerminalPanel({ defaultOpen = false } = {}) {
  const [open, setOpen] = useState(defaultOpen)
  const [panelState, setPanelState] = useState(defaultOpen ? TERMINAL_NORMAL : TERMINAL_CLOSED)
  const [height, setHeight] = useState(defaultTerminalHeight)
  const dragRef = useRef(null)

  const maxHeight = maxTerminalHeight

  const openPanel = useCallback(() => {
    setOpen(true)
    setPanelState(TERMINAL_NORMAL)
    setHeight((h) => (h > 0 ? h : defaultTerminalHeight()))
  }, [])

  const close = useCallback(() => {
    setOpen(false)
    setPanelState(TERMINAL_CLOSED)
  }, [])

  const toggle = useCallback(() => {
    setOpen((wasOpen) => {
      const next = !wasOpen
      if (next) {
        setPanelState(TERMINAL_NORMAL)
        setHeight((h) => (h > 0 ? h : defaultTerminalHeight()))
      } else {
        setPanelState(TERMINAL_CLOSED)
      }
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
