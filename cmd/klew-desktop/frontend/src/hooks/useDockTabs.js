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

/**
 * Coordinates the shared tab strip across terminal sessions and log streams.
 */
export function useDockTabs({
  terminalTabs,
  logStreams,
  onOpenTerminalPanel,
  onExpandDock,
}) {
  const [state, setState] = useState({
    tabOrder: [],
    logTabs: [],
    activeKey: null,
  })
  const stateRef = useRef(state)
  stateRef.current = state

  const lastTerminalIdRef = useRef(null)
  const knownTermIdsRef = useRef(new Set())

  // Keep terminal tabs synchronised into the combined tab strip.
  useEffect(() => {
    const termList = terminalTabs?.tabs || []
    const prevKnown = knownTermIdsRef.current
    let newestTermId = null
    for (const t of termList) {
      if (!prevKnown.has(t.id)) {
        newestTermId = t.id
      }
    }
    knownTermIdsRef.current = new Set(termList.map((t) => t.id))
    if (newestTermId) {
      lastTerminalIdRef.current = newestTermId
    } else if (terminalTabs?.activeId) {
      lastTerminalIdRef.current = terminalTabs.activeId
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
  }, [terminalTabs?.tabs, terminalTabs?.activeId])

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
    const termList = terminalTabs?.tabs || []
    if (termList.length > 0) {
      const preferred = lastTerminalIdRef.current
      const target = termList.find((t) => t.id === preferred)
        || termList.find((t) => t.id === terminalTabs?.activeId)
        || termList[termList.length - 1]
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
    }
    return addTerminalTab()
  }, [terminalTabs, onOpenTerminalPanel, onExpandDock, addTerminalTab])

  const openLogStream = useCallback(async (spec = {}) => {
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
    setState((prev) => addOrUpdateLogTab(prev, {
      streamId,
      spec: st?.spec || spec,
      sources: st?.sources || [],
      focus: true,
    }))
    return { streamId, reused: false, status: st }
  }, [logStreams, onExpandDock])

  const mergeIntoLogTab = useCallback(async (streamId, spec = {}) => {
    if (!streamId) return openLogStream(spec)
    onExpandDock?.()
    const st = await logStreams?.start?.({
      ...spec,
      streamId,
      mergeIntoActive: true,
    })
    if (!st) return null
    const id = st.id || st.streamId || streamId
    setState((prev) => addOrUpdateLogTab(prev, {
      streamId: id,
      spec: st.spec || spec,
      sources: st.sources || [],
      focus: true,
    }))
    return st
  }, [logStreams, onExpandDock, openLogStream])

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

  const restoreLogStreams = useCallback((statuses = []) => {
    if (!Array.isArray(statuses) || !statuses.length) return
    setState((prev) => {
      let next = prev
      for (const st of statuses) {
        const id = st?.id || st?.streamId || ''
        if (!id) continue
        next = addOrUpdateLogTab(next, {
          streamId: id,
          spec: st.spec || {},
          sources: st.sources || [],
          focus: !next.activeKey,
        })
      }
      return next
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

  const clearAllTabs = useCallback(() => {
    setState({ tabOrder: [], logTabs: [], activeKey: null })
  }, [])

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
  }
}
