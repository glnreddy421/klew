import { useEffect, useRef, useState } from 'react'
import { WindowIsMaximised, WindowToggleMaximise, Environment, EventsOn } from '../../wailsjs/runtime/runtime'
import {
  isBlankInvestigationQuery,
  normalizeInvestigationQuery,
} from '../lib/investigationQuery'

export function TopBar({
  cluster,
  syncing,
  onSync,
  onContextChange,
  onNamespaceChange,
  onNewWindow,
  query,
  onQueryChange,
  onQueryClear,
  running,
  starting = false,
  activeQuery = '',
  onStart,
  onStop,
}) {
  const inputRef = useRef(null)
  const contexts = cluster.contexts || []
  const namespaces = cluster.namespaces || []
  const syncedLabel = cluster.syncedAt
    ? new Date(cluster.syncedAt).toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' })
    : null
  const [windowMaximized, setWindowMaximized] = useState(false)
  const [isMac, setIsMac] = useState(false)

  const q = normalizeInvestigationQuery(query)
  const canInvestigate = Boolean(cluster.selectedNamespace) && !starting
  const queryChanged = running && !isBlankInvestigationQuery(q) && q !== normalizeInvestigationQuery(activeQuery)
  const investigateLabel = starting
    ? 'Starting…'
    : queryChanged
      ? 'Re-investigate'
      : running
        ? 'Restart'
        : 'Investigate'
  const contextLocked = running || starting
  const contextHint = contextLocked
    ? 'This window stays on the current cluster. Pick another context to open it in a new window.'
    : 'Kubernetes context for this window'

  useEffect(() => {
    WindowIsMaximised().then(setWindowMaximized).catch(() => {})
    Environment().then((env) => setIsMac(env.platform === 'darwin')).catch(() => {})
  }, [])

  useEffect(() => {
    const off = EventsOn('menu:focus-search', () => {
      inputRef.current?.focus()
      inputRef.current?.select()
    })
    return () => off?.()
  }, [])

  async function onToggleWindowMaximize() {
    WindowToggleMaximise()
    try {
      setWindowMaximized(await WindowIsMaximised())
    } catch {
      setWindowMaximized((v) => !v)
    }
  }

  useEffect(() => {
    function onKeyDown(e) {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        inputRef.current?.focus()
        inputRef.current?.select()
      }
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'n') {
        e.preventDefault()
        onNewWindow?.()
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [onNewWindow])

  const connectionTitle = cluster.syncError || cluster.kubeconfigPath || 'Cluster connection'

  return (
    <header className="topbar">
      <div className="topbar-scope">
        <ScopeSelect
          label="Context"
          value={cluster.selectedContext || cluster.currentContext || ''}
          options={contexts.map((c) => ({
            value: c.name,
            label: c.name,
            hint: c.cluster,
          }))}
          disabled={starting}
          title={contextHint}
          onChange={onContextChange}
        />
        <ScopeSelect
          label="Namespace"
          value={cluster.selectedNamespace || ''}
          options={namespaces.map((ns) => ({ value: ns, label: ns }))}
          disabled={running || starting || namespaces.length === 0}
          onChange={onNamespaceChange}
        />
        <button
          type="button"
          className="topbar-icon-btn"
          onClick={() => onNewWindow?.()}
          title="New window (⌘N)"
          aria-label="Open new window"
        >
          <WindowIconClone />
        </button>
      </div>

      <form className="topbar-command" onSubmit={onStart}>
        <div className="command-input">
          <svg className="command-input-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
            <circle cx="11" cy="11" r="7" /><path d="M20 20l-3-3" />
          </svg>
          <input
            ref={inputRef}
            className="command-input-field"
            value={query}
            onChange={(e) => onQueryChange(e.target.value)}
            placeholder="Search workloads"
            disabled={starting}
            aria-label="Search workloads"
            spellCheck={false}
            autoComplete="off"
          />
          <div className="command-input-trail">
            {query && !starting && !isBlankInvestigationQuery(query) ? (
              <button type="button" className="command-input-clear" onClick={onQueryClear} aria-label="Clear search">
                <svg viewBox="0 0 16 16" width="12" height="12" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
                  <path d="M4 4l8 8M12 4l-8 8" strokeLinecap="round" />
                </svg>
              </button>
            ) : (
              <kbd className="command-input-kbd" aria-hidden="true">⌘K</kbd>
            )}
          </div>
        </div>
        <div className="command-actions">
          <button
            type="submit"
            className="btn btn-primary btn-command"
            disabled={!canInvestigate}
            aria-busy={starting}
          >
            {investigateLabel}
          </button>
          {running && (
            <button
              type="button"
              className="btn btn-quiet btn-command"
              onClick={onStop}
              disabled={starting}
            >
              Stop
            </button>
          )}
        </div>
      </form>

      <div className="topbar-meta">
        {!isMac && (
          <button
            type="button"
            className="topbar-icon-btn"
            onClick={onToggleWindowMaximize}
            title={windowMaximized ? 'Restore window' : 'Maximize window'}
            aria-label={windowMaximized ? 'Restore window' : 'Maximize window'}
          >
            {windowMaximized ? <WindowIconRestore /> : <WindowIconMaximize />}
          </button>
        )}
        <button
          type="button"
          className="topbar-icon-btn"
          onClick={onSync}
          disabled={syncing || running || starting}
          title="Sync kubeconfig (⌘R)"
          aria-label={syncing ? 'Syncing' : 'Sync kubeconfig'}
        >
          <SyncIcon spinning={syncing} />
        </button>
        <div className="topbar-connection" title={connectionTitle}>
          <span className={`connection-dot ${cluster.syncError ? 'warn' : 'ok'}`} aria-hidden="true" />
          <span className="connection-copy">
            {cluster.syncError ? (
              <span className="connection-status connection-status--warn">Sync failed</span>
            ) : (
              <>
                <span className="connection-status">Connected</span>
                {syncedLabel && (
                  <span className="connection-time">{syncedLabel}</span>
                )}
              </>
            )}
          </span>
        </div>
      </div>
    </header>
  )
}

function ScopeSelect({ label, value, options, disabled, onChange, title }) {
  return (
    <label className={`scope-field ${disabled ? 'is-disabled' : ''}`} title={title}>
      <span className="scope-field-label">{label}</span>
      <span className="scope-field-control">
        <select
          key={value || label}
          className="scope-field-input"
          value={value}
          disabled={disabled || options.length === 0}
          onChange={(e) => onChange(e.target.value)}
        >
          {options.length === 0 && <option value="">—</option>}
          {options.map((o) => (
            <option key={o.value} value={o.value} title={o.hint || o.label}>
              {o.label}
            </option>
          ))}
        </select>
        <ChevronIcon />
      </span>
    </label>
  )
}

function ChevronIcon() {
  return (
    <svg className="scope-field-chevron" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M4 6l4 4 4-4" strokeLinecap="round" strokeLinejoin="round" />
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

function WindowIconClone() {
  return (
    <svg viewBox="0 0 16 16" width="15" height="15" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <rect x="4.5" y="4.5" width="9" height="9" rx="1.5" />
      <path d="M2.5 11.5V3.5A1 1 0 013.5 2.5h8" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

function WindowIconMaximize() {
  return (
    <svg viewBox="0 0 16 16" width="15" height="15" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <rect x="2.5" y="2.5" width="11" height="11" rx="1.5" />
    </svg>
  )
}

function WindowIconRestore() {
  return (
    <svg viewBox="0 0 16 16" width="15" height="15" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <rect x="5" y="2" width="8.5" height="8.5" rx="1" />
      <path d="M2.5 5.5v8H10.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}
