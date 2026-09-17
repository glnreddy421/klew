import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { filterBySubstring } from '../../lib/scopeSearch.js'
import { hasClusterContexts } from '../../lib/clusterIdentity.js'
import { ClusterCloudIcon } from './ClusterCloudIcon.jsx'
import {
  allBrowseScope,
  browseScopeLabel,
  multiBrowseScope,
  normalizeBrowseScope,
  normalizeInvestigationScope,
  singleBrowseScope,
} from '../../lib/browseScope.js'

function usePopoverDismiss(open, setOpen, anchorRef, panelRef) {
  useEffect(() => {
    if (!open) return
    function onDoc(e) {
      const target = e.target
      if (anchorRef.current?.contains(target)) return
      if (panelRef.current?.contains(target)) return
      setOpen(false)
    }
    function onKey(e) {
      if (e.key === 'Escape') setOpen(false)
    }
    document.addEventListener('mousedown', onDoc)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDoc)
      document.removeEventListener('keydown', onKey)
    }
  }, [open, setOpen, anchorRef, panelRef])
}

function usePopover() {
  const [open, setOpen] = useState(false)
  const anchorRef = useRef(null)
  const panelRef = useRef(null)
  usePopoverDismiss(open, setOpen, anchorRef, panelRef)
  return { open, setOpen, anchorRef, panelRef }
}

function usePopoverPosition(open, anchorRef, panelRef) {
  const [position, setPosition] = useState(null)

  useLayoutEffect(() => {
    if (!open) {
      setPosition(null)
      return undefined
    }

    function update() {
      const anchor = anchorRef.current
      if (!anchor) return
      const rect = anchor.getBoundingClientRect()
      const gap = 4
      const minWidth = Math.max(rect.width, 240)
      let left = rect.left
      const maxLeft = window.innerWidth - minWidth - 8
      if (left > maxLeft) left = Math.max(8, maxLeft)

      const panelHeight = panelRef.current?.offsetHeight || 280
      let top = rect.bottom + gap
      if (top + panelHeight > window.innerHeight - 8) {
        top = Math.max(8, rect.top - gap - panelHeight)
      }

      setPosition({ top, left, minWidth })
    }

    update()
    const frame = window.requestAnimationFrame(update)
    window.addEventListener('resize', update)
    window.addEventListener('scroll', update, true)
    return () => {
      window.cancelAnimationFrame(frame)
      window.removeEventListener('resize', update)
      window.removeEventListener('scroll', update, true)
    }
  }, [open, anchorRef, panelRef])

  return position
}

function PopoverPortal({ open, anchorRef, panelRef, className, role, ariaLabel, children }) {
  const position = usePopoverPosition(open, anchorRef, panelRef)
  if (!open || !position) return null

  return createPortal(
    <div
      ref={panelRef}
      className={`shell-popover shell-popover-portal ${className || ''}`.trim()}
      role={role}
      aria-label={ariaLabel}
      style={{
        top: `${position.top}px`,
        left: `${position.left}px`,
        minWidth: `${position.minWidth}px`,
      }}
    >
      {children}
    </div>,
    document.body,
  )
}

function usePopoverFilter(open) {
  const [filter, setFilter] = useState('')
  const inputRef = useRef(null)

  useEffect(() => {
    if (!open) return
    setFilter('')
    const t = window.setTimeout(() => inputRef.current?.focus(), 0)
    return () => window.clearTimeout(t)
  }, [open])

  return { filter, setFilter, inputRef }
}

function PopoverSearch({ value, onChange, inputRef, placeholder, ariaLabel, onKeyDown }) {
  return (
    <div className="shell-popover-search">
      <input
        ref={inputRef}
        type="search"
        className="shell-popover-search-input"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={onKeyDown}
        placeholder={placeholder}
        spellCheck={false}
        autoComplete="off"
        aria-label={ariaLabel}
      />
    </div>
  )
}

