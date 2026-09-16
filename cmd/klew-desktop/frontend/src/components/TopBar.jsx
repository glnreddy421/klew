import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { BrandWordmark, LogoMark } from './Logo'
import { WindowIsMaximised, WindowToggleMaximise, Environment, EventsOn } from '../../wailsjs/runtime/runtime'
import {
  isBlankInvestigationQuery,
  normalizeInvestigationQuery,
} from '../lib/investigationQuery'
import { ContextPopover, NamespacePopover } from './shell/ClusterNamespacePopover.jsx'
import { ClusterConnectionDot } from './shell/ClusterConnectionDot.jsx'
import { TimeWindowPopover } from './shell/TimeWindowPopover.jsx'
import { scopeSupportsInvestigate } from '../lib/browseScope.js'
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
  scope,
  scopeLocked = false,
  savedScopeLabel = '',
  investigationNs = '',
  onScopeChange,
  scopeVariant = 'browse',
  syncing,
  onSync,
  onContextChange,
  query,
  onQueryChange,
  onQueryClear,
  running,
  starting = false,
  activeQuery = '',
  onStart,
  onStop,
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
  reconnectBusy = false,
  connecting = false,
  connectingTarget = null,
}) {
  const inputRef = useRef(null)
  const [windowMaximized, setWindowMaximized] = useState(false)
  const [isMac, setIsMac] = useState(false)
  const [searchExpanded, setSearchExpanded] = useState(false)

  const q = normalizeInvestigationQuery(query)
  const canInvestigate = scopeSupportsInvestigate(scope) && !starting
  const queryChanged = running && !isBlankInvestigationQuery(q) && q !== normalizeInvestigationQuery(activeQuery)
  const contextLocked = running || starting

  useEffect(() => {
    WindowIsMaximised().then(setWindowMaximized).catch(() => {})
    Environment().then((env) => setIsMac(env.platform === 'darwin')).catch(() => {})
  }, [])

  useEffect(() => {
    const off = EventsOn('menu:focus-search', () => {
      setSearchExpanded(true)
      inputRef.current?.focus()
      inputRef.current?.select()
    })
    return () => off?.()
  }, [])

  useEffect(() => {
    function onKeyDown(e) {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setSearchExpanded(true)
        inputRef.current?.focus()
        inputRef.current?.select()
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
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
          <button
            type="button"
            className="topbar-nav-btn"
            onClick={() => onNavBack?.()}
            disabled={!canNavBack}
            title="Back"
            aria-label="Back"
          >
            <ChevronLeftIcon />
          </button>
          <button
            type="button"
            className="topbar-nav-btn"
            onClick={() => onNavForward?.()}
            disabled={!canNavForward}
            title="Forward"
            aria-label="Forward"
          >
            <ChevronRightIcon />
          </button>
          <button
            type="button"
            className="topbar-nav-btn"
            onClick={() => onNavHome?.()}
            title="Home — Resources"
            aria-label="Home — Resources"
          >
            <HomeIcon />
          </button>
        </div>
        <div className="topbar-scope">
          <ContextPopover
            cluster={cluster}
            disabled={starting}
            contextLocked={contextLocked}
            onContextChange={onContextChange}
          />
          <ClusterConnectionDot
            connection={connection}
            onReconnect={onReconnect}
            reconnectBusy={reconnectBusy}
            contextLabel={activeContextLabel(cluster, { connecting, connectingTarget })}
          />
        </div>
      </div>

      <div className="topbar-center">
        <div className="topbar-search-group">
          <NamespacePopover
            cluster={cluster}
            scope={scope}
            disabled={starting}
            locked={scopeLocked}
            lockedNamespace={investigationNs}
            savedScopeLabel={savedScopeLabel}
            onScopeChange={onScopeChange}
            variant={scopeVariant}
            compact
            inline
          />
          <form
            className={`topbar-search ${searchExpanded || query ? 'expanded' : ''}`}
            onSubmit={(e) => {
              e.preventDefault()
              onStart?.(e)
            }}
          >
            <div className="topbar-search-divider" aria-hidden="true" />
          <button
            type="button"
            className="topbar-search-toggle"
            onClick={() => {
              setSearchExpanded(true)
              inputRef.current?.focus()
            }}
            aria-label="Search"
            title="Search (⌘K)"
          >
            <SearchIcon />
          </button>
          <input
            ref={inputRef}
            className="topbar-search-input"
            value={query}
            onChange={(e) => onQueryChange(e.target.value)}
            onBlur={() => { if (!query) setSearchExpanded(false) }}
            placeholder="Search to investigate…"
            disabled={starting}
            aria-label="Search"
            spellCheck={false}
            autoComplete="off"
          />
          {!query && <kbd className="topbar-search-kbd" aria-hidden="true">⌘K</kbd>}
          {query && !starting && (
            <button type="button" className="topbar-search-clear" onClick={onQueryClear} aria-label="Clear">
              ×
            </button>
          )}
          <div className="topbar-search-actions">
            <button
              type="submit"
              className="topbar-search-investigate"
              disabled={!canInvestigate}
              title={investigateTitle({ running, starting, queryChanged, scope })}
            >
              {investigateLabel({ running, starting, queryChanged })}
            </button>
            <button
              type="button"
              className="topbar-search-new-window"
              onClick={() => onNewWindow?.()}
              title="Open a new window for another cluster (⌘N)"
              aria-label="New window"
            >
              <span className="topbar-search-new-window-label">New window</span>
              <kbd className="topbar-search-kbd" aria-hidden="true">⌘N</kbd>
            </button>
            {running && (
              <button
                type="button"
                className="topbar-search-stop-icon"
                onClick={onStop}
                disabled={starting}
                title="Stop investigation"
                aria-label="Stop investigation"
              >
                <StopIcon />
              </button>
            )}
          </div>
        </form>
        </div>
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
          onClick={onSync}
          disabled={syncing || starting}
          title={running ? 'Refresh investigation snapshot' : 'Sync kubeconfig'}
          aria-label={running ? 'Refresh investigation snapshot' : 'Sync kubeconfig'}
        >
          <SyncIcon spinning={syncing} />
        </button>
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

function investigateLabel({ running, starting, queryChanged }) {
  if (starting) return '…'
  if (running && queryChanged) return 'Re-run'
  if (running) return 'Restart'
  return 'Investigate'
}

function investigateTitle({ running, starting, queryChanged, scope }) {
  if (starting) return 'Starting investigation…'
  if (!scopeSupportsInvestigate(scope)) return 'Select one namespace to investigate'
  if (running && queryChanged) return 'Re-run with updated query in current scope'
  if (running) return 'Restart investigation in current scope'
  return 'Start investigation in current scope'
}

function StopIcon() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="currentColor" aria-hidden="true">
      <rect x="3" y="3" width="10" height="10" rx="1" />
    </svg>
  )
}

function SearchIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
      <circle cx="11" cy="11" r="7" /><path d="M20 20l-3-3" />
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
