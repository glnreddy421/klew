import { useEffect, useRef, useState } from 'react'
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime'

/** Release whose highlights the banner shows; bump it to show the banner again. */
export const WHATS_NEW_VERSION = '2.0'

const RELEASE_NOTES_URL = 'https://github.com/glnreddy421/klew/blob/main/CHANGELOG.md'

const HIGHLIGHTS = [
  {
    id: 'investigate',
    title: 'Investigate from Resources',
    body: 'Switch Watch → Investigate (⌘K) to scope one namespace or object. Results open as tabs on the same screen.',
  },
  {
    id: 'overview',
    title: 'An Overview that answers first',
    body: 'Verdict, failing pods with exit codes, pod impact, why it broke and what changed, before the raw evidence.',
  },
  {
    id: 'cli',
    title: 'klew in your terminal',
    body: 'klew investigate prints the same verdict in any shell or CI job.',
    action: 'cli',
  },
  {
    id: 'mcp',
    title: 'Klew for AI assistants',
    body: 'A read-only MCP server gives Claude, Cursor and others Klew\'s analysis instead of raw kubectl.',
    action: 'mcp',
  },
  {
    id: 'extensions',
    title: 'Your own collectors and rules',
    body: 'Add CEL rules for your resources in Settings → Extensions.',
  },
]

/**
 * One-time "Klew 2.0" strip on Home: one line, with the highlights in a
 * popover. Dismissed per release, so new and upgrading users see it once.
 */
export function WhatsNewBanner({ onDismiss, onOpenMcpSetup, onOpenCli }) {
  const [open, setOpen] = useState(false)
  const anchorRef = useRef(null)
  const panelRef = useRef(null)
  const actions = { mcp: onOpenMcpSetup, cli: onOpenCli }

  useEffect(() => {
    if (!open) return undefined
    function onDoc(e) {
      if (anchorRef.current?.contains(e.target) || panelRef.current?.contains(e.target)) return
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
  }, [open])

  return (
    <div className="whats-new-strip" role="status">
      <span className="whats-new-spark" aria-hidden="true">✦</span>
      <span className="whats-new-text">
        <strong>Klew {WHATS_NEW_VERSION}</strong>
        {' — '}the CLI, MCP and Investigate in Resources are new.
      </span>
      <span className="whats-new-anchor" ref={anchorRef}>
        <button
          type="button"
          className="whats-new-link"
          onClick={() => setOpen((v) => !v)}
          aria-expanded={open}
          aria-haspopup="dialog"
        >
          See what's new ›
        </button>
        {open && (
          <div ref={panelRef} className="whats-new-popover" role="dialog" aria-label={`What's new in Klew ${WHATS_NEW_VERSION}`}>
            <ul className="whats-new-list">
              {HIGHLIGHTS.map((h) => {
                const act = h.action ? actions[h.action] : null
                return (
                  <li key={h.id} className="whats-new-item">
                    <span className="whats-new-item-title">{h.title}</span>
                    <span className="whats-new-item-body">{h.body}</span>
                    {act && (
                      <button
                        type="button"
                        className="whats-new-item-link"
                        onClick={() => { setOpen(false); act() }}
                      >
                        {h.action === 'mcp' ? 'Set up MCP →' : 'Try the CLI →'}
                      </button>
                    )}
                  </li>
                )
              })}
            </ul>
            <button type="button" className="whats-new-item-link whats-new-notes" onClick={() => BrowserOpenURL(RELEASE_NOTES_URL)}>
              Full release notes ↗
            </button>
          </div>
        )}
      </span>
      <button type="button" className="whats-new-close" onClick={onDismiss} aria-label="Dismiss" title="Dismiss">
        ×
      </button>
    </div>
  )
}
