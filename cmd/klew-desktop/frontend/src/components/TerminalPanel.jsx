import { PANEL_CLOSED, PANEL_MAXIMIZED, PANEL_MINIMIZED, PANEL_NORMAL } from '../hooks/useStreamPanel'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTerminalTabs } from '../hooks/useTerminalTabs'
import { TerminalTabPane } from './TerminalTabPane'
import {
  createSplitView,
  isTerminalPaneVisible,
  resolveSplitPartner,
  splitIncludesTab,
} from '../lib/terminalSplit'

import { TerminalFindBar } from './TerminalFindBar'
import { normalizeTerminalAppearance, terminalAppearanceStyle } from '../lib/terminalAppearance'
import { isEditableTarget } from '../lib/keyboard'

export function TerminalPanel({
  open,
  cluster,
  shellPref = '',
  appearance = 'midnight',
  panelState = PANEL_NORMAL,
  height = 280,
  layout = 'dock',
  onClose,
  onMinimize,
  onMaximize,
  onRestore,
  onResizeStart,
  onChangeShell,
  shellRestartToken = 0,
  launchRequest = null,
  onLaunchHandled,
  embedded = false,
}) {
  const isWorkspace = layout === 'workspace'
  const isOpen = open && panelState !== PANEL_CLOSED
  const minimized = !embedded && panelState === PANEL_MINIMIZED

  const {
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
  } = useTerminalTabs(cluster, {
    open: isOpen,
    persist: isWorkspace,
    onEmpty: isWorkspace ? undefined : onClose,
    skipInitialSeed: Boolean(launchRequest?.id),
  })

  const shellRestartRef = useRef(shellRestartToken)
  const launchRef = useRef(null)
  const tabMenuRef = useRef(null)
  const tabSearchApisRef = useRef(new Map())
  const findResultsDisposeRef = useRef(null)
  const [splitView, setSplitView] = useState(null)
  const [tabMenu, setTabMenu] = useState(null)
  const [findOpen, setFindOpen] = useState(false)
  const [findQuery, setFindQuery] = useState('')
  const [findResult, setFindResult] = useState({ index: -1, count: 0 })
  const findQueryRef = useRef(findQuery)
  findQueryRef.current = findQuery

  const focusedTabId = splitView?.focusId || activeId

  const getSearchApi = useCallback(() => {
    if (!focusedTabId) return null
    return tabSearchApisRef.current.get(focusedTabId) || null
  }, [focusedTabId])

  const bindFindResults = useCallback((tabId) => {
    findResultsDisposeRef.current?.dispose?.()
    findResultsDisposeRef.current = null
    if (!tabId) return
    const api = tabSearchApisRef.current.get(tabId)
    if (!api) return
    findResultsDisposeRef.current = api.onResults((result) => {
      const count = result?.resultCount ?? api.countMatches?.(findQueryRef.current.trim()) ?? 0
      setFindResult({
        index: result?.resultIndex ?? -1,
        count,
      })
    })
  }, [])

  const closeFind = useCallback(() => {
    getSearchApi()?.clearDecorations()
    findResultsDisposeRef.current?.dispose?.()
    findResultsDisposeRef.current = null
    setFindOpen(false)
    setFindResult({ index: -1, count: 0 })
  }, [getSearchApi])

  const applyFindResult = useCallback((api, query, found) => {
    const count = api?.countMatches?.(query) ?? 0
    setFindResult({
      index: found && count > 0 ? 0 : -1,
      count,
    })
  }, [])

  const runFind = useCallback((direction = 'next') => {
    const trimmed = findQuery.trim()
    const api = getSearchApi()
    if (!api) return
    if (!trimmed) {
      api.clearDecorations()
      setFindResult({ index: -1, count: 0 })
      return
    }
    if (direction === 'prev') api.findPrevious(trimmed)
    else api.findNext(trimmed)
    setFindResult((prev) => ({
      ...prev,
      count: api.countMatches?.(trimmed) ?? 0,
    }))
  }, [findQuery, getSearchApi])

  const handleFindQueryChange = useCallback((query) => {
    setFindQuery(query)
    const trimmed = query.trim()
    const api = getSearchApi()
    if (!trimmed) {
      api?.clearDecorations()
      setFindResult({ index: -1, count: 0 })
      return
    }
    const found = api.findNext(trimmed, { incremental: true })
    applyFindResult(api, trimmed, found)
  }, [getSearchApi, applyFindResult])

  const openFind = useCallback(() => {
    setFindOpen(true)
  }, [])

  const registerTabSearch = useCallback((tabId, api) => {
    if (!api) {
      tabSearchApisRef.current.delete(tabId)
      return
    }
    tabSearchApisRef.current.set(tabId, api)
    if (findOpen && tabId === focusedTabId) {
      bindFindResults(tabId)
      const query = findQueryRef.current.trim()
      if (query) {
        const found = api.findNext(query, { incremental: true })
        applyFindResult(api, query, found)
      }
    }
  }, [findOpen, focusedTabId, bindFindResults, applyFindResult])

  const dismissTabMenu = useCallback(() => setTabMenu(null), [])

  const handleSelectTab = useCallback((id) => {
    setSplitView(null)
    selectTab(id)
  }, [selectTab])

  const handleCloseTab = useCallback((id) => {
    setSplitView((prev) => {
      if (!prev || !splitIncludesTab(prev, id)) return prev
      return null
    })
    closeTab(id)
  }, [closeTab])

  const openSplit = useCallback((orientation, anchorTabId) => {
    const partnerId = resolveSplitPartner(tabs, anchorTabId, activeId)
    const next = createSplitView(orientation, anchorTabId, partnerId)
    if (!next) return
    setSplitView(next)
    dismissTabMenu()
  }, [tabs, activeId, dismissTabMenu])

  useEffect(() => {
    if (tabs.length < 2) setSplitView(null)
  }, [tabs.length])

  useEffect(() => {
    if (!tabMenu) return undefined

    function onPointerDown(e) {
      if (tabMenuRef.current?.contains(e.target)) return
      dismissTabMenu()
    }
    function onKeyDown(e) {
      if (e.key === 'Escape') dismissTabMenu()
    }
    window.addEventListener('mousedown', onPointerDown)
    window.addEventListener('keydown', onKeyDown)
    window.addEventListener('scroll', dismissTabMenu, true)
    return () => {
      window.removeEventListener('mousedown', onPointerDown)
      window.removeEventListener('keydown', onKeyDown)
      window.removeEventListener('scroll', dismissTabMenu, true)
    }
  }, [tabMenu, dismissTabMenu])

  useEffect(() => {
    if (!isOpen || minimized) return undefined
    function onKeyDown(e) {
      if (!(e.metaKey || e.ctrlKey) || e.key.toLowerCase() !== 'f') return
      if (e.altKey) return
      if (isEditableTarget(e.target) && !e.target?.closest?.('.terminal-find-bar')) return
      e.preventDefault()
      e.stopPropagation()
      setFindOpen(true)
    }
    window.addEventListener('keydown', onKeyDown, true)
    return () => window.removeEventListener('keydown', onKeyDown, true)
  }, [isOpen, minimized])

  useEffect(() => {
    if (!findOpen) {
      findResultsDisposeRef.current?.dispose?.()
      findResultsDisposeRef.current = null
      return undefined
    }
    bindFindResults(focusedTabId)
    return () => {
      findResultsDisposeRef.current?.dispose?.()
      findResultsDisposeRef.current = null
    }
  }, [findOpen, focusedTabId, bindFindResults])

  useEffect(() => {
    if (!findOpen) return
    const query = findQueryRef.current.trim()
    if (!query) return
    getSearchApi()?.findNext(query, { incremental: true })
  }, [findOpen, focusedTabId, getSearchApi])

  useEffect(() => {
    if (!isOpen) closeFind()
  }, [isOpen, closeFind])

  useEffect(() => {
    if (!isOpen || !shellRestartToken || shellRestartRef.current === shellRestartToken) return
    shellRestartRef.current = shellRestartToken
    if (tabs.length > 0) restartAllTabs()
  }, [isOpen, shellRestartToken, tabs.length, restartAllTabs])

  useEffect(() => {
    if (!launchRequest?.id || launchRef.current === launchRequest.id) return
    launchRef.current = launchRequest.id
    addTab({
      title: launchRequest.title,
      namespace: launchRequest.namespace || '',
      initialInput: launchRequest.initialInput || '',
    })
    onLaunchHandled?.()
  }, [launchRequest, addTab, onLaunchHandled])

  if (!open || panelState === PANEL_CLOSED) {
    return null
  }

  const maximized = !embedded && panelState === PANEL_MAXIMIZED
  const appearanceId = normalizeTerminalAppearance(appearance)

  const handleTabState = (id, patch) => {
    updateTab(id, patch)
  }

  return (
    <section
      className={[
        'stream-panel',
        'terminal-panel',
        embedded ? 'terminal-panel-embedded' : '',
        `terminal-theme-${appearanceId}`,
        isWorkspace ? 'terminal-panel-workspace' : '',
        maximized ? 'maximized' : '',
        minimized ? 'collapsed' : '',
      ].filter(Boolean).join(' ')}
      style={{
        ...(!maximized && !minimized && height ? { height } : null),
        ...terminalAppearanceStyle(appearanceId),
      }}
      aria-label="Cluster terminal"
    >
      <header className="stream-header terminal-header">
        {!minimized && !maximized && (
          <div
            className="stream-resize-handle"
            onMouseDown={onResizeStart}
            role="separator"
            aria-orientation="horizontal"
            aria-label="Resize terminal"
          />
        )}
        <div className="stream-title">
          <TerminalIcon />
          <span>Terminal</span>
          {activeTab?.shell && (
            <span className="stream-chip terminal-shell-chip">{activeTab.shell}</span>
          )}
          <span className="stream-chip mono terminal-context-chip">
            {contextName}{namespace ? ` / ${namespace}` : ''}
          </span>
        </div>
        <div className="stream-toolbar">
          <button
            type="button"
            className="btn btn-ghost btn-sm"
            onClick={onChangeShell}
            title="Change shell"
          >
            Shell…
          </button>
          <button
            type="button"
            className="btn btn-ghost btn-sm"
            onClick={openFind}
            disabled={!activeId && !splitView}
            title="Find in terminal (⌘F)"
          >
            Find
          </button>
          <button
            type="button"
            className="btn btn-ghost btn-sm"
            onClick={() => restartTab(activeId)}
            disabled={!activeId}
            title="Restart active shell"
          >
            Restart
          </button>
          {onClose && !embedded ? (
            <button type="button" className="stream-icon-btn" onClick={onClose} aria-label="Close terminal">×</button>
          ) : null}
          {!embedded && (
            <>
          {minimized ? (
            <button type="button" className="stream-icon-btn" onClick={onRestore} aria-label="Expand">▲</button>
          ) : (
            <button type="button" className="stream-icon-btn" onClick={onMinimize} aria-label="Minimize">▼</button>
          )}
          {!maximized ? (
            <button type="button" className="stream-icon-btn" onClick={onMaximize} aria-label="Maximize">⛶</button>
          ) : (
            <button type="button" className="stream-icon-btn" onClick={onRestore} aria-label="Restore">⛶</button>
          )}
            </>
          )}
        </div>
      </header>

      {!minimized && (
        <>
          <div className="terminal-tab-bar" role="tablist" aria-label="Terminal tabs">
            {tabs.map((tab) => (
              <div
                key={tab.id}
                className={[
                  'terminal-tab',
                  tab.id === activeId && !splitView ? 'is-active' : '',
                  splitView && splitIncludesTab(splitView, tab.id) ? 'is-split-member' : '',
                  tab.ready ? '' : 'is-pending',
                ].filter(Boolean).join(' ')}
                onContextMenu={(e) => {
                  if (tabs.length < 2) return
                  e.preventDefault()
                  setTabMenu({ tabId: tab.id, x: e.clientX, y: e.clientY })
                }}
              >
                <button
                  type="button"
                  className="terminal-tab-select"
                  role="tab"
                  aria-selected={tab.id === activeId && !splitView}
                  title={[
                    tab.title,
                    tab.shell,
                    tab.contextName,
                    tab.namespace,
                    tabs.length >= 2 ? 'Right-click to split' : '',
                  ].filter(Boolean).join(' · ')}
                  onClick={() => handleSelectTab(tab.id)}
                >
                  <span className="terminal-tab-label">{tab.title}</span>
                  {tab.shell && (
                    <span className="terminal-tab-shell">{tab.shell}</span>
                  )}
                  {!tab.ready && !tab.error && <span className="terminal-tab-dot" aria-hidden="true" />}
                </button>
                <button
                  type="button"
                  className="terminal-tab-close"
                  aria-label={`Close ${tab.title}`}
                  onClick={(e) => {
                    e.stopPropagation()
                    handleCloseTab(tab.id)
                  }}
                >
                  ×
                </button>
              </div>
            ))}
            <button
              type="button"
              className="terminal-tab-add"
              aria-label="New terminal tab"
              title="New tab"
              onClick={() => addTab()}
            >
              +
            </button>
          </div>

          <div className="stream-body terminal-body">
            <TerminalFindBar
              open={findOpen}
              query={findQuery}
              resultIndex={findResult.index}
              resultCount={findResult.count}
              onQueryChange={handleFindQueryChange}
              onFindNext={() => runFind('next')}
              onFindPrevious={() => runFind('prev')}
              onClose={closeFind}
            />
            {tabs.length === 0 && (
              <div className="terminal-empty" role="status">
                No open terminals.
                <button type="button" className="btn btn-ghost btn-sm" onClick={() => addTab()}>
                  New tab
                </button>
              </div>
            )}
            {activeTab?.error && (
              <div className="terminal-error" role="alert">{activeTab.error}</div>
            )}
            <div
              className={[
                'terminal-host',
                'terminal-host-tabs',
                splitView ? `is-split is-split-${splitView.orientation}` : '',
              ].filter(Boolean).join(' ')}
            >
              {tabs.map((tab) => {
                const visible = isTerminalPaneVisible(splitView, tab.id, activeId)
                const paneActive = splitView ? visible : tab.id === activeId
                const focused = splitView ? splitView.focusId === tab.id : tab.id === activeId
                return (
                  <div
                    key={tab.id}
                    className={[
                      'terminal-split-pane',
                      visible ? 'is-visible' : '',
                      focused ? 'is-focused' : '',
                    ].filter(Boolean).join(' ')}
                    onMouseDown={() => {
                      if (!splitView) return
                      setSplitView((prev) => (prev ? { ...prev, focusId: tab.id } : prev))
                    }}
                  >
                    <TerminalTabPane
                      tab={tab}
                      active={paneActive}
                      focused={focused}
                      open={isOpen && !minimized}
                      cluster={cluster}
                      shellPref={shellPref}
                      appearance={appearanceId}
                      onStateChange={handleTabState}
                      onSearchRegister={(api) => registerTabSearch(tab.id, api)}
                    />
                  </div>
                )
              })}
            </div>
          </div>

          {tabMenu && (
            <div
              ref={tabMenuRef}
              className="terminal-tab-menu"
              role="menu"
              aria-label="Terminal tab actions"
              style={{ left: tabMenu.x, top: tabMenu.y }}
            >
              <button
                type="button"
                role="menuitem"
                className="terminal-tab-menu-item"
                onClick={() => openSplit('horizontal', tabMenu.tabId)}
              >
                <SplitRightIcon />
                <span>Split right</span>
              </button>
              <button
                type="button"
                role="menuitem"
                className="terminal-tab-menu-item"
                onClick={() => openSplit('vertical', tabMenu.tabId)}
              >
                <SplitDownIcon />
                <span>Split down</span>
              </button>
              {splitView && (
                <button
                  type="button"
                  role="menuitem"
                  className="terminal-tab-menu-item"
                  onClick={() => {
                    setSplitView(null)
                    dismissTabMenu()
                  }}
                >
                  <CloseSplitIcon />
                  <span>Close split</span>
                </button>
              )}
            </div>
          )}
        </>
      )}
    </section>
  )
}

function TerminalIcon() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <rect x="1.5" y="3" width="13" height="10" rx="1.5" />
      <path d="M4.5 7.5L6.5 9.5L4.5 11.5" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M8 11.5h3.5" strokeLinecap="round" />
    </svg>
  )
}

function SplitRightIcon() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.25" aria-hidden="true">
      <rect x="2" y="3" width="12" height="10" rx="1.5" />
      <path d="M8 3v10" />
    </svg>
  )
}

function SplitDownIcon() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.25" aria-hidden="true">
      <rect x="2" y="3" width="12" height="10" rx="1.5" />
      <path d="M2 8h12" />
    </svg>
  )
}

function CloseSplitIcon() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.25" aria-hidden="true">
      <rect x="2" y="3" width="12" height="10" rx="1.5" />
      <path d="M5 8h6" />
    </svg>
  )
}
