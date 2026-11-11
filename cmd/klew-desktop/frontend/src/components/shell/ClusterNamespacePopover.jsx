import { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { filterBySubstring } from '../../lib/scopeSearch.js'
import { hasClusterContexts } from '../../lib/clusterIdentity.js'
import { displayContextName, orderContextsForDisplay } from '../../lib/clusterContexts.js'
import { ClusterCloudIcon } from './ClusterCloudIcon.jsx'
import { ContextRecordingToggle } from './ContextRecordingToggle.jsx'
import {
  allBrowseScope,
  browseScopeLabel,
  browseScopesEqual,
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

export function usePopover() {
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

export function PopoverPortal({ open, anchorRef, panelRef, className, role, ariaLabel, children }) {
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

export function usePopoverFilter(open) {
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

export function PopoverSearch({ value, onChange, inputRef, placeholder, ariaLabel, onKeyDown }) {
  return (
    <div className="shell-popover-search">
      <svg className="shell-popover-search-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
        <circle cx="11" cy="11" r="6.5" />
        <path d="M20 20l-4-4" strokeLinecap="round" />
      </svg>
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

/** Plug: connect (or reconnect) a context. */
function PlugIcon() {
  return (
    <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M9 3v4M15 3v4" />
      <path d="M6.5 7h11v4a5.5 5.5 0 0 1-11 0V7Z" />
      <path d="M12 16.5V21" />
    </svg>
  )
}

/** Unplugged: stop watching the connected context. */
function UnplugIcon() {
  return (
    <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M9 3v4M15 3v4" />
      <path d="M6.5 7h11v4a5.5 5.5 0 0 1-11 0V7Z" />
      <path d="M12 16.5V21" />
      <path d="M3.5 3.5l17 17" />
    </svg>
  )
}

/**
 * Inline connection control for a context row: Disconnect on the connected
 * context, Reconnect when it is paused or dropped, Connect on any other.
 */
function ConnectionButton({ name, active, paused, disconnected, busy, onConnect, onReconnect, onDisconnect }) {
  let kind = 'connect'
  if (active) kind = paused || disconnected ? 'reconnect' : 'disconnect'
  const action = { connect: onConnect, reconnect: onReconnect, disconnect: onDisconnect }[kind]
  if (!action) return null
  const label = {
    connect: `Connect to ${name}`,
    reconnect: busy ? 'Connecting…' : `Reconnect ${name}`,
    disconnect: `Disconnect ${name} (stop watching)`,
  }[kind]
  return (
    <button
      type="button"
      className={`shell-popover-conn-btn is-${kind}`}
      title={label}
      aria-label={label}
      disabled={busy && kind !== 'connect'}
      onClick={(e) => {
        e.stopPropagation()
        action()
      }}
    >
      {kind === 'disconnect' ? <UnplugIcon /> : <PlugIcon />}
    </button>
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
  favoriteContexts = [],
  contextAliases = {},
  compact = false,
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

  const filteredRaw = filterBySubstring(contexts, filter, (c) =>
    [
      c.name,
      displayContextName(contextAliases, c.name, { truncate: false }),
      c.cluster,
      c.user,
      c.namespace,
    ].filter(Boolean).join(' '),
  )
  const filtered = filter.trim()
    ? filteredRaw
    : orderContextsForDisplay(filteredRaw, { favoriteContexts })

  return (
    <div className="shell-popover-anchor shell-scope-anchor" ref={anchorRef}>
      <button
        type="button"
        className={[
          'shell-scope-btn',
          'shell-scope-btn-context',
          compact ? 'is-compact' : '',
          disconnected ? 'is-disconnected' : '',
          monitoringPaused ? 'is-paused' : '',
        ].filter(Boolean).join(' ')}
        onClick={() => setOpen((v) => !v)}
        disabled={disabled}
        aria-expanded={open}
        aria-haspopup="listbox"
        aria-label={compact ? 'Open another context' : `Context: ${ctx}${disconnected ? ' (disconnected)' : ''}`}
        title={compact ? 'Open another context' : contextHint}
      >
        {compact ? (
          <svg viewBox="0 0 16 16" width="13" height="13" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" aria-hidden="true">
            <path d="M8 3v10M3 8h10" />
          </svg>
        ) : (
          <>
            <span className="shell-scope-value mono">{ctx}</span>
            <ChevronDown />
          </>
        )}
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
              const itemTone = active ? cloudTone : 'muted'
              const label = displayContextName(contextAliases, c.name)
              const fullLabel = displayContextName(contextAliases, c.name, { truncate: false })
              return (
                <li key={c.name}>
                  <button
                    type="button"
                    role="option"
                    aria-selected={active}
                    className={`shell-popover-item ${active ? 'active' : ''}`}
                    title={[
                      fullLabel !== c.name ? `${fullLabel} (${c.name})` : c.name,
                      c.cluster,
                      c.user,
                      c.namespace && `ns: ${c.namespace}`,
                    ].filter(Boolean).join(' · ')}
                    onClick={() => {
                      onContextChange?.(c.name)
                      if (!contextLocked) setOpen(false)
                    }}
                  >
                    {active && <span className="shell-popover-check" aria-hidden="true">✓</span>}
                    <ClusterCloudIcon tone={itemTone} className="shell-popover-item-cloud" size={13} />
                    <span className="shell-popover-item-stack">
                      <span className="mono">{label}</span>
                      {fullLabel !== c.name && (
                        <span className="shell-popover-item-sub muted">{c.name}</span>
                      )}
                      {c.cluster && c.cluster !== c.name && fullLabel === c.name && (
                        <span className="shell-popover-item-sub muted">{c.cluster}</span>
                      )}
                      <ContextRecordingToggle contextName={c.name} size={14} />
                    </span>
                  </button>
                  <div className="shell-popover-item-actions">
                    <ConnectionButton
                      name={c.name}
                      active={active}
                      paused={monitoringPaused}
                      disconnected={disconnected}
                      busy={reconnectBusy}
                      onConnect={() => {
                        onContextChange?.(c.name)
                        if (!contextLocked) setOpen(false)
                      }}
                      onReconnect={onReconnect ? () => { onReconnect(); setOpen(false) } : undefined}
                      onDisconnect={onDisconnect ? () => { onDisconnect(); setOpen(false) } : undefined}
                    />
                  </div>
                </li>
              )
            })}
            {filtered.length === 0 && (
              <li className="shell-popover-empty muted">No contexts match &quot;{filter}&quot;</li>
            )}
          </ul>
        )}
        {(monitoringPaused || (disconnected && syncError)) && (
          <p className="shell-popover-status-note" title={syncError || undefined}>
            {monitoringPaused
              ? 'Monitoring paused · use the plug to reconnect'
              : syncError}
          </p>
        )}
      </PopoverPortal>
    </div>
  )
}

const NS_NAME_RE = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/
const MAX_BROWSE_NAMESPACES = 50

function scopeFromBrowseSelection(selected, allNamespaces) {
  const list = [...selected]
  if (list.length === 0) return allBrowseScope()
  if (list.length === 1) return singleBrowseScope(list[0])
  return multiBrowseScope(list)
}

function browseSelectionSeed(normalized, namespaces) {
  const s = normalizeBrowseScope(normalized)
  if (s.mode === 'multi') return new Set(s.namespaces)
  if (s.mode === 'single' && s.namespace) return new Set([s.namespace])
  if (s.mode === 'all') return new Set(namespaces)
  return new Set()
}

function scopeFromBrowseDraft(draft, namespaces, allExplicit) {
  if (
    allExplicit
    && namespaces.length > 0
    && draft.size === namespaces.length
  ) {
    return allBrowseScope()
  }
  if (draft.size === 0) return null
  return scopeFromBrowseSelection(draft, namespaces)
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

  const [browseDraft, setBrowseDraft] = useState(() => new Set())
  const [browseAllExplicit, setBrowseAllExplicit] = useState(false)

  const multiScopeKey = normalized.mode === 'multi' ? normalized.namespaces.join('\0') : ''
  const namespaceListKey = namespaces.join('\0')

  useEffect(() => {
    if (!open || isInvestigate) return
    setBrowseAllExplicit(normalized.mode === 'all')
    setBrowseDraft(browseSelectionSeed(normalized, namespaces))
  }, [open, isInvestigate, normalized.mode, normalized.namespace, multiScopeKey, namespaceListKey, namespaces])

  const draftAllNamespaces = !isInvestigate
    && browseAllExplicit
    && namespaces.length > 0
    && browseDraft.size === namespaces.length

  const browseCanApply = useMemo(() => {
    if (isInvestigate) return false
    const next = scopeFromBrowseDraft(browseDraft, namespaces, browseAllExplicit)
    if (!next) return false
    return !browseScopesEqual(next, normalized)
  }, [isInvestigate, browseDraft, browseAllExplicit, namespaces, normalized])

  const browseFooterNote = useMemo(() => {
    if (draftAllNamespaces) return 'All namespaces'
    if (browseDraft.size === 0) return 'Select at least one namespace'
    if (browseDraft.size === 1) return `1 namespace · ${[...browseDraft][0]}`
    return `${browseDraft.size} namespaces selected`
  }, [browseDraft, draftAllNamespaces])

  function applyScope(next, { close = true } = {}) {
    onScopeChange?.(next)
    if (close) setOpen(false)
  }

  function pickSingle(name) {
    applyScope(singleBrowseScope(name))
  }

  function nextBrowseDraftOnCheck(prev, name, checked) {
    if (checked) {
      if (prev.has(name)) return prev
      if (prev.size === 0) return new Set([name])
      if (prev.size === 1) {
        const only = [...prev][0]
        if (only === name) return prev
        return new Set([only, name])
      }
      if (prev.size >= MAX_BROWSE_NAMESPACES) return prev
      const next = new Set(prev)
      next.add(name)
      return next
    }
    const next = new Set(prev)
    next.delete(name)
    return next
  }

  function setBrowseDraftChecked(name, checked) {
    if (!name) return
    const leavingAll = browseAllExplicit
      && namespaces.length > 0
      && browseDraft.size === namespaces.length
    if (leavingAll) {
      setBrowseAllExplicit(false)
    } else if (checked) {
      setBrowseAllExplicit(false)
    }
    setBrowseDraft((prev) => {
      if (checked && leavingAll) return new Set([name])
      return nextBrowseDraftOnCheck(prev, name, checked)
    })
  }

  function toggleBrowseDraftAll(checked) {
    setBrowseAllExplicit(checked)
    setBrowseDraft(checked ? new Set(namespaces) : new Set())
  }

  function cancelBrowseDraft() {
    setOpen(false)
  }

  function applyBrowseDraft() {
    const next = scopeFromBrowseDraft(browseDraft, namespaces, browseAllExplicit)
    if (!next) return
    applyScope(next)
  }

  function onSearchKeyDown(e) {
    if (e.key !== 'Enter') return
    e.preventDefault()
    const pick = exactMatch ? trimmed : (canUseTyped ? trimmed : (filtered.length === 1 ? filtered[0] : ''))
    if (!pick) return
    if (!isInvestigate) {
      setBrowseDraftChecked(pick, true)
      return
    }
    pickSingle(pick)
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
          {!isInvestigate && (
            <p className="shell-popover-hint muted">
              Adjust checkboxes, then Apply or Cancel — nothing closes until you choose.
            </p>
          )}
          <ul className="shell-popover-list shell-popover-list-scroll shell-popover-list-tall">
            {!isInvestigate && (
              <li>
                <label className={`shell-popover-item shell-popover-item-check ${draftAllNamespaces ? 'active' : ''}`}>
                  <input
                    type="checkbox"
                    checked={draftAllNamespaces}
                    onChange={(e) => toggleBrowseDraftAll(e.target.checked)}
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
                  onClick={() => {
                    if (isInvestigate) pickSingle(trimmed)
                    else setBrowseDraftChecked(trimmed, true)
                  }}
                >
                  <span className="mono">
                    {isInvestigate ? `Use namespace "${trimmed}"` : `Select "${trimmed}"`}
                  </span>
                </button>
              </li>
            )}
            {filtered.map((name) => {
              if (!isInvestigate) {
                const checked = browseDraft.has(name)
                const atCap = !checked && browseDraft.size >= MAX_BROWSE_NAMESPACES
                return (
                  <li key={name}>
                    <label className={`shell-popover-item shell-popover-item-check ${checked ? 'active' : ''}`}>
                      <input
                        type="checkbox"
                        checked={checked}
                        disabled={atCap}
                        onChange={(e) => setBrowseDraftChecked(name, e.target.checked)}
                      />
                      <span className="mono">{name}</span>
                    </label>
                  </li>
                )
              }
              const active = normalized.namespace === name
              return (
                <li key={name}>
                  <button
                    type="button"
                    role="option"
                    aria-selected={active}
                    className={`shell-popover-item ${active ? 'active' : ''}`}
                    onClick={() => pickSingle(name)}
                  >
                    {active && <span className="shell-popover-check" aria-hidden="true">✓</span>}
                    <span className="mono">{name}</span>
                  </button>
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
        {!isInvestigate && (
          <div className="shell-popover-footer shell-scope-multi-footer">
            <span className="shell-popover-footer-note muted">{browseFooterNote}</span>
            <div className="shell-popover-footer-actions">
              <button
                type="button"
                className="shell-popover-footer-btn"
                onClick={cancelBrowseDraft}
              >
                Cancel
              </button>
              <button
                type="button"
                className="shell-popover-footer-btn primary"
                disabled={!browseCanApply}
                onClick={applyBrowseDraft}
              >
                Apply
              </button>
            </div>
          </div>
        )}
      </PopoverPortal>
    </div>
  )
}

export function ChevronDown() {
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
