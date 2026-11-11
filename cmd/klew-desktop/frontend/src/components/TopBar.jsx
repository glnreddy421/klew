import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { BrandWordmark, LogoMark } from './Logo'
import { TerminalGlyph } from './DockLauncher'
import { WindowIsMaximised, WindowToggleMaximise, Environment } from '../../wailsjs/runtime/runtime'
import { ContextPopover } from './shell/ClusterNamespacePopover.jsx'
import { ClusterConnectionDot, connectionTone } from './shell/ClusterConnectionDot.jsx'
import { ContextTabs } from './shell/ContextTabs.jsx'
import { EventRecordingDot } from './shell/EventRecordingDot.jsx'
import { HoverTip } from './HoverTip.jsx'
import { TimeWindowPopover } from './shell/TimeWindowPopover.jsx'
import { activeContextLabel } from '../lib/clusterConnection.js'
import {
  applyTopbarScale,
  bumpTopbarScale,
  normalizeTopbarScale,
  TOPBAR_SCALE_MAX,
  TOPBAR_SCALE_MIN,
  TOPBAR_SCALE_STEP,
} from '../lib/topbarScale.js'

export function TopBar({
  cluster,
  syncing,
  onSync,
  onContextChange,
  contextTabs = [],
  onSelectContextTab,
  onCloseContextTab,
  /** Context tabs are for working in a cluster — hide on Home (Contexts). */
  showContextTabs = true,
  running,
  starting = false,
  onNewWindow,
  onOpenSettings,
  onOpenHelp,
  onNavBack,
  onNavForward,
  onNavHome,
  canNavBack = false,
  canNavForward = false,
  prefs,
  onPrefsChange,
  live,
  connection = null,
  onReconnect,
  onDisconnect,
  reconnectBusy = false,
  monitoringPaused = false,
  defaultContext = '',
  favoriteContexts = [],
  contextAliases = {},
  onSetDefaultContext,
  onToggleFavoriteContext,
  connecting = false,
  connectingTarget = null,
  panelOpen = false,
  panelRunning = false,
  onTogglePanel,
}) {
  const [windowMaximized, setWindowMaximized] = useState(false)
  const [isMac, setIsMac] = useState(false)
  const contextLocked = running || starting

  useEffect(() => {
    WindowIsMaximised().then(setWindowMaximized).catch(() => {})
    Environment().then((env) => setIsMac(env.platform === 'darwin')).catch(() => {})
  }, [])

  async function onToggleWindowMaximize() {
    WindowToggleMaximise()
    try {
      setWindowMaximized(await WindowIsMaximised())
    } catch {
      setWindowMaximized((v) => !v)
    }
  }

  const topbarZoom = normalizeTopbarScale(prefs?.topbarScale ?? 100) / 100

  function setTopbarScale(next) {
    const normalized = applyTopbarScale(next)
    onPrefsChange?.({ topbarScale: normalized })
  }

  return (
    <header className="topbar topbar-compact">
      <div className="topbar-scaled" style={{ zoom: topbarZoom }}>
      <div className="topbar-left">
        <div className="topbar-brand">
          <LogoMark />
          <BrandWordmark variant="topbar" />
        </div>
        <div className="topbar-nav-history" role="navigation" aria-label="History">
          <HoverTip label="Back" detail="Previous page in this window.">
            <button
              type="button"
              className="topbar-nav-btn"
              onClick={() => onNavBack?.()}
              disabled={!canNavBack}
              aria-label="Back"
            >
              <ChevronLeftIcon />
            </button>
          </HoverTip>
          <HoverTip label="Forward" detail="Next page in this window.">
            <button
              type="button"
              className="topbar-nav-btn"
              onClick={() => onNavForward?.()}
              disabled={!canNavForward}
              aria-label="Forward"
            >
              <ChevronRightIcon />
            </button>
          </HoverTip>
          <HoverTip label="Contexts" detail="Pick a cluster. Hide context tabs and return to the start page.">
            <button
              type="button"
              className="topbar-nav-btn"
              onClick={() => onNavHome?.()}
              aria-label="Contexts"
            >
              <HomeIcon />
            </button>
          </HoverTip>
        </div>
        {showContextTabs && (
          <div className="topbar-scope">
            <ContextTabs
              tabs={contextTabs}
              // The clicked tab becomes active at once, while the cluster connects.
              active={connecting && connectingTarget?.kind === 'context' && connectingTarget.name
                ? connectingTarget.name
                : (cluster?.selectedContext || cluster?.currentContext || '')}
              statusTone={connection && connection.phase !== 'idle' ? connectionTone(connection.phase) : 'muted'}
              statusTitle={connection?.phase === 'paused' ? 'monitoring paused' : (connection?.phase || '')}
              locked={contextLocked}
              contextAliases={contextAliases}
              onSelect={onSelectContextTab}
              onClose={onCloseContextTab}
            >
              <ContextPopover
                cluster={cluster}
                disabled={starting}
                contextLocked={contextLocked}
                onContextChange={onContextChange}
                connection={connection}
                onReconnect={onReconnect}
                onDisconnect={onDisconnect}
                reconnectBusy={reconnectBusy}
                monitoringPaused={monitoringPaused}
                defaultContext={defaultContext}
                favoriteContexts={favoriteContexts}
                contextAliases={contextAliases}
                onSetDefaultContext={onSetDefaultContext}
                onToggleFavoriteContext={onToggleFavoriteContext}
                compact={contextTabs.length > 0}
              />
            </ContextTabs>
            {contextTabs.length === 0 && (
              <ClusterConnectionDot
                connection={connection}
                onReconnect={onReconnect}
                reconnectBusy={reconnectBusy}
                contextLabel={activeContextLabel(cluster, { connecting, connectingTarget })}
              />
            )}
            <EventRecordingDot contextName={cluster?.selectedContext || cluster?.currentContext || ''} />
          </div>
        )}
      </div>

      </div>

      <div className="topbar-right">
        <TopbarScaleControl
          scale={prefs?.topbarScale ?? 100}
          onChange={setTopbarScale}
        />
        {running && (
          <TimeWindowPopover
            live={live}
            windowMin={prefs?.windowMin || 15}
            autoRefresh={prefs?.autoRefresh}
            running={running}
            onWindowChange={(m) => onPrefsChange?.({ windowMin: m })}
            onAutoRefreshChange={(v) => onPrefsChange?.({ autoRefresh: v })}
          />
        )}
        <button
          type="button"
          className="topbar-icon-btn"
          onClick={() => onNewWindow?.()}
          title="Open a new window for another cluster (⌘N)"
          aria-label="New window"
        >
          <NewWindowIcon />
        </button>
        <button
          type="button"
          className="topbar-icon-btn"
          onClick={onSync}
          disabled={syncing || starting}
          title={running ? 'Refresh investigation snapshot' : 'Sync kubeconfig'}
          aria-label={running ? 'Refresh investigation snapshot' : 'Sync kubeconfig'}
        >
          <SyncIcon spinning={syncing} />
        </button>
        {onTogglePanel && (
          <button
            type="button"
            className={`topbar-icon-btn topbar-panel-toggle ${panelOpen ? 'is-active' : ''}`}
            onClick={() => onTogglePanel()}
            aria-pressed={panelOpen}
            title={panelOpen ? 'Hide terminal (⌘`)' : panelRunning ? 'Show terminal — sessions running (⌘`)' : 'Terminal (⌘`)'}
            aria-label={panelOpen ? 'Hide terminal' : 'Show terminal'}
          >
            <TerminalGlyph size={15} />
            {panelRunning && !panelOpen && <span className="topbar-panel-dot" aria-hidden="true" />}
          </button>
        )}
        <button
          type="button"
          className="topbar-icon-btn"
          onClick={() => onOpenSettings?.()}
          title="Settings"
          aria-label="Settings"
        >
          <SettingsIcon />
        </button>
        <button
          type="button"
          className="topbar-icon-btn"
          onClick={() => onOpenHelp?.()}
          title="Help"
          aria-label="Help"
        >
          <HelpIcon />
        </button>
        {!isMac && (
          <button
            type="button"
            className="topbar-icon-btn"
            onClick={onToggleWindowMaximize}
            title={windowMaximized ? 'Restore' : 'Maximize'}
            aria-label={windowMaximized ? 'Restore window' : 'Maximize window'}
          >
            {windowMaximized ? <WindowIconRestore /> : <WindowIconMaximize />}
          </button>
        )}
      </div>
    </header>
  )
}

