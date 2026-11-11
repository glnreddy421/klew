import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  addOrUpdateLogTab,
  buildDockTabItems,
  findMatchingLogTab,
  logTabKey,
  parseDockTabKey,
  removeTabFromDock,
  syncTerminalTabsInOrder,
  termTabKey,
} from '../lib/dockTabs'

const EMPTY_DOCK = { tabOrder: [], logTabs: [], activeKey: null }

/**
 * Coordinates the shared tab strip across terminal sessions and log streams.
 */
export function useDockTabs({
  terminalTabs,
  logStreams,
  onOpenTerminalPanel,
  onExpandDock,
}) {
  // One strip per kube context: its tab order, log tabs and active tab are
  // parked while another context is showing and come back untouched.
  const ctx = String(terminalTabs?.contextName || '')
  const [byContext, setByContext] = useState({})
  const byContextRef = useRef(byContext)
  byContextRef.current = byContext
  const state = byContext[ctx] || EMPTY_DOCK
  const stateRef = useRef(state)
  stateRef.current = state
  const ctxRef = useRef(ctx)
  ctxRef.current = ctx

  /** Update one context's strip (the showing one unless `forCtx` is given). */
  const setStateFor = useCallback((forCtx, update) => {
    setByContext((all) => {
      const prev = all[forCtx] || EMPTY_DOCK
      const next = typeof update === 'function' ? update(prev) : update
      return next === prev ? all : { ...all, [forCtx]: next }
    })
  }, [])
  const setState = useCallback((update) => setStateFor(ctxRef.current, update), [setStateFor])

  const lastTerminalIdRef = useRef(null)
  const knownTermIdsRef = useRef(new Set())
  const lastContextRef = useRef(ctx)

  // Keep terminal tabs synchronised into the combined tab strip.
  useEffect(() => {
    const termList = terminalTabs?.tabs || []
    // A context switch swaps the whole strip: those shells are parked, not new,
    // so land on the tab that context last had active.
    const switched = lastContextRef.current !== ctx
    lastContextRef.current = ctx
    const prevKnown = knownTermIdsRef.current
    let newestTermId = null
    if (!switched) {
      for (const t of termList) {
        if (!prevKnown.has(t.id)) {
          newestTermId = t.id
        }
      }
    }
    knownTermIdsRef.current = new Set(termList.map((t) => t.id))
    if (newestTermId) {
      lastTerminalIdRef.current = newestTermId
    } else if (terminalTabs?.activeId) {
      lastTerminalIdRef.current = terminalTabs.activeId
    } else if (switched) {
      lastTerminalIdRef.current = null
    }

    setState((prev) => {
      const nextOrder = syncTerminalTabsInOrder(prev.tabOrder, termList)
      let nextActive = prev.activeKey
      if (newestTermId) {
        nextActive = termTabKey(newestTermId)
      } else if (nextActive && !nextOrder.includes(nextActive)) {
        nextActive = nextOrder[nextOrder.length - 1] || null
      } else if (!nextActive && nextOrder.length > 0) {
        nextActive = nextOrder[0]
      }
      if (nextOrder === prev.tabOrder && nextActive === prev.activeKey) return prev
      return {
        ...prev,
        tabOrder: nextOrder,
        activeKey: nextActive,
      }
    })
  }, [terminalTabs?.tabs, terminalTabs?.activeId, ctx, setState])

  const selectTab = useCallback((key) => {
    const parsed = parseDockTabKey(key)
    if (!parsed) return
    setState((prev) => (prev.activeKey === key ? prev : { ...prev, activeKey: key }))
    if (parsed.kind === 'terminal') {
      lastTerminalIdRef.current = parsed.id
      terminalTabs?.selectTab?.(parsed.id)
    }
  }, [terminalTabs])

  const addTerminalTab = useCallback((overrides = {}) => {
    onOpenTerminalPanel?.()
    const id = terminalTabs?.addTab?.(overrides)
    if (id) {
      lastTerminalIdRef.current = id
      const k = termTabKey(id)
      setState((prev) => ({
        ...prev,
        tabOrder: prev.tabOrder.includes(k) ? prev.tabOrder : [...prev.tabOrder, k],
        activeKey: k,
      }))
    }
    onExpandDock?.()
    return id
  }, [terminalTabs, onOpenTerminalPanel, onExpandDock])

  const focusOrOpenTerminal = useCallback(() => {
    onOpenTerminalPanel?.()
    // terminalTabs.tabs is already isolated to the active kube context.
    const termList = terminalTabs?.tabs || []
    const wantCtx = String(terminalTabs?.contextName || '').trim()
    const target = termList.find((t) => t.id === lastTerminalIdRef.current)
      || termList.find((t) => t.id === terminalTabs?.activeId)
      || termList[termList.length - 1]
      || null
    if (target) {
      lastTerminalIdRef.current = target.id
      terminalTabs?.selectTab?.(target.id)
      setState((prev) => ({
        ...prev,
        tabOrder: syncTerminalTabsInOrder(prev.tabOrder, termList),
        activeKey: termTabKey(target.id),
      }))
      onExpandDock?.()
      return target.id
    }
    return addTerminalTab({
      contextName: wantCtx,
    })
  }, [terminalTabs, onOpenTerminalPanel, onExpandDock, addTerminalTab])

  const openLogStream = useCallback(async (spec = {}) => {
    // The stream belongs to the context it was started in, even if the user
    // switches away before it comes up.
    const startCtx = ctxRef.current
    const curState = stateRef.current
    const existing = findMatchingLogTab(curState.logTabs, spec)
    if (existing) {
      const existingStream = logStreams?.streams?.[existing.streamId]
      if (existingStream?.status?.active) {
        const key = logTabKey(existing.streamId)
        setState((prev) => ({ ...prev, activeKey: key }))
        onExpandDock?.()
        return {
          streamId: existing.streamId,
          reused: true,
          status: existingStream.status,
        }
      }
      // Existing tab was stopped (idle): remove old entry and replace with fresh stream.
      setState((prev) => removeTabFromDock(prev, logTabKey(existing.streamId)))
    }

    onExpandDock?.()
    const st = await logStreams?.start?.(spec)
    const streamId = st?.id || st?.streamId || ''
    if (!streamId) {
      return { streamId: '', reused: false, status: null }
    }
    setStateFor(startCtx, (prev) => addOrUpdateLogTab(prev, {
      streamId,
      spec: st?.spec || spec,
      sources: st?.sources || [],
      focus: true,
    }))
    return { streamId, reused: false, status: st }
  }, [logStreams, onExpandDock, setState, setStateFor])

  const mergeIntoLogTab = useCallback(async (streamId, spec = {}) => {
    if (!streamId) return openLogStream(spec)
    const startCtx = ctxRef.current
    onExpandDock?.()
    const st = await logStreams?.start?.({
      ...spec,
      streamId,
      mergeIntoActive: true,
    })
    if (!st) return null
    const id = st.id || st.streamId || streamId
    setStateFor(startCtx, (prev) => addOrUpdateLogTab(prev, {
      streamId: id,
      spec: st.spec || spec,
      sources: st.sources || [],
      focus: true,
    }))
    return st
  }, [logStreams, onExpandDock, openLogStream, setStateFor])

  /** Close one or more tabs. Batched so "Close others" / "Close all" cannot read stale state. */
  const closeTabs = useCallback((keys = []) => {
    let nextState = stateRef.current
    const termIds = []
    for (const key of keys) {
      const parsed = parseDockTabKey(key)
      if (!parsed) continue
      nextState = removeTabFromDock(nextState, key)
      if (parsed.kind === 'log') logStreams?.stop?.(parsed.id, { keepEntry: false })
      else if (parsed.kind === 'terminal') termIds.push(parsed.id)
    }
    if (nextState === stateRef.current) return
    setState(nextState)
    if (termIds.length > 0) terminalTabs?.closeTabs?.(termIds)
    const nextParsed = parseDockTabKey(nextState.activeKey)
    if (nextParsed?.kind === 'terminal') {
      lastTerminalIdRef.current = nextParsed.id
      terminalTabs?.selectTab?.(nextParsed.id)
    }
  }, [logStreams, terminalTabs])

  const closeTab = useCallback((key) => closeTabs([key]), [closeTabs])

  /** Rebuild log tabs after a reload; `contextById` says which context owns each stream. */
  const restoreLogStreams = useCallback((statuses = [], contextById = {}) => {
    if (!Array.isArray(statuses) || !statuses.length) return
    setByContext((all) => {
      const out = { ...all }
      for (const st of statuses) {
        const id = st?.id || st?.streamId || ''
        if (!id) continue
        const owner = String(contextById?.[id] ?? ctxRef.current)
        const prev = out[owner] || EMPTY_DOCK
        out[owner] = addOrUpdateLogTab(prev, {
          streamId: id,
          spec: st.spec || {},
          sources: st.sources || [],
          focus: !prev.activeKey,
        })
      }
      return out
    })
  }, [])

  const clearLogTabs = useCallback(() => {
    setState((prev) => {
      let next = prev
      for (const lt of prev.logTabs) {
        next = removeTabFromDock(next, logTabKey(lt.streamId))
      }
      return next
    })
  }, [])

  /**
   * Drop one context's whole strip and stop its log streams (its context tab
   * was closed). The caller closes that context's shells.
   */
  const closeContext = useCallback((name) => {
    const key = String(name || '')
    const bucket = byContextRef.current[key]
    if (!bucket) return
    for (const lt of bucket.logTabs) logStreams?.stop?.(lt.streamId, { keepEntry: false })
    setByContext((all) => {
      if (!all[key]) return all
      const next = { ...all }
      delete next[key]
      return next
    })
  }, [logStreams])

  /** Clears the showing context's strip only. */
  const clearAllTabs = useCallback(() => {
    setState(EMPTY_DOCK)
  }, [setState])

  const tabs = useMemo(
    () => buildDockTabItems(state.tabOrder, terminalTabs?.tabs || [], state.logTabs, logStreams?.streams || {}),
    [state.tabOrder, terminalTabs?.tabs, state.logTabs, logStreams?.streams],
  )

  const activeItem = useMemo(() => parseDockTabKey(state.activeKey), [state.activeKey])

  return {
    tabs,
    logTabs: state.logTabs,
    activeKey: state.activeKey,
    activeItem,
    selectTab,
    addTerminalTab,
    focusOrOpenTerminal,
    openLogStream,
    mergeIntoLogTab,
    closeTab,
    closeTabs,
    restoreLogStreams,
    clearLogTabs,
    clearAllTabs,
    closeContext,
  }
}
