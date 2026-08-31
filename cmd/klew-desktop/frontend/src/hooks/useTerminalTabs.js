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
  }
}

export function useTerminalTabs(cluster, { open = false, persist = false, onEmpty } = {}) {
  const [tabs, setTabs] = useState([])
  const [activeId, setActiveId] = useState(null)
  const seededRef = useRef(false)

  const contextName = cluster?.selectedContext || cluster?.currentContext || ''
  const namespace = cluster?.selectedNamespace || ''

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
      if (next.length === 0) onEmpty?.()
      return next
    })
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
      if (!persist) {
        seededRef.current = false
        setTabs([])
        setActiveId(null)
      }
      return
    }
    if (tabs.length === 0 && !seededRef.current) {
      seededRef.current = true
      addTab()
    }
  }, [open, persist, tabs.length, addTab])

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
  }
}