export function ContextPopover({
  cluster,
  disabled,
  contextLocked,
  onContextChange,
  connection = null,
  onReconnect,
  onDisconnect,
  reconnectBusy = false,
  monitoringPaused = false,
  defaultContext = '',
  onSetDefaultContext,
}) {
  const { open, setOpen, anchorRef, panelRef } = usePopover()
  const { filter, setFilter, inputRef } = usePopoverFilter(open)
  const contexts = cluster.contexts || []
  const ctx = cluster.selectedContext || cluster.currentContext || (contexts[0]?.name || 'Select context')
  const syncError = String(cluster?.syncError || '').trim()
  const connecting = connection?.phase === 'connecting' || reconnectBusy
  const disconnected = !monitoringPaused && !connecting && (
    connection?.phase === 'disconnected'
    || connection?.phase === 'retrying'
    || Boolean(syncError)
  )
  const cloudTone = monitoringPaused
    ? 'muted'
    : connection?.phase === 'connected' && !syncError
      ? 'ok'
      : connecting || connection?.phase === 'retrying'
        ? 'warn'
        : (disconnected ? 'crit' : 'muted')
  const contextHint = contextLocked
    ? 'This window stays on the current cluster. Pick another context to open it in a new window.'
    : monitoringPaused
      ? `${ctx} — monitoring paused. Reconnect to resume.`
      : disconnected
        ? `${ctx} — not connected. Open to switch context or reconnect.`
        : 'Kubernetes context for this window'

  const filtered = filterBySubstring(contexts, filter, (c) =>
    [c.name, c.cluster, c.user, c.namespace].filter(Boolean).join(' '),
  )

  return (
    <div className="shell-popover-anchor shell-scope-anchor" ref={anchorRef}>
      <button
        type="button"
        className={[
          'shell-scope-btn',
          'shell-scope-btn-context',
          disconnected ? 'is-disconnected' : '',
          monitoringPaused ? 'is-paused' : '',
        ].filter(Boolean).join(' ')}
        onClick={() => setOpen((v) => !v)}
        disabled={disabled}
        aria-expanded={open}
        aria-haspopup="listbox"
        aria-label={`Context: ${ctx}${disconnected ? ' (disconnected)' : ''}`}
        title={contextHint}
      >
        <ClusterCloudIcon tone={cloudTone} className="shell-scope-cloud" />
        <span className="shell-scope-value mono">{ctx}</span>
        <ChevronDown />
      </button>
      <PopoverPortal
        open={open}
        anchorRef={anchorRef}
        panelRef={panelRef}
        className="shell-scope-popover"
        role="listbox"
        ariaLabel="Context"
      >
        <PopoverSearch
          inputRef={inputRef}
          value={filter}
          onChange={setFilter}
          placeholder="Search contexts…"
          ariaLabel="Filter contexts"
        />
        {contexts.length > 8 && (
          <p className="shell-popover-count muted">
            {filtered.length} of {contexts.length}
          </p>
        )}
        {!hasClusterContexts(cluster) ? (
          <div className="shell-popover-empty-state">
            <p className="shell-popover-empty-title">No contexts found</p>
            <p className="shell-popover-empty muted">
              Check your kubeconfig path in Settings, or reconnect once the cluster API is reachable.
            </p>
          </div>
        ) : (
          <ul className="shell-popover-list shell-popover-list-scroll shell-popover-list-tall">
            {filtered.map((c) => {
              const active = c.name === ctx
              const isDefault = c.name === defaultContext
              const itemTone = active ? cloudTone : 'muted'
              return (
                <li key={c.name}>
                  <button
                    type="button"
                    role="option"
                    aria-selected={active}
                    className={`shell-popover-item ${active ? 'active' : ''}`}
                    title={[c.cluster, c.user, c.namespace && `ns: ${c.namespace}`].filter(Boolean).join(' · ')}
                    onClick={() => {
                      onContextChange?.(c.name)
                      if (!contextLocked) setOpen(false)
                    }}
                  >
                    {active && <span className="shell-popover-check" aria-hidden="true">✓</span>}
                    <ClusterCloudIcon tone={itemTone} className="shell-popover-item-cloud" size={13} />
                    <span className="shell-popover-item-stack">
                      <span className="mono">
                        {c.name}
                        {isDefault && (
                          <span className="shell-popover-default-tag">default</span>
                        )}
                      </span>
                      {c.cluster && c.cluster !== c.name && (
                        <span className="shell-popover-item-sub muted">{c.cluster}</span>
                      )}
                    </span>
                  </button>
                  {onSetDefaultContext && (
                    <button
                      type="button"
                      className={['shell-popover-pin-btn', isDefault ? 'is-default' : ''].filter(Boolean).join(' ')}
                      title={isDefault ? 'Clear default on launch' : 'Set as default context'}
                      aria-label={isDefault ? `Clear ${c.name} as default context` : `Set ${c.name} as default context`}
                      onClick={(e) => {
                        e.stopPropagation()
                        onSetDefaultContext(c.name)
                      }}
                    >
                      {isDefault ? '★' : '☆'}
                    </button>
                  )}
                </li>
              )
            })}
            {filtered.length === 0 && (
              <li className="shell-popover-empty muted">No contexts match &quot;{filter}&quot;</li>
            )}
          </ul>
        )}
        {(monitoringPaused || disconnected) && onReconnect && (
          <div className="shell-popover-footer">
            {syncError && !monitoringPaused && (
              <p className="shell-popover-footer-note muted" title={syncError}>
                {syncError}
              </p>
            )}
            {monitoringPaused && (
              <p className="shell-popover-footer-note muted">
                Monitoring is paused — no live polls or watches until you reconnect.
              </p>
            )}
            <div className="shell-popover-footer-actions">
              <button
                type="button"
                className="btn btn-outline btn-sm shell-popover-reconnect-btn"
                onClick={() => {
                  onReconnect()
                  setOpen(false)
                }}
                disabled={reconnectBusy}
              >
                {reconnectBusy ? 'Connecting…' : 'Reconnect'}
              </button>
              {!monitoringPaused && onDisconnect && (
                <button
                  type="button"
                  className="text-link-btn shell-popover-disconnect-btn"
                  onClick={() => {
                    onDisconnect()
                    setOpen(false)
                  }}
                  disabled={reconnectBusy}
                >
                  Disconnect
                </button>
              )}
            </div>
          </div>
        )}
        {!monitoringPaused && !disconnected && onDisconnect && hasClusterContexts(cluster) && (
          <div className="shell-popover-footer shell-popover-footer-compact">
            <button
              type="button"
              className="text-link-btn shell-popover-disconnect-btn"
              onClick={() => {
                onDisconnect()
                setOpen(false)
              }}
            >
              Disconnect — stop monitoring
            </button>
          </div>
        )}
      </PopoverPortal>
    </div>
  )
}

