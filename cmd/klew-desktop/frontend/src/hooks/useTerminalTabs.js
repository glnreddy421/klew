import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

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

const STORAGE_KEY = 'klew.terminal.tabs.v2'
const LEGACY_STORAGE_KEY = 'klew.terminal.tabs'

export function contextBucketKey(name) {
  return String(name || '').trim()
}

/** Tabs belonging to one kube context (empty key = unpinned). */
export function tabsForContext(tabs = [], contextName = '') {
  const want = contextBucketKey(contextName)
  return (tabs || []).filter((t) => contextBucketKey(t.contextName) === want)
}

/** Strip runtime-only fields before caching. */
export function tabsForCache(tabs) {
  return (tabs || []).map((t) => ({
    id: t.id,
    title: t.title,
    contextName: t.contextName,
    namespace: t.namespace,
    initialInput: t.initialInput,
    initialInputSent: t.initialInputSent,
  }))
}

/**
 * Persist shape: { byContext: { [ctx]: { tabs, activeId } } }
 * Migrates the flat v1 `{ tabs, activeId }` blob when present.
 */
export function loadPersistedTerminalBuckets() {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY)
    if (raw) {
      const obj = JSON.parse(raw)
      if (obj && obj.byContext && typeof obj.byContext === 'object') {
        return normalizeBuckets(obj.byContext)
      }
    }
  } catch {
    /* ignore */
  }
  try {
    const legacy = sessionStorage.getItem(LEGACY_STORAGE_KEY)
    if (!legacy) return {}
    const obj = JSON.parse(legacy)
    if (!obj || !Array.isArray(obj.tabs)) return {}
    return bucketsFromFlatTabs(obj.tabs, obj.activeId)
  } catch {
    return {}
  }
}

function normalizeBuckets(byContext) {
  const out = {}
  for (const [rawKey, bucket] of Object.entries(byContext || {})) {
    const key = contextBucketKey(rawKey)
    const tabs = Array.isArray(bucket?.tabs) ? bucket.tabs : []
    if (!tabs.length) continue
    out[key] = {
      tabs: tabsForCache(tabs),
      activeId: bucket?.activeId || null,
    }
  }
  return out
}

export function bucketsFromFlatTabs(tabs = [], activeId = null) {
  const out = {}
  for (const t of tabs || []) {
    const key = contextBucketKey(t.contextName)
    if (!out[key]) out[key] = { tabs: [], activeId: null }
    out[key].tabs.push(t)
  }
  for (const [key, bucket] of Object.entries(out)) {
    bucket.tabs = tabsForCache(bucket.tabs)
    const match = (tabs || []).find((t) => t.id === activeId)
    if (match && contextBucketKey(match.contextName) === key) {
      bucket.activeId = activeId
    } else {
      bucket.activeId = bucket.tabs[0]?.id || null
    }
  }
  return out
}

function persistBuckets(byContext) {
  try {
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify({ byContext: normalizeBuckets(byContext) }))
  } catch {
    /* quota / private mode */
  }
}

function bumpSeqFromTabs(tabs) {
  for (const t of tabs || []) {
    const n = Number(String(t.id).replace('term-tab-', ''))
    if (Number.isFinite(n) && n >= nextTabSeq) nextTabSeq = n + 1
  }
}

function restoreAllFromBuckets(byContext) {
  const buckets = normalizeBuckets(byContext)
  const tabs = []
  const activeByContext = {}
  for (const [key, bucket] of Object.entries(buckets)) {
    const revived = reviveTerminalTabs(bucket.tabs.map((t) => ({
      ...t,
      id: t.id || `term-tab-${nextTabSeq++}`,
      title: t.title || 'Shell',
      contextName: t.contextName || key,
    })))
    bumpSeqFromTabs(revived)
    tabs.push(...revived)
    activeByContext[key] = pickActiveId(bucket.activeId, revived)
  }
  return { tabs, activeByContext }
}

