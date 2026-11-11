/** Pure helpers for the unified terminal + log stream tab strip. */

import { logSpecKey, logTabTitle } from './logStream'

export function termTabKey(termId) {
  return termId ? `term:${termId}` : ''
}

export function logTabKey(streamId) {
  return streamId ? `log:${streamId}` : ''
}

export function parseDockTabKey(key) {
  const s = String(key || '')
  if (s.startsWith('term:')) return { kind: 'terminal', id: s.slice(5) }
  if (s.startsWith('log:')) return { kind: 'log', id: s.slice(4) }
  return null
}

/**
 * Pick an existing terminal tab pinned to wantCtx (empty → unpinned only).
 * Prefer preferredId, then activeId, then the most recently added same-context tab.
 */
export function pickTerminalTabForContext(termList = [], wantCtx = '', preferredId = null, activeId = null) {
  const want = String(wantCtx || '').trim()
  const sameCtx = want
    ? termList.filter((t) => String(t.contextName || '').trim() === want)
    : termList.filter((t) => !String(t.contextName || '').trim())
  if (sameCtx.length === 0) return null
  return sameCtx.find((t) => t.id === preferredId)
    || sameCtx.find((t) => t.id === activeId)
    || sameCtx[sameCtx.length - 1]
}

/**
 * Synchronises terminal tabs into the combined tabOrder while preserving
 * the relative order of existing terminal and log tabs.
 */
export function syncTerminalTabsInOrder(tabOrder = [], termTabs = []) {
  const liveTermKeys = new Set((termTabs || []).map((t) => termTabKey(t.id)).filter(Boolean))
  const kept = tabOrder.filter((k) => !k.startsWith('term:') || liveTermKeys.has(k))
  const present = new Set(kept)
  const added = []
  for (const t of termTabs || []) {
    const k = termTabKey(t.id)
    if (k && !present.has(k)) {
      present.add(k)
      added.push(k)
    }
  }
  if (added.length === 0 && kept.length === tabOrder.length) return tabOrder
  return [...kept, ...added]
}

/**
 * Finds an existing log tab matching the same selection (targets + container).
 */
export function findMatchingLogTab(logTabs = [], spec = {}) {
  const key = logSpecKey(spec)
  if (!key) return null
  return (logTabs || []).find((t) => t.selectionKey === key) || null
}

/**
 * Adds or updates a log tab and places it in tabOrder.
 */
export function addOrUpdateLogTab(state, { streamId, spec = {}, sources = [], title = '', focus = true }) {
  if (!streamId) return state
  const key = logTabKey(streamId)
  const selectionKey = logSpecKey(spec)
  const computedTitle = title || logTabTitle(spec, sources)
  const existingIdx = state.logTabs.findIndex((t) => t.streamId === streamId)
  const entry = {
    streamId,
    selectionKey,
    title: computedTitle,
    spec,
  }
  const nextLogTabs = existingIdx >= 0
    ? state.logTabs.map((t, i) => (i === existingIdx ? entry : t))
    : [...state.logTabs, entry]
  const nextOrder = state.tabOrder.includes(key)
    ? state.tabOrder
    : [...state.tabOrder, key]
  return {
    ...state,
    tabOrder: nextOrder,
    logTabs: nextLogTabs,
    activeKey: focus ? key : (state.activeKey || key),
  }
}

/**
 * Removes a tab key from tabOrder (and logTabs when it is a log tab) and
 * picks the neighboring tab as activeKey when the active tab was closed.
 */
export function removeTabFromDock(state, key) {
  const idx = state.tabOrder.indexOf(key)
  const nextOrder = idx >= 0
    ? state.tabOrder.filter((k) => k !== key)
    : state.tabOrder
  const parsed = parseDockTabKey(key)
  const nextLogTabs = parsed?.kind === 'log'
    ? state.logTabs.filter((t) => t.streamId !== parsed.id)
    : state.logTabs

  let nextActive = state.activeKey
  if (state.activeKey === key || (nextActive && !nextOrder.includes(nextActive))) {
    if (nextOrder.length === 0) {
      nextActive = null
    } else {
      const pickIdx = idx >= 0 ? Math.min(idx, nextOrder.length - 1) : nextOrder.length - 1
      nextActive = nextOrder[pickIdx]
    }
  }
  return {
    ...state,
    tabOrder: nextOrder,
    logTabs: nextLogTabs,
    activeKey: nextActive,
  }
}

/** Keys of the tabs after `key` in strip order — empty when it is the last tab. */
export function tabsRightOf(tabs = [], key = '') {
  const idx = tabs.findIndex((t) => t.key === key)
  if (idx < 0) return []
  return tabs.slice(idx + 1).map((t) => t.key)
}

/**
 * Builds the ordered list of display tab descriptors for the dock header.
 */
export function buildDockTabItems(tabOrder = [], termTabs = [], logTabs = [], streamsById = {}) {
  const termById = new Map((termTabs || []).map((t) => [t.id, t]))
  const logById = new Map((logTabs || []).map((t) => [t.streamId, t]))
  const items = []
  for (const key of tabOrder) {
    const parsed = parseDockTabKey(key)
    if (!parsed) continue
    if (parsed.kind === 'terminal') {
      const t = termById.get(parsed.id)
      if (!t) continue
      items.push({
        key,
        kind: 'terminal',
        id: t.id,
        title: t.title || 'Shell',
        shell: t.shell || '',
        ready: Boolean(t.ready),
        error: t.error || '',
        contextName: t.contextName || '',
        namespace: t.namespace || t.shellNamespace || '',
      })
    } else if (parsed.kind === 'log') {
      const l = logById.get(parsed.id)
      if (!l) continue
      const stream = streamsById[parsed.id]
      const st = stream?.status || {}
      const active = Boolean(st.active)
      const paused = Boolean(st.paused)
      const state = active ? (paused ? 'paused' : 'live') : 'idle'
      items.push({
        key,
        kind: 'log',
        id: l.streamId,
        streamId: l.streamId,
        title: l.title || logTabTitle(st.spec || l.spec, st.sources),
        selectionKey: l.selectionKey,
        state,
        sources: st.sources || [],
      })
    }
  }
  return items
}
