import { useCallback, useEffect, useRef, useState } from 'react'

let nextTabSeq = 1

export function createTerminalTab(overrides = {}) {
  const n = nextTabSeq++
  return {
    id: `term-tab-${n}`,
    title: overrides.title || `Shell ${n}`,
    shell: overrides.shell || '',
    sessionId: null,
    ready: false,
    error: '',
    contextName: overrides.contextName || '',
    namespace: overrides.namespace || '',
    initialInput: overrides.initialInput || '',
    initialInputSent: false,
  }
}

export function terminalContextKey(cluster) {
  const ctx = cluster?.selectedContext || cluster?.currentContext || ''
  const kube = cluster?.kubeconfigPath || ''
  return `${kube}\0${ctx}`
}

/** Drop live PTY fields so cached tabs restart cleanly when restored. */
export function reviveTerminalTabs(savedTabs) {
  if (!Array.isArray(savedTabs) || savedTabs.length === 0) return []
  const token = Date.now()
  return savedTabs.map((t) => ({
    ...t,
    sessionId: null,
    ready: false,
    error: '',
    shell: '',
    restartToken: token,
  }))
}

function pickActiveId(savedId, tabs) {
  if (savedId && tabs.some((t) => t.id === savedId)) return savedId
  return tabs[0]?.id || null
}

const STORAGE_KEY = 'klew.terminal.tabs'

function loadPersisted() {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY)
    const obj = raw ? JSON.parse(raw) : null
    if (!obj || !Array.isArray(obj.tabs)) return { tabs: [], activeId: null }
    return obj
  } catch {
    return { tabs: [], activeId: null }
  }
}

function persist(tabs, activeId) {
  try {
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify({ tabs: tabsForCache(tabs), activeId }))
  } catch {
    /* quota / private mode */
  }
}

/** Strip runtime-only fields before caching. */
export function tabsForCache(tabs) {
  return tabs.map((t) => ({
    id: t.id,
    title: t.title,
    contextName: t.contextName,
    namespace: t.namespace,
    initialInput: t.initialInput,
    initialInputSent: t.initialInputSent,
  }))
}

function restorePersisted() {
  const saved = loadPersisted()
  const tabs = reviveTerminalTabs(saved.tabs.map((t) => ({
    ...t,
    id: t.id || `term-tab-${nextTabSeq++}`,
    title: t.title || 'Shell',
  })))
  for (const t of tabs) {
    const n = Number(String(t.id).replace('term-tab-', ''))
    if (Number.isFinite(n) && n >= nextTabSeq) nextTabSeq = n + 1
  }
  return { tabs, activeId: pickActiveId(saved.activeId, tabs) }
}

/**
 * Terminal tabs for the dock. Each tab is pinned to the kube context that was
 * current when it opened; switching context leaves existing tabs (and their
 * shells) alone, and new tabs pick up the new context.
 */
export function useTerminalTabs(cluster, { open = false, persist: keepOnClose = false, onEmpty, skipInitialSeed = false } = {}) {
  const contextName = cluster?.selectedContext || cluster?.currentContext || ''
  // New shells follow the kube context's own namespace, not the browse selection.
  const namespace = ''

  const tabsRef = useRef([])
  const activeIdRef = useRef(null)
  const seededRef = useRef(false)

  const [tabs, setTabs] = useState([])
  const [activeId, setActiveId] = useState(null)

  tabsRef.current = tabs
  activeIdRef.current = activeId

  useEffect(() => {
    if (!open) return
    persist(tabs, activeId)
  }, [open, tabs, activeId])

  const addTab = useCallback((overrides = {}) => {
    // Build outside the state updater: updaters may run twice (StrictMode) and
    // createTerminalTab bumps a counter, which would fork the id.
    const tab = createTerminalTab({ contextName, namespace, ...overrides })
    setTabs((prev) => [...prev, tab])
    setActiveId(tab.id)
    return tab.id
  }, [contextName, namespace])

  const closeTab = useCallback((id) => {
    const prev = tabsRef.current
    const idx = prev.findIndex((t) => t.id === id)
    if (idx < 0) return
    const next = prev.filter((t) => t.id !== id)
    setTabs(next)
    setActiveId((cur) => {
      if (cur !== id) return cur
      if (next.length === 0) return null
      return next[Math.min(idx, next.length - 1)].id
    })
    if (next.length === 0) {
      seededRef.current = true
      persist([], null)
      onEmpty?.()
    }
  }, [onEmpty])

  /** Close several tabs in one pass — looping closeTab would read stale state. */
  const closeTabs = useCallback((ids = []) => {
    const drop = new Set(ids)
    if (drop.size === 0) return
    const prev = tabsRef.current
    const next = prev.filter((t) => !drop.has(t.id))
    if (next.length === prev.length) return
    setTabs(next)
    setActiveId((cur) => (cur && drop.has(cur) ? next[next.length - 1]?.id || null : cur))
    if (next.length === 0) {
      seededRef.current = true
      persist([], null)
      onEmpty?.()
    }
  }, [onEmpty])

  const updateTab = useCallback((id, patch) => {
    setTabs((prev) => prev.map((t) => (t.id === id ? { ...t, ...patch } : t)))
  }, [])

  const selectTab = useCallback((id) => {
    setActiveId(id)
  }, [])

  const restartTab = useCallback((id) => {
    updateTab(id, { shell: '', sessionId: null, ready: false, error: '', restartToken: Date.now() })
  }, [updateTab])

  const restartAllTabs = useCallback(() => {
    const token = Date.now()
    setTabs((prev) => prev.map((t) => ({
      ...t,
      shell: '',
      sessionId: null,
      ready: false,
      error: '',
      restartToken: token,
    })))
  }, [])

  useEffect(() => {
    if (!open) {
      if (!keepOnClose) {
        if (tabsRef.current.length > 0) persist(tabsRef.current, activeIdRef.current)
        setTabs([])
        setActiveId(null)
        seededRef.current = false
      }
      return
    }

    if (tabs.length > 0) return

    const restored = restorePersisted()
    if (restored.tabs.length > 0) {
      seededRef.current = true
      setTabs(restored.tabs)
      setActiveId(restored.activeId)
      return
    }

    if (!seededRef.current && !skipInitialSeed) {
      seededRef.current = true
      addTab()
    }
  }, [open, keepOnClose, skipInitialSeed, tabs.length, addTab])

  const activeTab = tabs.find((t) => t.id === activeId) || null

  return {
    tabs,
    activeId,
    activeTab,
    addTab,
    closeTab,
    closeTabs,
    updateTab,
    selectTab,
    restartTab,
    restartAllTabs,
    contextName,
    namespace,
    contextKey: terminalContextKey(cluster),
  }
}
