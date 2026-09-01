import { useCallback, useEffect, useRef, useState } from 'react'
import { PANEL_NORMAL } from './useStreamPanel'

export const DOCK_VIEW_TERMINAL = 'terminal'
export const DOCK_VIEW_STREAM = 'stream'
export const DOCK_VIEW_SPLIT = 'split'

const MIN_HEIGHT = 120
const DEFAULT_HEIGHT_RATIO = 0.42
const MAX_HEIGHT_RATIO = 0.85

function defaultDockHeight() {
  return Math.max(MIN_HEIGHT, Math.floor(window.innerHeight * DEFAULT_HEIGHT_RATIO))
}

function maxDockHeight() {
  return Math.max(MIN_HEIGHT, Math.floor(window.innerHeight * MAX_HEIGHT_RATIO))
}

export function useConsoleDock({ stream, terminal }) {
  const [activeView, setActiveView] = useState(DOCK_VIEW_STREAM)
  const [expanded, setExpanded] = useState(false)
  const [height, setHeight] = useState(defaultDockHeight)
  const [maximized, setMaximized] = useState(false)
  const dragRef = useRef(null)

  const openTerminal = useCallback(() => {
    terminal.openPanel()
    setActiveView(DOCK_VIEW_TERMINAL)
    setExpanded(true)
    setMaximized(false)
  }, [terminal])

  const openStream = useCallback(() => {
    stream.openPanel(PANEL_NORMAL)
    setActiveView(DOCK_VIEW_STREAM)
    setExpanded(true)
    setMaximized(false)
  }, [stream])

  const openSplit = useCallback(() => {
    terminal.openPanel()
    stream.openPanel(PANEL_NORMAL)
    setActiveView(DOCK_VIEW_SPLIT)
    setExpanded(true)
    setMaximized(false)
  }, [stream, terminal])

  const selectView = useCallback((view) => {
    if (view === DOCK_VIEW_TERMINAL) {
      openTerminal()
      return
    }
    if (view === DOCK_VIEW_STREAM) {
      openStream()
      return
    }
    openSplit()
  }, [openTerminal, openStream, openSplit])

  const collapse = useCallback(() => {
    setExpanded(false)
    setMaximized(false)
  }, [])

  const closeAll = useCallback(() => {
    setExpanded(false)
    setMaximized(false)
    terminal.close()
    stream.close()
  }, [stream, terminal])

  const minimize = collapse

  const maximize = useCallback(() => setMaximized(true), [])
  const restore = useCallback(() => setMaximized(false), [])

  const startResize = useCallback((e) => {
    if (!expanded || maximized) return
    e.preventDefault()
    const startY = e.clientY
    const startH = height
    dragRef.current = { startY, startH }

    function onMove(ev) {
      if (!dragRef.current) return
      const delta = dragRef.current.startY - ev.clientY
      const next = Math.min(maxDockHeight(), Math.max(MIN_HEIGHT, dragRef.current.startH + delta))
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
  }, [expanded, maximized, height])

  useEffect(() => {
    function onResize() {
      setHeight((h) => Math.min(h, maxDockHeight()))
    }
    window.addEventListener('resize', onResize)
    return () => window.removeEventListener('resize', onResize)
  }, [])

  return {
    activeView,
    expanded,
    height,
    maximized,
    openTerminal,
    openStream,
    openSplit,
    selectView,
    collapse,
    closeAll,
    minimize,
    maximize,
    restore,
    startResize,
  }
}
