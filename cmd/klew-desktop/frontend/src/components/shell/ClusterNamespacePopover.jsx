import { useEffect, useRef, useState } from 'react'

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

export function ContextPopover({
  cluster,
  disabled,
  contextLocked,
  onContextChange,
}) {
  const { open, setOpen, rootRef } = usePopover()
  const contexts = cluster.contexts || []
  const ctx = cluster.selectedContext || cluster.currentContext || '—'
  const contextHint = contextLocked
    ? 'This window stays on the current cluster. Pick another context to open it in a new window.'
    : 'Kubernetes context for this window'

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
          <ul className="shell-popover-list shell-popover-list-scroll">
            {contexts.map((c) => {
              const active = c.name === ctx
              return (
                <li key={c.name}>
                  <button
                    type="button"
                    role="option"
                    aria-selected={active}
                    className={`shell-popover-item ${active ? 'active' : ''}`}
                    title={c.cluster}
                    onClick={() => {
                      onContextChange?.(c.name)
                      if (!contextLocked) setOpen(false)
                    }}
                  >
                    {active && <span className="shell-popover-check" aria-hidden="true">✓</span>}
                    <span className="mono">{c.name}</span>
                  </button>
                </li>
              )
            })}
          </ul>
        </div>
      )}
    </div>
  )
}

export function NamespacePopover({
  cluster,
  disabled,
  onNamespaceChange,
}) {
  const { open, setOpen, rootRef } = usePopover()
  const namespaces = cluster.namespaces || []
  const ns = cluster.selectedNamespace || '—'

  return (
    <div className="shell-popover-anchor shell-scope-anchor" ref={rootRef}>
      <button
        type="button"
        className="shell-scope-btn shell-scope-btn-namespace"
        onClick={() => setOpen((v) => !v)}
        disabled={disabled || namespaces.length === 0}
        aria-expanded={open}
        aria-haspopup="listbox"
        aria-label={`Namespace: ${ns}`}
        title="Namespace for this window"
      >
        <span className="shell-scope-value mono">{ns}</span>
        <ChevronDown />
      </button>
      {open && (
        <div className="shell-popover shell-scope-popover" role="listbox" aria-label="Namespace">
          <ul className="shell-popover-list shell-popover-list-scroll">
            {namespaces.map((name) => {
              const active = name === ns
              return (
                <li key={name}>
                  <button
                    type="button"
                    role="option"
                    aria-selected={active}
                    className={`shell-popover-item ${active ? 'active' : ''}`}
                    onClick={() => {
                      onNamespaceChange?.(name)
                      setOpen(false)
                    }}
                  >
                    {active && <span className="shell-popover-check" aria-hidden="true">✓</span>}
                    <span className="mono">{name}</span>
                  </button>
                </li>
              )
            })}
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
