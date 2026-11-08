import { useCallback, useEffect, useRef, useState } from 'react'
import { PANEL_NORMAL } from '../hooks/useStreamPanel'
import { LiveStreamPanel } from './LiveStreamPanel'
import {
  CloseSplitIcon,
  SplitDownIcon,
  SplitRightIcon,
  TerminalActionButtons,
  TerminalPanel,
} from './TerminalPanel'
import { StreamLogsGlyph, TerminalGlyph } from './DockLauncher'
import { tabsRightOf } from '../lib/dockTabs'
import {
  createSplitView,
  resolveSplitPartner,
  splitIncludesTab,
} from '../lib/terminalSplit'

/**
 * Unified bottom dock: one tab strip shared by terminal shells and independent
 * log streams, with a [+] button and [▾] menu to open a terminal or stream the
 * current Resources selection.
 */
export function ConsoleDock({
  visible = true,
  tabs = [],
  logTabs = [],
  activeKey = null,
  activeItem = null,
  onSelectTab,
  onCloseTab,
  onCloseTabs,
  onNewTerminal,
  onStreamSelection,
  canStreamSelection = false,
  selectionLabel = '',
  height,
  maximized,
  onMinimize,
  onMaximize,
  onRestore,
  onClose,
  onResizeStart,
  logStreams,
  streamProps = {},
  terminalTabs,
  terminalProps = {},
}) {
  const [terminalActions, setTerminalActions] = useState(null)
  const [addMenuOpen, setAddMenuOpen] = useState(false)
  const [tabMenu, setTabMenu] = useState(null)
  const [splitView, setSplitView] = useState(null)
  const addAnchorRef = useRef(null)
  const addMenuRef = useRef(null)
  const tabMenuRef = useRef(null)

  const termList = terminalTabs?.tabs || []
  const termCount = termList.length

  useEffect(() => {
    if (termCount < 2) setSplitView(null)
  }, [termCount])

  useEffect(() => {
    if (!addMenuOpen) return undefined
    function onPointerDown(e) {
      if (addAnchorRef.current?.contains(e.target)) return
      if (addMenuRef.current?.contains(e.target)) return
      setAddMenuOpen(false)
    }
    function onKeyDown(e) {
      if (e.key === 'Escape') setAddMenuOpen(false)
    }
    window.addEventListener('mousedown', onPointerDown)
    window.addEventListener('keydown', onKeyDown)
    return () => {
      window.removeEventListener('mousedown', onPointerDown)
      window.removeEventListener('keydown', onKeyDown)
    }
  }, [addMenuOpen])

  const dismissTabMenu = useCallback(() => setTabMenu(null), [])

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

  const openSplit = useCallback((orientation, anchorTabId) => {
    const partnerId = resolveSplitPartner(termList, anchorTabId, terminalTabs?.activeId)
    const next = createSplitView(orientation, anchorTabId, partnerId)
    if (!next) return
    setSplitView(next)
    onSelectTab?.(`term:${anchorTabId}`)
    dismissTabMenu()
  }, [termList, terminalTabs?.activeId, onSelectTab, dismissTabMenu])

  const handleSelectDockTab = useCallback((tab) => {
    if (tab.kind === 'terminal') {
      setSplitView(null)
    }
    onSelectTab?.(tab.key)
  }, [onSelectTab])

  const closeTabKeys = useCallback((keys) => {
    if (!keys.length) return
    setSplitView(null)
    if (onCloseTabs) onCloseTabs(keys)
    else for (const k of keys) onCloseTab?.(k)
    dismissTabMenu()
  }, [onCloseTabs, onCloseTab, dismissTabMenu])

  const handleCloseDockTab = useCallback((tab) => {
    if (tab.kind === 'terminal') {
      setSplitView((prev) => (prev && splitIncludesTab(prev, tab.id) ? null : prev))
    }
    onCloseTab?.(tab.key)
  }, [onCloseTab])

  if (!visible) return null

  const showTerminalPane = activeItem?.kind === 'terminal'
  const activeTerm = terminalTabs?.activeTab
  const activeTerminalScope = [
    activeTerm?.contextName,
    activeTerm?.namespace || activeTerm?.shellNamespace,
  ].filter(Boolean).join(' / ')
  const activeLogStreamId = activeItem?.kind === 'log' ? activeItem.id : null
  const streamError = logStreams?.lastError || ''

  return (
    <div
      className={['console-dock', 'console-dock-ide', 'is-expanded', maximized ? 'is-maximized' : ''].filter(Boolean).join(' ')}
      aria-label="Panel"
    >
      {!maximized && (
        <div
          className="stream-resize-handle"
          role="separator"
          aria-orientation="horizontal"
          aria-label="Resize panel"
          onMouseDown={onResizeStart}
        />
      )}
      <div className="console-dock-body" style={maximized ? undefined : { height }}>
        <div className="console-dock-ide-header">
          <div className="console-dock-tabs" role="tablist" aria-label="Open terminals and log streams">
            {tabs.map((tab) => {
              const isActive = tab.key === activeKey
              const isSplitMember = tab.kind === 'terminal' && splitView && splitIncludesTab(splitView, tab.id)
              const tabTooltip = tab.kind === 'terminal'
                ? [
                  tab.title,
                  tab.shell,
                  tab.contextName,
                  tab.namespace,
                  termCount >= 2 ? 'Right-click to split' : '',
                ].filter(Boolean).join(' · ')
                : [
                  tab.title,
                  ...(tab.sources || []).map((s) => (s.namespace ? `${s.label} (${s.namespace})` : s.label)),
                  tab.state === 'live' ? 'Streaming' : tab.state === 'paused' ? 'Paused' : 'Stopped',
                ].filter(Boolean).join(' · ')

              return (
                <div
                  key={tab.key}
                  className={[
                    'console-dock-tab-item',
                    `is-${tab.kind}`,
                    isActive ? 'is-active' : '',
                    isSplitMember && !isActive ? 'is-split-member' : '',
                  ].filter(Boolean).join(' ')}
                  onContextMenu={(e) => {
                    if (tab.kind !== 'terminal') return
                    e.preventDefault()
                    setTabMenu({ tabId: tab.id, key: tab.key, x: e.clientX, y: e.clientY })
                  }}
                >
                  <button
                    type="button"
                    role="tab"
                    id={`console-dock-tab-${tab.key}`}
                    aria-selected={isActive}
                    className="console-dock-tab-select"
                    title={tabTooltip}
                    onClick={() => handleSelectDockTab(tab)}
                  >
                    {tab.kind === 'terminal' ? (
                      <TerminalGlyph size={13} />
                    ) : (
                      <StreamLogsGlyph size={13} />
                    )}
                    <span className="console-dock-tab-label">{tab.title}</span>
                    {tab.kind === 'terminal' && (tab.contextName || tab.shell) && (
                      <span className="terminal-tab-shell">{tab.contextName || tab.shell}</span>
                    )}
                    {tab.kind === 'terminal' && !tab.ready && !tab.error && (
                      <span className="terminal-tab-dot" aria-hidden="true" />
                    )}
                    {tab.kind === 'log' && (
                      <span
                        className={`dock-log-tab-dot is-${tab.state || 'idle'}`}
                        title={tab.state === 'live' ? 'Streaming live' : tab.state === 'paused' ? 'Paused' : 'Idle'}
                        aria-label={tab.state === 'live' ? 'Streaming live' : tab.state === 'paused' ? 'Paused' : 'Idle'}
                      />
                    )}
                  </button>
                  <button
                    type="button"
                    className="console-dock-tab-close"
                    aria-label={`Close ${tab.title}`}
                    title={`Close ${tab.title}`}
                    onClick={(e) => {
                      e.stopPropagation()
                      handleCloseDockTab(tab)
                    }}
                  >
                    ×
                  </button>
                </div>
              )
            })}

            <div className="console-dock-add-group" ref={addAnchorRef}>
              <button
                type="button"
                className="console-dock-add-btn"
                title="New terminal (⌘`)"
                aria-label="New terminal"
                onClick={() => {
                  setSplitView(null)
                  onNewTerminal?.()
                }}
              >
                +
              </button>
              <button
                type="button"
                className={`console-dock-add-menu-btn ${addMenuOpen ? 'is-open' : ''}`}
                title="New tab options"
                aria-label="New tab options"
                aria-expanded={addMenuOpen}
                aria-haspopup="menu"
                onClick={() => setAddMenuOpen((v) => !v)}
              >
                ▾
              </button>
              {addMenuOpen && (
                <div
                  ref={addMenuRef}
                  className="terminal-tab-menu console-dock-add-menu"
                  role="menu"
                  aria-label="New tab options"
                >
                  <button
                    type="button"
                    role="menuitem"
                    className="terminal-tab-menu-item"
                    onClick={() => {
                      setAddMenuOpen(false)
                      setSplitView(null)
                      onNewTerminal?.()
                    }}
                  >
                    <TerminalGlyph size={13} />
                    <span>New terminal</span>
                    <span className="console-dock-menu-shortcut muted">⌘`</span>
                  </button>
                  <button
                    type="button"
                    role="menuitem"
                    className="terminal-tab-menu-item"
                    disabled={!canStreamSelection}
                    title={
                      canStreamSelection
                        ? `Stream logs from ${selectionLabel || 'selection'}`
                        : 'Select a pod, workload or Service in Resources first'
                    }
                    onClick={() => {
                      if (!canStreamSelection) return
                      setAddMenuOpen(false)
                      onStreamSelection?.()
                    }}
                  >
                    <StreamLogsGlyph size={13} />
                    <span>Stream logs from selection</span>
                    <span className="console-dock-menu-shortcut muted">⌘⇧L</span>
                  </button>
                </div>
              )}
            </div>
          </div>

          <div className="console-dock-window-controls">
            {showTerminalPane && (
              <>
                {activeTerminalScope && (
                  <span className="console-dock-scope mono" title={activeTerminalScope}>
                    {activeTerminalScope}
                  </span>
                )}
                <TerminalActionButtons
                  onChangeShell={terminalProps.onChangeShell}
                  onFind={terminalActions?.find}
                  onRestart={terminalActions?.restart}
                  canFind={Boolean(terminalActions?.canFind)}
                  canRestart={Boolean(terminalActions?.canRestart)}
                />
                <span className="console-dock-control-divider" aria-hidden="true" />
              </>
            )}
            {maximized ? (
              <WindowBtn label="Restore" onClick={onRestore}>⛶</WindowBtn>
            ) : (
              <WindowBtn label="Maximize" onClick={onMaximize}>⛶</WindowBtn>
            )}
            <WindowBtn label="Hide" onClick={onMinimize}>▼</WindowBtn>
            <WindowBtn label="Close" onClick={onClose}>×</WindowBtn>
          </div>
        </div>

        {streamError && (
          <div className="console-dock-error-bar" role="alert">
            <span>{streamError}</span>
            <button type="button" className="btn btn-ghost btn-sm" onClick={() => logStreams?.clearError?.()}>
              Dismiss
            </button>
          </div>
        )}

        <div className="console-dock-pane-stack" aria-live="polite">
          {tabs.length === 0 ? (
            <div className="console-dock-empty" role="status">
              <strong>No tabs open</strong>
              <span className="muted">
                Open a cluster shell, or select a pod, workload or Service in Resources and click Stream logs (⌘⇧L).
              </span>
              <div className="console-dock-empty-actions">
                <button
                  type="button"
                  className="btn btn-primary btn-sm"
                  onClick={() => onNewTerminal?.()}
                >
                  New terminal
                </button>
                {canStreamSelection && (
                  <button
                    type="button"
                    className="btn btn-ghost btn-sm"
                    onClick={() => onStreamSelection?.()}
                  >
                    Stream logs from selection
                  </button>
                )}
              </div>
            </div>
          ) : (
            <>
              {logTabs.map((lt) => {
                const isActive = activeLogStreamId === lt.streamId
                const streamHandle = logStreams.getStream(lt.streamId)
                return (
                  <div
                    key={`log-pane-${lt.streamId}`}
                    className={['console-dock-pane-layer', isActive ? 'is-active' : ''].filter(Boolean).join(' ')}
                    role="tabpanel"
                    aria-labelledby={`console-dock-tab-log:${lt.streamId}`}
                    aria-hidden={!isActive}
                  >
                    <LiveStreamPanel
                      {...streamProps}
                      logStream={streamHandle}
                      activeTab={isActive}
                      ideDock
                      dockControls={null}
                    />
                  </div>
                )
              })}

              <div
                className={['console-dock-pane-layer', showTerminalPane ? 'is-active' : ''].filter(Boolean).join(' ')}
                role="tabpanel"
                aria-hidden={!showTerminalPane}
              >
                <TerminalPanel
                  {...terminalProps}
                  terminalTabs={terminalTabs}
                  splitView={splitView}
                  onSplitViewChange={setSplitView}
                  activePane={showTerminalPane}
                  onActionsChange={setTerminalActions}
                  embedded
                  ideDock
                  open={termCount > 0 || Boolean(terminalProps.launchRequest?.id)}
                  height={height}
                  panelState={PANEL_NORMAL}
                  dockControls={null}
                />
              </div>
            </>
          )}
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
              onClick={() => closeTabKeys([tabMenu.key])}
            >
              <span className="terminal-tab-menu-glyph" aria-hidden="true">×</span>
              <span>Close</span>
            </button>
            <button
              type="button"
              role="menuitem"
              className="terminal-tab-menu-item"
              disabled={tabs.length < 2}
              onClick={() => closeTabKeys(tabs.filter((t) => t.key !== tabMenu.key).map((t) => t.key))}
            >
              <span className="terminal-tab-menu-glyph" aria-hidden="true">⤫</span>
              <span>Close others</span>
            </button>
            <button
              type="button"
              role="menuitem"
              className="terminal-tab-menu-item"
              disabled={tabsRightOf(tabs, tabMenu.key).length === 0}
              onClick={() => closeTabKeys(tabsRightOf(tabs, tabMenu.key))}
            >
              <span className="terminal-tab-menu-glyph" aria-hidden="true">⇥</span>
              <span>Close tabs to the right</span>
            </button>
            <button
              type="button"
              role="menuitem"
              className="terminal-tab-menu-item"
              onClick={() => closeTabKeys(tabs.map((t) => t.key))}
            >
              <span className="terminal-tab-menu-glyph" aria-hidden="true">⨯</span>
              <span>Close all tabs</span>
            </button>
            <div className="terminal-tab-menu-sep" role="separator" />
            <button
              type="button"
              role="menuitem"
              className="terminal-tab-menu-item"
              disabled={termCount < 2}
              onClick={() => openSplit('horizontal', tabMenu.tabId)}
            >
              <SplitRightIcon />
              <span>Split right</span>
            </button>
            <button
              type="button"
              role="menuitem"
              className="terminal-tab-menu-item"
              disabled={termCount < 2}
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
      </div>
    </div>
  )
}

function WindowBtn({ label, onClick, children }) {
  return (
    <button type="button" className="stream-icon-btn" aria-label={label} title={label} onClick={onClick}>
      {children}
    </button>
  )
}