function snapshotBuckets(allTabs, activeByContext) {
  const byContext = {}
  for (const t of allTabs || []) {
    const key = contextBucketKey(t.contextName)
    if (!byContext[key]) byContext[key] = { tabs: [], activeId: activeByContext[key] || null }
    byContext[key].tabs.push(t)
  }
  for (const [key, bucket] of Object.entries(byContext)) {
    bucket.tabs = tabsForCache(bucket.tabs)
    if (!bucket.activeId || !bucket.tabs.some((t) => t.id === bucket.activeId)) {
      bucket.activeId = bucket.tabs[0]?.id || null
    }
  }
  return byContext
}

/**
 * Terminal tabs isolated per kube context.
 * The dock only sees the active context's shells; other contexts stay parked
 * (panes stay mounted via allTabs so PTYs survive cluster switches).
 */
export function useTerminalTabs(cluster, {
  open = false,
  persist: keepOnClose = false,
  onEmpty,
  skipInitialSeed = false,
  /** Load saved tabs straight away, so the first open cannot race a restore. */
  restoreOnMount = false,
} = {}) {
  const contextName = cluster?.selectedContext || cluster?.currentContext || ''
  const namespace = ''

  const allTabsRef = useRef([])
  const activeByContextRef = useRef({})
  const seededRef = useRef(false)
  const initialRef = useRef(null)
  if (initialRef.current === null) {
    initialRef.current = restoreOnMount
      ? restoreAllFromBuckets(loadPersistedTerminalBuckets())
      : { tabs: [], activeByContext: {} }
    if (restoreOnMount) seededRef.current = true
  }

  const [allTabs, setAllTabs] = useState(initialRef.current.tabs)
  const [activeByContext, setActiveByContext] = useState(initialRef.current.activeByContext)

  allTabsRef.current = allTabs
  activeByContextRef.current = activeByContext

  const ctxKey = contextBucketKey(contextName)
  const tabs = useMemo(() => tabsForContext(allTabs, ctxKey), [allTabs, ctxKey])
  const activeId = activeByContext[ctxKey] || null
  const activeTab = tabs.find((t) => t.id === activeId) || tabs[tabs.length - 1] || null

  useEffect(() => {
    if (!open && !restoreOnMount) return
    persistBuckets(snapshotBuckets(allTabs, activeByContext))
  }, [open, restoreOnMount, allTabs, activeByContext])

  // Ensure each context has a remembered active tab when its set changes.
  useEffect(() => {
    setActiveByContext((prev) => {
      let changed = false
      const next = { ...prev }
      const grouped = {}
      for (const t of allTabs) {
        const key = contextBucketKey(t.contextName)
        if (!grouped[key]) grouped[key] = []
        grouped[key].push(t)
      }
      for (const [key, list] of Object.entries(grouped)) {
        if (!next[key] || !list.some((t) => t.id === next[key])) {
          next[key] = list[list.length - 1]?.id || null
          changed = true
        }
      }
      for (const key of Object.keys(next)) {
        if (!grouped[key]) {
          delete next[key]
          changed = true
        }
      }
      return changed ? next : prev
    })
  }, [allTabs])

  const addTab = useCallback((overrides = {}) => {
    const pinnedCtx = String(overrides.contextName || contextName || '').trim()
    const pinnedNs = overrides.namespace !== undefined ? overrides.namespace : namespace
    const tab = createTerminalTab({
      ...overrides,
      contextName: pinnedCtx,
      namespace: pinnedNs || '',
      // Strip is already context-scoped — default to Shell N, not the cluster name.
      title: overrides.title || undefined,
    })
    setAllTabs((prev) => [...prev, tab])
    setActiveByContext((prev) => ({
      ...prev,
      [contextBucketKey(pinnedCtx)]: tab.id,
    }))
    return tab.id
  }, [contextName, namespace])

  const closeTab = useCallback((id) => {
    const prev = allTabsRef.current
    const target = prev.find((t) => t.id === id)
    if (!target) return
    const key = contextBucketKey(target.contextName)
    const next = prev.filter((t) => t.id !== id)
    const remaining = tabsForContext(next, key)
    setAllTabs(next)
    setActiveByContext((cur) => {
      const copy = { ...cur }
      if (remaining.length === 0) delete copy[key]
      else if (copy[key] === id) copy[key] = remaining[remaining.length - 1].id
      return copy
    })
    if (remaining.length === 0 && key === contextBucketKey(contextName)) {
      seededRef.current = true
      persistBuckets(snapshotBuckets(next, activeByContextRef.current))
      onEmpty?.()
    }
  }, [onEmpty, contextName])

  const closeTabs = useCallback((ids = []) => {
    const drop = new Set(ids)
    if (drop.size === 0) return
    const prev = allTabsRef.current
    const next = prev.filter((t) => !drop.has(t.id))
    if (next.length === prev.length) return
    setAllTabs(next)
    setActiveByContext((cur) => {
      const copy = { ...cur }
      for (const key of Object.keys(copy)) {
        const remaining = tabsForContext(next, key)
        if (remaining.length === 0) delete copy[key]
        else if (!remaining.some((t) => t.id === copy[key])) {
          copy[key] = remaining[remaining.length - 1].id
        }
      }
      return copy
    })
    if (tabsForContext(next, contextName).length === 0) {
      seededRef.current = true
      persistBuckets(snapshotBuckets(next, activeByContextRef.current))
      onEmpty?.()
    }
  }, [onEmpty, contextName])

  const updateTab = useCallback((id, patch) => {
    setAllTabs((prev) => prev.map((t) => (t.id === id ? { ...t, ...patch } : t)))
  }, [])

  const selectTab = useCallback((id) => {
    const target = allTabsRef.current.find((t) => t.id === id)
    if (!target) return
    const key = contextBucketKey(target.contextName)
    setActiveByContext((prev) => (prev[key] === id ? prev : { ...prev, [key]: id }))
  }, [])

  const restartTab = useCallback((id) => {
    updateTab(id, { shell: '', sessionId: null, ready: false, error: '', restartToken: Date.now() })
  }, [updateTab])

  const restartAllTabs = useCallback(() => {
    const token = Date.now()
    const want = contextBucketKey(contextName)
    setAllTabs((prev) => prev.map((t) => (
      contextBucketKey(t.contextName) !== want
        ? t
        : {
          ...t,
          shell: '',
          sessionId: null,
          ready: false,
          error: '',
          restartToken: token,
        }
    )))
  }, [contextName])

  useEffect(() => {
    if (!open) {
      if (!keepOnClose) {
        if (allTabsRef.current.length > 0) {
          persistBuckets(snapshotBuckets(allTabsRef.current, activeByContextRef.current))
        }
        setAllTabs([])
        setActiveByContext({})
        seededRef.current = false
      }
      return
    }

    if (seededRef.current) return
    seededRef.current = true

    const restored = restoreAllFromBuckets(loadPersistedTerminalBuckets())
    if (restored.tabs.length > 0) {
      // Merge, never replace: a shell opened in the same tick must survive.
      setAllTabs((prev) => {
        const have = new Set(prev.map((t) => t.id))
        return [...restored.tabs.filter((t) => !have.has(t.id)), ...prev]
      })
      setActiveByContext((prev) => ({ ...restored.activeByContext, ...prev }))
      // Current context may still be empty — caller focusOrOpen fills it.
      return
    }

    if (!skipInitialSeed) addTab()
  }, [open, keepOnClose, skipInitialSeed, addTab])

  return {
    /** Dock strip: only the active kube context. */
    tabs,
    /** Keep-alive panes for every context (parked shells stay mounted). */
    allTabs,
    activeId: activeTab?.id || activeId,
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