function TopbarScaleControl({ scale, onChange }) {
  const [open, setOpen] = useState(false)
  const [popoverPos, setPopoverPos] = useState(null)
  const anchorRef = useRef(null)
  const popoverRef = useRef(null)
  const normalized = normalizeTopbarScale(scale)

  const updatePopoverPos = () => {
    const el = anchorRef.current
    if (!el) return
    const rect = el.getBoundingClientRect()
    const width = 148
    setPopoverPos({
      top: rect.bottom + 6,
      left: Math.max(8, rect.right - width),
    })
  }

  useLayoutEffect(() => {
    if (!open) {
      setPopoverPos(null)
      return undefined
    }
    updatePopoverPos()
    window.addEventListener('resize', updatePopoverPos)
    window.addEventListener('scroll', updatePopoverPos, true)
    return () => {
      window.removeEventListener('resize', updatePopoverPos)
      window.removeEventListener('scroll', updatePopoverPos, true)
    }
  }, [open, normalized])

  useEffect(() => {
    if (!open) return undefined
    function onDoc(e) {
      if (anchorRef.current?.contains(e.target)) return
      if (popoverRef.current?.contains(e.target)) return
      setOpen(false)
    }
    function onKey(e) {
      if (e.key === 'Escape') setOpen(false)
    }
    const timer = window.setTimeout(() => {
      document.addEventListener('mousedown', onDoc)
      document.addEventListener('keydown', onKey)
    }, 0)
    return () => {
      window.clearTimeout(timer)
      document.removeEventListener('mousedown', onDoc)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  const popover = open && popoverPos ? createPortal(
    <div
      ref={popoverRef}
      className="topbar-scale-popover topbar-scale-popover-fixed"
      role="dialog"
      aria-label="Top bar size"
      style={{
        position: 'fixed',
        top: popoverPos.top,
        left: popoverPos.left,
        zIndex: 10000,
      }}
    >
      <p className="topbar-scale-popover-title">Top bar size</p>
      <div className="topbar-scale-popover-actions">
        <button
          type="button"
          className="topbar-scale-step-btn"
          onClick={() => onChange?.(bumpTopbarScale(normalized, -TOPBAR_SCALE_STEP))}
          disabled={normalized <= TOPBAR_SCALE_MIN}
          aria-label="Decrease top bar size"
        >
          <ZoomOutIcon />
        </button>
        <span className="topbar-scale-popover-value">{normalized}%</span>
        <button
          type="button"
          className="topbar-scale-step-btn"
          onClick={() => onChange?.(bumpTopbarScale(normalized, TOPBAR_SCALE_STEP))}
          disabled={normalized >= TOPBAR_SCALE_MAX}
          aria-label="Increase top bar size"
        >
          <ZoomInIcon />
        </button>
      </div>
      {normalized !== 100 && (
        <button
          type="button"
          className="topbar-scale-reset-btn"
          onClick={() => onChange?.(100)}
        >
          Reset to 100%
        </button>
      )}
    </div>,
    document.body,
  ) : null

  return (
    <div className="topbar-scale-anchor" ref={anchorRef}>
      <button
        type="button"
        className={`topbar-icon-btn ${open ? 'is-open' : ''}`.trim()}
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        aria-haspopup="dialog"
        title={`Top bar size · ${normalized}%`}
        aria-label={`Top bar size ${normalized} percent`}
      >
        <TopbarSizeIcon />
      </button>
      {popover}
    </div>
  )
}

function TopbarSizeIcon() {
  return (
    <svg viewBox="0 0 16 16" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true">
      <path d="M2.5 12h11" strokeLinecap="round" />
      <path d="M4 8.5h8" strokeLinecap="round" />
      <path d="M5.5 5h5" strokeLinecap="round" />
    </svg>
  )
}

function ZoomOutIcon() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <circle cx="7" cy="7" r="4.25" />
      <path d="M10.2 10.2 13 13" strokeLinecap="round" />
      <path d="M5 7h4" strokeLinecap="round" />
    </svg>
  )
}

function ZoomInIcon() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <circle cx="7" cy="7" r="4.25" />
      <path d="M10.2 10.2 13 13" strokeLinecap="round" />
      <path d="M5 7h4M7 5v4" strokeLinecap="round" />
    </svg>
  )
}

