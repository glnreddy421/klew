import { useEffect, useRef, useState } from 'react'
import { WindowIsMaximised, WindowToggleMaximise } from '../../wailsjs/runtime/runtime'
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
    ? new Date(cluster.syncedAt).toLocaleTimeString()
    : '—'
  const contextLabel = cluster.selectedContext || cluster.currentContext || '—'
  const [windowMaximized, setWindowMaximized] = useState(false)

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

  return (
    <header className="topbar">
      <div className="topbar-cluster">
        <SelectChip
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
        <SelectChip
          label="Namespace"
          value={cluster.selectedNamespace || ''}
          options={namespaces.map((ns) => ({ value: ns, label: ns }))}
          disabled={running || starting || namespaces.length === 0}
          onChange={onNamespaceChange}
        />
        <button
          type="button"
          className="btn btn-window"
          onClick={() => onNewWindow?.()}
          title="Open a new window for another cluster (⌘N)"
          aria-label="Open new window"
        >
          <WindowIconClone />
        </button>
      </div>

      <form className="topbar-search" onSubmit={onStart}>
        <div className="search-field">
          <svg className="search-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
            <circle cx="11" cy="11" r="7" /><path d="M20 20l-3-3" />
          </svg>
          <input
            ref={inputRef}
            className="search-input"
            value={query}
            onChange={(e) => onQueryChange(e.target.value)}
            placeholder="Search (empty = all in namespace)"
            disabled={starting}
            aria-label="Search workloads"
          />
          <div className="search-field-end">
            {query && !starting && !isBlankInvestigationQuery(query) ? (
              <button type="button" className="search-clear" onClick={onQueryClear} aria-label="Clear search">
                ×
              </button>
            ) : (
              <kbd className="search-kbd" aria-hidden="true">⌘K</kbd>
            )}
          </div>
        </div>
        <button
          type="submit"
          className="btn btn-primary btn-investigate"
          disabled={!canInvestigate}
          aria-busy={starting}
        >
          {investigateLabel}
        </button>
        {running && (
          <button
            type="button"
            className="btn btn-ghost btn-investigate"
            onClick={onStop}
            disabled={starting}
          >
            Stop
          </button>
        )}
      </form>

      <div className="topbar-status">
        <button
          type="button"
          className="btn btn-window"
          onClick={onToggleWindowMaximize}
          title={windowMaximized ? 'Restore window' : 'Maximize window'}
          aria-label={windowMaximized ? 'Restore window' : 'Maximize window'}
        >
          {windowMaximized ? <WindowIconRestore /> : <WindowIconMaximize />}
        </button>
        <button
          type="button"
          className="btn btn-sync"
          onClick={onSync}
          disabled={syncing || running || starting}
          title="Reload kubeconfig and refresh namespaces (⌘R)"
        >
          <SyncIcon spinning={syncing} />
          {syncing ? 'Syncing…' : 'Sync now'}
        </button>
        <span className={`status-dot ${cluster.syncError ? 'warn' : 'ok'}`} title={cluster.syncError || 'Connected'} />
        <span className="status-context">{contextLabel}</span>
        <span className="status-sync" title={cluster.syncError || cluster.kubeconfigPath}>
          {cluster.syncError ? 'Sync failed' : `Synced ${syncedLabel}`}
        </span>
      </div>
    </header>
  )
}

function SelectChip({ label, value, options, disabled, onChange, title }) {
  return (
    <div className={`select-chip ${disabled ? 'disabled' : ''}`} title={title}>
      <span className="chip-label">{label}</span>
      <select
        key={value || label}
        className="chip-select"
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
    </div>
  )
}

function SyncIcon({ spinning }) {
  return (
    <svg className={`sync-icon ${spinning ? 'spin' : ''}`} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M21 12a9 9 0 11-2.64-6.36" />
      <path d="M21 3v6h-6" />
    </svg>
  )
}

function WindowIconClone() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <rect x="4.5" y="4.5" width="9" height="9" rx="1.5" />
      <path d="M2.5 11.5V3.5A1 1 0 013.5 2.5h8" />
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
      <path d="M2.5 5.5v8H10.5" />
    </svg>
  )
}
