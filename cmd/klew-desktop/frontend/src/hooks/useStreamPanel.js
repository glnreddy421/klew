import { useCallback, useEffect, useRef, useState } from 'react'
import { StreamMode } from '../lib/streamView'

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
  const [mode, setMode] = useState(StreamMode.Logs)
  const [search, setSearch] = useState('')
  /** Explicit pod subset. Empty = show all tailed pods (no pod filter). */
  const [selectedPods, setSelectedPods] = useState([])
  /** When true, user chose a pod subset; empty selection stays "all pods". */
  const [podFilterPinned, setPodFilterPinned] = useState(false)
  const [follow, setFollow] = useState(true)
  /** When true, Live tail freezes the visible lines until resume. */
  const [paused, setPaused] = useState(false)
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

  const togglePod = useCallback((name, allPods = []) => {
    setPodFilterPinned(true)
    setSelectedPods((prev) => {
      const all = (Array.isArray(allPods) ? allPods : []).map((p) => String(p || '').trim()).filter(Boolean)
      // Empty = all pods. Unchecking one restricts to every other listed pod.
      if (!prev.length) {
        if (!all.length) return [name]
        return all.filter((p) => p !== name)
      }
      if (prev.includes(name)) {
        return prev.filter((p) => p !== name)
      }
      const next = [...prev, name]
      if (all.length && next.length === all.length && all.every((p) => next.includes(p))) {
        return []
      }
      return next
    })
  }, [])

  const selectPods = useCallback((names, { pinned = true } = {}) => {
    setSelectedPods(Array.isArray(names) ? [...names] : [])
    setPodFilterPinned(pinned)
  }, [])

  const selectMatchedPods = useCallback((matched) => {
    const list = Array.isArray(matched) ? [...matched] : []
    setSelectedPods(list)
    setPodFilterPinned(list.length > 0)
  }, [])

  const selectAllPods = useCallback(() => {
    // Empty selection = all pods (no filter).
    setSelectedPods([])
    setPodFilterPinned(false)
  }, [])

  const resetFilters = useCallback(() => {
    setMode(StreamMode.Logs)
    setSelectedPods([])
    setPodFilterPinned(false)
    setSearch('')
    setPaused(false)
  }, [])

  const togglePaused = useCallback(() => {
    setPaused((p) => {
      const next = !p
      if (next) setFollow(false)
      else setFollow(true)
      return next
    })
  }, [])

  return {
    panelState,
    setPanelState,
    height,
    setHeight,
    mode,
    setMode,
    search,
    setSearch,
    selectedPods,
    setSelectedPods,
    podFilterPinned,
    togglePod,
    selectPods,
    selectMatchedPods,
    selectAllPods,
    follow,
    setFollow,
    paused,
    setPaused,
    togglePaused,
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