function SyncIcon({ spinning }) {
  return (
    <svg className={`sync-icon ${spinning ? 'spin' : ''}`} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75">
      <path d="M21 12a9 9 0 11-2.64-6.36" strokeLinecap="round" />
      <path d="M21 3v6h-6" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

function SettingsIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M12 15a3 3 0 100-6 3 3 0 000 6z" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 010 2.83 2 2 0 01-2.83 0l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 01-2.83 0 2 2 0 010-2.83l.06-.06A1.65 1.65 0 004.68 15a1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 012.83-2.83l.06.06A1.65 1.65 0 009 4.68a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 012.83 2.83l-.06.06A1.65 1.65 0 0019.4 9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

function HelpIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <circle cx="12" cy="12" r="9" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M9.1 9a3 3 0 015.8 1c0 2-3 2-3 4" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M12 17h.01" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

function WindowIconMaximize() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <rect x="2.5" y="2.5" width="11" height="11" rx="1.5" />
    </svg>
  )
}

function WindowIconRestore() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <rect x="5" y="2" width="8.5" height="8.5" rx="1" />
      <path d="M2.5 5.5v8H10.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

function ChevronLeftIcon() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M10 3L5 8l5 5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

function ChevronRightIcon() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M6 3l5 5-5 5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

function HomeIcon() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M2.5 6.5L8 2l5.5 4.5V13a1 1 0 01-1 1H3.5a1 1 0 01-1-1V6.5z" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M6.5 14V9h3v5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

function NewWindowIcon() {
  return (
    <svg viewBox="0 0 16 16" width="15" height="15" fill="none" stroke="currentColor" strokeWidth="1.4" aria-hidden="true">
      <rect x="2.5" y="4.5" width="9" height="9" rx="1.5" />
      <path d="M5.5 2.5h7a1 1 0 011 1v7" strokeLinecap="round" />
    </svg>
  )
}

