import { useEffect, useRef, useState } from 'react'
import { filterBySubstring } from '../../lib/scopeSearch.js'

function usePopover() {
  const [open, setOpen] = useState(false)
  const rootRef = useRef(null)

  useEffect(() => {
    if (!open) return
    function onDoc(e) {
      if (rootRef.current && !rootRef.current.contains(e.target)) setOpen(false)
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
  }, [open])

  return { open, setOpen, rootRef }
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
}) {
  const { open, setOpen, rootRef } = usePopover()
  const { filter, setFilter, inputRef } = usePopoverFilter(open)
  const contexts = cluster.contexts || []
  const ctx = cluster.selectedContext || cluster.currentContext || '—'
  const contextHint = contextLocked
    ? 'This window stays on the current cluster. Pick another context to open it in a new window.'
    : 'Kubernetes context for this window'

  const filtered = filterBySubstring(contexts, filter, (c) =>
    [c.name, c.cluster, c.user, c.namespace].filter(Boolean).join(' '),
  )

  return (
    <div className="shell-popover-anchor shell-scope-anchor" ref={rootRef}>
      <button
        type="button"
        className="shell-scope-btn shell-scope-btn-context"
        onClick={() => setOpen((v) => !v)}
        disabled={disabled}
        aria-expanded={open}
        aria-haspopup="listbox"
        aria-label={`Context: ${ctx}`}
        title={contextHint}
      >
        <span className="shell-scope-value mono">{ctx}</span>
        <ChevronDown />
      </button>
      {open && (
        <div className="shell-popover shell-scope-popover" role="listbox" aria-label="Context">
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
          <ul className="shell-popover-list shell-popover-list-scroll shell-popover-list-tall">
            {filtered.map((c) => {
              const active = c.name === ctx
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
                    <span className="shell-popover-item-stack">
                      <span className="mono">{c.name}</span>
                      {c.cluster && c.cluster !== c.name && (
                        <span className="shell-popover-item-sub muted">{c.cluster}</span>
                      )}
                    </span>
                  </button>
                </li>
              )
            })}
            {filtered.length === 0 && (
              <li className="shell-popover-empty muted">No contexts match &quot;{filter}&quot;</li>
            )}
          </ul>
        </div>
      )}
    </div>
  )
}

const NS_NAME_RE = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/

export function NamespacePopover({
  cluster,
  disabled,
  onNamespaceChange,
}) {
  const { open, setOpen, rootRef } = usePopover()
  const { filter, setFilter, inputRef } = usePopoverFilter(open)
  const namespaces = cluster.namespaces || []
  const ns = cluster.selectedNamespace || '—'
  const trimmed = filter.trim()
  const filtered = filterBySubstring(namespaces, filter, (name) => name)
  const exactMatch = trimmed && namespaces.includes(trimmed)
  const canUseTyped = trimmed && NS_NAME_RE.test(trimmed) && !exactMatch

  function pickNamespace(name) {
    onNamespaceChange?.(name)
    setOpen(false)
  }

  function onSearchKeyDown(e) {
    if (e.key !== 'Enter') return
    e.preventDefault()
    if (exactMatch) {
      pickNamespace(trimmed)
      return
    }
    if (canUseTyped) {
      pickNamespace(trimmed)
      return
    }
    if (filtered.length === 1) {
      pickNamespace(filtered[0])
    }
  }

  return (
    <div className="shell-popover-anchor shell-scope-anchor" ref={rootRef}>
      <button
        type="button"
        className="shell-scope-btn shell-scope-btn-namespace"
        onClick={() => setOpen((v) => !v)}
        disabled={disabled}
        aria-expanded={open}
        aria-haspopup="listbox"
        aria-label={`Namespace: ${ns}`}
        title={cluster.syncWarning || 'Namespace for this window'}
      >
        <span className="shell-scope-value mono">{ns}</span>
        <ChevronDown />
      </button>
      {open && (
        <div className="shell-popover shell-scope-popover" role="listbox" aria-label="Namespace">
          {cluster.syncWarning && (
            <p className="shell-popover-hint">{cluster.syncWarning}</p>
          )}
          <PopoverSearch
            inputRef={inputRef}
            value={filter}
            onChange={setFilter}
            placeholder="Search or type namespace…"
            ariaLabel="Filter namespaces"
            onKeyDown={onSearchKeyDown}
          />
          {namespaces.length > 8 && (
            <p className="shell-popover-count muted">
              {filtered.length} of {namespaces.length}
            </p>
          )}
          <ul className="shell-popover-list shell-popover-list-scroll shell-popover-list-tall">
            {canUseTyped && (
              <li>
                <button
                  type="button"
                  className="shell-popover-item shell-popover-item-use-typed"
                  onClick={() => pickNamespace(trimmed)}
                >
                  <span className="mono">Use namespace &quot;{trimmed}&quot;</span>
                </button>
              </li>
            )}
            {filtered.map((name) => {
              const active = name === ns
              return (
                <li key={name}>
                  <button
                    type="button"
                    role="option"
                    aria-selected={active}
                    className={`shell-popover-item ${active ? 'active' : ''}`}
                    onClick={() => pickNamespace(name)}
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
      )}
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