const NS_NAME_RE = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/
const MAX_BROWSE_NAMESPACES = 50

function selectedBrowseNamespaces(scope, allNamespaces) {
  const normalized = normalizeBrowseScope(scope)
  if (normalized.mode === 'all') return new Set(allNamespaces)
  if (normalized.mode === 'multi') return new Set(normalized.namespaces)
  if (normalized.namespace) return new Set([normalized.namespace])
  return new Set()
}

function scopeFromBrowseSelection(selected, allNamespaces) {
  const list = [...selected]
  if (list.length === 0) return allBrowseScope()
  if (list.length === 1) return singleBrowseScope(list[0])
  return multiBrowseScope(list)
}

export function NamespacePopover({
  cluster,
  scope,
  disabled,
  onScopeChange,
  compact = false,
  inline = false,
  locked = false,
  lockedNamespace = '',
  savedScopeLabel = '',
  variant = 'browse',
}) {
  const { open, setOpen, anchorRef, panelRef } = usePopover()
  const { filter, setFilter, inputRef } = usePopoverFilter(open)
  const isInvestigate = variant === 'investigate'
  const normalized = isInvestigate
    ? normalizeInvestigationScope(scope, { fallbackNamespace: cluster.selectedNamespace })
    : normalizeBrowseScope(scope)
  const namespaces = cluster.namespaces || []
  const label = locked
    ? (lockedNamespace || investigationLockedLabel(normalized, savedScopeLabel))
    : browseScopeLabel(normalized, { namespaces })
  const trimmed = filter.trim()
  const filtered = filterBySubstring(namespaces, filter, (name) => name)
  const exactMatch = trimmed && namespaces.includes(trimmed)
  const canUseTyped = trimmed && NS_NAME_RE.test(trimmed) && !exactMatch

  const scopeLabelText = isInvestigate ? 'Investigate' : 'Resources'
  const anchorClass = [
    'shell-popover-anchor',
    'shell-scope-anchor',
    compact ? 'shell-scope-anchor-compact' : '',
    inline ? 'shell-scope-anchor-inline' : '',
    open ? 'shell-scope-anchor-open' : '',
  ].filter(Boolean).join(' ')
  const openTitle = isInvestigate
    ? 'Pick one namespace to investigate'
    : 'Browse resources by namespace'

  const browseSelected = selectedBrowseNamespaces(normalized, namespaces)
  const browseAllChecked = !isInvestigate && normalized.mode === 'all'
  const browseSelectedCount = browseAllChecked ? namespaces.length : browseSelected.size

  function applyScope(next, { close = true } = {}) {
    onScopeChange?.(next)
    if (close) setOpen(false)
  }

  function pickSingle(name) {
    applyScope(singleBrowseScope(name))
  }

  function applyBrowseSelection(nextSelected) {
    applyScope(scopeFromBrowseSelection(nextSelected, namespaces), { close: false })
  }

  function toggleBrowseAll(checked) {
    if (checked) {
      applyScope(allBrowseScope(), { close: false })
      return
    }
    const fallback = cluster.selectedNamespace || namespaces[0] || ''
    applyScope(fallback ? singleBrowseScope(fallback) : allBrowseScope(), { close: false })
  }

  function toggleBrowseNamespace(name) {
    const next = new Set(browseSelected)
    if (browseAllChecked) {
      next.delete(name)
    } else if (next.has(name)) {
      next.delete(name)
    } else if (next.size >= MAX_BROWSE_NAMESPACES) {
      return
    } else {
      next.add(name)
    }
    applyBrowseSelection(next)
  }

  function onSearchKeyDown(e) {
    if (e.key !== 'Enter') return
    e.preventDefault()
    if (exactMatch) {
      if (isInvestigate) pickSingle(trimmed)
      else toggleBrowseNamespace(trimmed)
      return
    }
    if (canUseTyped) {
      if (isInvestigate) pickSingle(trimmed)
      else toggleBrowseNamespace(trimmed)
      return
    }
    if (filtered.length === 1) {
      if (isInvestigate) pickSingle(filtered[0])
      else toggleBrowseNamespace(filtered[0])
    }
  }

  const lockTitle = isInvestigate
    ? `Investigation locked to ${label}. Stop investigation to change scope.`
    : (savedScopeLabel
      ? `Resources locked to ${label} during investigation. Browse scope was ${savedScopeLabel}. Stop to observe freely.`
      : `Resources locked to ${label} during investigation. Stop to observe freely.`)

  if (locked) {
    return (
      <div className={`${anchorClass} shell-scope-anchor-locked`}>
        <span
          className={`shell-scope-btn shell-scope-btn-namespace shell-scope-btn-locked ${compact ? 'shell-scope-btn-compact' : ''} ${inline ? 'shell-scope-btn-inline' : ''}`}
          title={lockTitle}
          aria-label={`${scopeLabelText} scope: ${label}`}
        >
          {!compact && <span className="shell-scope-label muted">{scopeLabelText}</span>}
          <LockIcon />
          <span className="shell-scope-value mono">{label}</span>
        </span>
      </div>
    )
  }

  return (
    <div className={anchorClass} ref={anchorRef}>
      <button
        type="button"
        className={`shell-scope-btn shell-scope-btn-namespace ${compact ? 'shell-scope-btn-compact' : ''} ${inline ? 'shell-scope-btn-inline' : ''}`}
        onClick={() => setOpen((v) => !v)}
        disabled={disabled}
        aria-expanded={open}
        aria-haspopup="listbox"
        aria-label={`Scope: ${label}`}
        title={cluster.syncWarning || openTitle}
      >
        {!compact && <span className="shell-scope-label muted">{scopeLabelText}</span>}
        <span className="shell-scope-value mono">{label}</span>
        <ChevronDown />
      </button>
      <PopoverPortal
        open={open}
        anchorRef={anchorRef}
        panelRef={panelRef}
        className={`shell-scope-popover ${isInvestigate ? 'shell-scope-popover-investigate' : 'shell-scope-popover-browse'}`}
        role="listbox"
        ariaLabel="Cluster scope"
      >
        <PopoverSearch
          inputRef={inputRef}
          value={filter}
          onChange={setFilter}
          placeholder="Search or type namespace…"
          ariaLabel="Filter namespaces"
          onKeyDown={onSearchKeyDown}
        />
        <div className="shell-popover-list-body">
          {namespaces.length > 8 && (
            <p className="shell-popover-count muted">
              {filtered.length} of {namespaces.length}
            </p>
          )}
          <ul className="shell-popover-list shell-popover-list-scroll shell-popover-list-tall">
            {!isInvestigate && (
              <li>
                <label className={`shell-popover-item shell-popover-item-check ${browseAllChecked ? 'active' : ''}`}>
                  <input
                    type="checkbox"
                    checked={browseAllChecked}
                    onChange={(e) => toggleBrowseAll(e.target.checked)}
                  />
                  <span>All namespaces</span>
                </label>
              </li>
            )}
            {canUseTyped && (
              <li>
                <button
                  type="button"
                  className="shell-popover-item shell-popover-item-use-typed"
                  onClick={() => (isInvestigate ? pickSingle(trimmed) : toggleBrowseNamespace(trimmed))}
                >
                  <span className="mono">{isInvestigate ? `Use namespace "${trimmed}"` : `Add "${trimmed}"`}</span>
                </button>
              </li>
            )}
            {filtered.map((name) => {
              const activeSingle = isInvestigate && normalized.namespace === name
              const isChecked = isInvestigate ? activeSingle : browseSelected.has(name)
              return (
                <li key={name}>
                  {isInvestigate ? (
                    <button
                      type="button"
                      role="option"
                      aria-selected={activeSingle}
                      className={`shell-popover-item ${activeSingle ? 'active' : ''}`}
                      onClick={() => pickSingle(name)}
                    >
                      {activeSingle && <span className="shell-popover-check" aria-hidden="true">✓</span>}
                      <span className="mono">{name}</span>
                    </button>
                  ) : (
                    <label className={`shell-popover-item shell-popover-item-check ${isChecked ? 'active' : ''}`}>
                      <input
                        type="checkbox"
                        checked={isChecked}
                        disabled={!isChecked && browseSelectedCount >= MAX_BROWSE_NAMESPACES && !browseAllChecked}
                        onChange={(e) => {
                          e.stopPropagation()
                          toggleBrowseNamespace(name)
                        }}
                      />
                      <span className="mono">{name}</span>
                    </label>
                  )}
                </li>
              )
            })}
            {filtered.length === 0 && !canUseTyped && (
              <li className="shell-popover-empty muted">
                {trimmed ? `No namespaces match "${trimmed}"` : 'No namespaces listed'}
              </li>
            )}
          </ul>
        </div>
      </PopoverPortal>
    </div>
  )
}

function ChevronDown() {
  return (
    <svg className="shell-scope-chevron" viewBox="0 0 16 16" width="12" height="12" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M4 6l4 4 4-4" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

function LockIcon() {
  return (
    <svg className="shell-scope-lock" viewBox="0 0 16 16" width="12" height="12" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M4.5 7V5a3.5 3.5 0 117 0v2" strokeLinecap="round" />
      <rect x="3.5" y="7" width="9" height="6.5" rx="1.5" />
    </svg>
  )
}

function investigationLockedLabel(scope, savedScopeLabel) {
  if (scope.mode === 'single' && scope.namespace) return scope.namespace
  if (savedScopeLabel) return '…'
  return '…'
}
