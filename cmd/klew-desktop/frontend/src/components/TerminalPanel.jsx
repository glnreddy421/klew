import { PANEL_CLOSED, PANEL_MAXIMIZED, PANEL_MINIMIZED, PANEL_NORMAL } from '../hooks/useStreamPanel'
import { useEffect, useRef } from 'react'
import { useTerminalTabs } from '../hooks/useTerminalTabs'
import { TerminalTabPane } from './TerminalTabPane'

import { normalizeTerminalAppearance, terminalAppearanceStyle } from '../lib/terminalAppearance'

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
  embedded = false,
}) {
  const isWorkspace = layout === 'workspace'
  const isOpen = open && panelState !== PANEL_CLOSED

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
  })

  const shellRestartRef = useRef(shellRestartToken)
  useEffect(() => {
    if (!isOpen || !shellRestartToken || shellRestartRef.current === shellRestartToken) return
    shellRestartRef.current = shellRestartToken
    if (tabs.length > 0) restartAllTabs()
  }, [isOpen, shellRestartToken, tabs.length, restartAllTabs])

  if (!open || panelState === PANEL_CLOSED) {
    return null
  }

  const maximized = !embedded && panelState === PANEL_MAXIMIZED
  const minimized = !embedded && panelState === PANEL_MINIMIZED
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
                className={`terminal-tab ${tab.id === activeId ? 'is-active' : ''} ${tab.ready ? '' : 'is-pending'}`}
              >
                <button
                  type="button"
                  className="terminal-tab-select"
                  role="tab"
                  aria-selected={tab.id === activeId}
                  title={[
                    tab.title,
                    tab.shell,
                    tab.contextName,
                    tab.namespace,
                  ].filter(Boolean).join(' · ')}
                  onClick={() => selectTab(tab.id)}
                >
                  <span className="terminal-tab-label">{tab.title}</span>
                  {tab.shell && (
                    <span className="terminal-tab-shell">{tab.shell}</span>
                  )}
                  {!tab.ready && !tab.error && <span className="terminal-tab-dot" aria-hidden="true" />}
                </button>
                {tabs.length > 1 && (
                  <button
                    type="button"
                    className="terminal-tab-close"
                    aria-label={`Close ${tab.title}`}
                    onClick={(e) => {
                      e.stopPropagation()
                      closeTab(tab.id)
                    }}
                  >
                    ×
                  </button>
                )}
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
            {activeTab?.error && (
              <div className="terminal-error" role="alert">{activeTab.error}</div>
            )}
            <div className="terminal-host terminal-host-tabs">
              {tabs.map((tab) => (
                <TerminalTabPane
                  key={tab.id}
                  tab={tab}
                  active={tab.id === activeId}
                  open={isOpen && !minimized}
                  cluster={cluster}
                  shellPref={shellPref}
                  appearance={appearanceId}
                  onStateChange={handleTabState}
                />
              ))}
            </div>
          </div>
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
