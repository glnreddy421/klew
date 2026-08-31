import { useEffect, useRef, useState } from 'react'
import { LogoMark } from './Logo'
import { WindowIsMaximised, WindowToggleMaximise, Environment, EventsOn } from '../../wailsjs/runtime/runtime'
import {
  isBlankInvestigationQuery,
  normalizeInvestigationQuery,
} from '../lib/investigationQuery'
import { ContextPopover, NamespacePopover } from './shell/ClusterNamespacePopover.jsx'
import { TimeWindowPopover } from './shell/TimeWindowPopover.jsx'

export function TopBar({
  cluster,
  syncing,
  onSync,
  onContextChange,
  onNamespaceChange,
  query,
  onQueryChange,
  onQueryClear,
  running,
  starting = false,
  activeQuery = '',
  onStart,
  onStop,
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
}) {
  const inputRef = useRef(null)
  const [windowMaximized, setWindowMaximized] = useState(false)
  const [isMac, setIsMac] = useState(false)
  const [searchExpanded, setSearchExpanded] = useState(false)

  const q = normalizeInvestigationQuery(query)
  const canInvestigate = Boolean(cluster.selectedNamespace) && !starting
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

  return (
    <header className="topbar topbar-compact">
      <div className="topbar-left">
        <div className="topbar-brand">
          <LogoMark />
          <span className="topbar-brand-name">KLEW</span>
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
            title="Home — Overview"
            aria-label="Home — Overview"
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
          <NamespacePopover
            cluster={cluster}
            disabled={running || starting}
            onNamespaceChange={onNamespaceChange}
          />
        </div>
      </div>

      <div className="topbar-center">
        <form
          className={`topbar-search ${searchExpanded || query ? 'expanded' : ''}`}
          onSubmit={(e) => {
            e.preventDefault()
            onStart?.(e)
          }}
        >
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
            placeholder="Search resources, signals…"
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
              title={investigateTitle({ running, starting, queryChanged })}
            >
              {investigateLabel({ running, starting, queryChanged })}
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

      <div className="topbar-right">
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
          disabled={syncing || running || starting}
          title="Refresh investigation"
          aria-label="Refresh investigation"
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

function investigateLabel({ running, starting, queryChanged }) {
  if (starting) return '…'
  if (running && queryChanged) return 'Re-run'
  if (running) return 'Restart'
  return 'Investigate'
}

function investigateTitle({ running, starting, queryChanged }) {
  if (starting) return 'Starting investigation…'
  if (running && queryChanged) return 'Re-run with updated query'
  if (running) return 'Restart investigation'
  return 'Start investigation'
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
