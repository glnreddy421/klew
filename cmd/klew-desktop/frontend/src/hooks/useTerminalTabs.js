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

const STORAGE_KEY = 'klew.terminal.tabsByContext'

function loadPersistedCache() {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY)
    if (!raw) return new Map()
    const obj = JSON.parse(raw)
    if (!obj || typeof obj !== 'object') return new Map()
    return new Map(Object.entries(obj))
  } catch {
    return new Map()
  }
}

function persistCache(map) {
  try {
    const obj = Object.fromEntries(map.entries())
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify(obj))
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

export function useTerminalTabs(cluster, { open = false, persist = false, onEmpty, skipInitialSeed = false } = {}) {
  const contextKey = terminalContextKey(cluster)
  const contextName = cluster?.selectedContext || cluster?.currentContext || ''
  const namespace = cluster?.selectedNamespace || ''

  const cacheRef = useRef(loadPersistedCache())
  const contextKeyRef = useRef(contextKey)
  const tabsRef = useRef([])
  const activeIdRef = useRef(null)
  const seededRef = useRef(false)

  const [tabs, setTabs] = useState([])
  const [activeId, setActiveId] = useState(null)

  tabsRef.current = tabs
  activeIdRef.current = activeId

  const writeCache = useCallback((key, tabList, active) => {
    if (!key || !String(key).split('\0')[1]) return
    cacheRef.current.set(key, {
      tabs: tabsForCache(tabList),
      activeId: active,
    })
    persistCache(cacheRef.current)
  }, [])

  const loadFromCache = useCallback((key) => {
    const cached = cacheRef.current.get(key)
    if (!cached?.tabs?.length) {
      return { tabs: [], activeId: null, hadCache: false }
    }
    const ctxFromKey = String(key).split('\0')[1] || ''
    const token = Date.now()
    const restored = cached.tabs.map((t) => ({
      id: t.id || `term-tab-${nextTabSeq++}`,
      title: t.title || 'Shell',
      shell: '',
      sessionId: null,
      ready: false,
      error: '',
      contextName: t.contextName || ctxFromKey,
      namespace: t.namespace || '',
      initialInput: t.initialInput || '',
      initialInputSent: Boolean(t.initialInputSent),
      restartToken: token,
    }))
    return {
      tabs: restored,
      activeId: pickActiveId(cached.activeId, restored),
      hadCache: true,
    }
  }, [])

  // Switch tab group when kube context changes.
  useEffect(() => {
    const prevKey = contextKeyRef.current
    if (prevKey === contextKey) return

    writeCache(prevKey, tabsRef.current, activeIdRef.current)

    const loaded = loadFromCache(contextKey)
    contextKeyRef.current = contextKey
    seededRef.current = loaded.hadCache
    setTabs(loaded.tabs)
    setActiveId(loaded.activeId)
  }, [contextKey, writeCache, loadFromCache])

  // Keep cache in sync while the dock is open.
  useEffect(() => {
    if (!open) return
    writeCache(contextKey, tabs, activeId)
  }, [open, contextKey, tabs, activeId, writeCache])

  const addTab = useCallback((overrides = {}) => {
    let tabId = ''
    setTabs((prev) => {
      const tab = createTerminalTab({
        contextName,
        namespace,
        ...overrides,
      })
      tabId = tab.id
      return [...prev, tab]
    })
    setActiveId(tabId)
    return tabId
  }, [contextName, namespace])

  const closeTab = useCallback((id) => {
    setTabs((prev) => {
      const idx = prev.findIndex((t) => t.id === id)
      if (idx < 0) return prev
      const next = prev.filter((t) => t.id !== id)
      setActiveId((cur) => {
        if (cur !== id) return cur
        if (next.length === 0) return null
        const pick = next[Math.min(idx, next.length - 1)]
        return pick.id
      })
      if (next.length === 0) {
        seededRef.current = true
        writeCache(contextKeyRef.current, [], null)
        onEmpty?.()
      }
      return next
    })
  }, [onEmpty, writeCache])

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
      if (!persist) {
        writeCache(contextKeyRef.current, tabsRef.current, activeIdRef.current)
        setTabs([])
        setActiveId(null)
        seededRef.current = false
      }
      return
    }

    if (tabs.length > 0) return

    const loaded = loadFromCache(contextKey)
    if (loaded.tabs.length > 0) {
      seededRef.current = true
      setTabs(loaded.tabs)
      setActiveId(loaded.activeId)
      return
    }

    if (!seededRef.current && !skipInitialSeed) {
      seededRef.current = true
      addTab()
    }
  }, [open, persist, skipInitialSeed, tabs.length, contextKey, addTab, loadFromCache, writeCache])

  const activeTab = tabs.find((t) => t.id === activeId) || null

  return {
    tabs,
    activeId,
    activeTab,
    addTab,
    closeTab,
    updateTab,
    selectTab,
    restartTab,
    restartAllTabs,
    contextName,
    namespace,
    contextKey,
  }
}
