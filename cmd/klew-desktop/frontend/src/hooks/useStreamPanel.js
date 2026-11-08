import { useCallback, useEffect, useRef, useState } from 'react'

export const PANEL_MINIMIZED = 'minimized'
export const PANEL_NORMAL = 'normal'
export const PANEL_MAXIMIZED = 'maximized'
export const PANEL_CLOSED = 'closed'

const DEFAULT_HEIGHT = 240
const MIN_HEIGHT = 120
const MAX_HEIGHT_RATIO = 0.82

export function useStreamPanel() {
  const [panelState, setPanelState] = useState(PANEL_MINIMIZED)
  const [height, setHeight] = useState(DEFAULT_HEIGHT)
  const [follow, setFollow] = useState(true)
  const dragRef = useRef(null)

  const maxHeight = () => Math.max(MIN_HEIGHT, Math.floor(window.innerHeight * MAX_HEIGHT_RATIO))

  const startResize = useCallback((e) => {
    if (panelState !== PANEL_NORMAL) {
      return
    }
    e.preventDefault()
    const startY = e.clientY
    const startH = height

    dragRef.current = { startY, startH }

    function onMove(ev) {
      if (!dragRef.current) {
        return
      }
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

  const openPanel = useCallback((state = PANEL_NORMAL) => {
    setPanelState(state)
  }, [])

  const minimize = useCallback(() => setPanelState(PANEL_MINIMIZED), [])
  const maximize = useCallback(() => setPanelState(PANEL_MAXIMIZED), [])
  const restore = useCallback(() => setPanelState(PANEL_NORMAL), [])
  const close = useCallback(() => setPanelState(PANEL_CLOSED), [])

  const toggleMaximize = useCallback(() => {
    setPanelState((s) => (s === PANEL_MAXIMIZED ? PANEL_NORMAL : PANEL_MAXIMIZED))
  }, [])

  const resetFilters = useCallback(() => {}, [])

  return {
    panelState,
    setPanelState,
    height,
    setHeight,
    follow,
    setFollow,
    startResize,
    openPanel,
    minimize,
    maximize,
    restore,
    close,
    toggleMaximize,
    resetFilters,
  }
}
