import { useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { MCPSetupInfo } from '../../wailsjs/go/main/App'
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime'
import {
  MCP_BREW_FORMULA,
  MCP_CLIENTS,
  MCP_DOCS_URL,
  mcpFlags,
  mcpServerCommand,
  mcpSnippet,
  MCP_APP_PATH,
} from '../lib/mcpSetup.js'

const CLOSE_MS = 140

function copyText(text) {
  if (!text || !navigator.clipboard?.writeText) return Promise.resolve(false)
  return navigator.clipboard.writeText(text).then(() => true).catch(() => false)
}

function CopyBlock({ text, label }) {
  const [copied, setCopied] = useState(false)
  useEffect(() => {
    if (!copied) return undefined
    const t = setTimeout(() => setCopied(false), 1400)
    return () => clearTimeout(t)
  }, [copied])
  return (
    <div className="mcp-code">
      <pre aria-label={label}><code>{text}</code></pre>
      <button
        type="button"
        className="btn btn-ghost btn-sm mcp-copy"
        onClick={() => copyText(text).then(setCopied)}
      >
        {copied ? 'Copied' : 'Copy'}
      </button>
    </div>
  )
}

function SourceOption({ value, current, onChange, title, children }) {
  const selected = value === current
  return (
    <label className={`mcp-source ${selected ? 'is-selected' : ''}`}>
      <input type="radio" name="mcp-source" value={value} checked={selected} onChange={() => onChange(value)} />
      <span className="mcp-source-body">
        <span className="mcp-source-title">{title}</span>
        <span className="mcp-source-text">{children}</span>
      </span>
    </label>
  )
}

/**
 * Set Up MCP: shows how to point Claude Code, Claude Desktop, Cursor and
 * other clients at Klew's read-only MCP server, either this app's built-in
 * server ("<app> mcp") or the standalone klew command ("klew mcp"). It only shows text to copy;
 * it changes no client configuration itself.
 */
export function McpSetupDialog({ open, onClose, contexts = [] }) {
  const [mounted, setMounted] = useState(open)
  const [closing, setClosing] = useState(false)
  const [info, setInfo] = useState(null)
  const [source, setSource] = useState('app')
  const [client, setClient] = useState(MCP_CLIENTS[0].id)
  const [context, setContext] = useState('')
  const [noLogSamples, setNoLogSamples] = useState(false)
  const [noHistory, setNoHistory] = useState(false)
  const dialogRef = useRef(null)
  const returnFocusRef = useRef(null)

  useEffect(() => {
    if (open) {
      returnFocusRef.current = document.activeElement
      setMounted(true)
      setClosing(false)
      MCPSetupInfo().then(setInfo).catch(() => setInfo({}))
      return undefined
    }
    if (!mounted) return undefined
    setClosing(true)
    const t = setTimeout(() => {
      setMounted(false)
      setClosing(false)
      returnFocusRef.current?.focus?.()
    }, CLOSE_MS)
    return () => clearTimeout(t)
  }, [open])

  useEffect(() => {
    if (!open) return undefined
    dialogRef.current?.focus()
    function onKey(e) {
      if (e.key === 'Escape') {
        e.stopPropagation()
        onClose?.()
      }
    }
    window.addEventListener('keydown', onKey, true)
    return () => window.removeEventListener('keydown', onKey, true)
  }, [open, onClose])

  const appUsable = Boolean(info?.executable) && !info?.translocated
  const effectiveSource = appUsable ? source : 'standalone'
  const server = mcpServerCommand({ source: effectiveSource })
  const elsewhere = appUsable && info.executable !== MCP_APP_PATH
  const flags = mcpFlags({ context, noLogSamples, noHistory })
  const clientMeta = MCP_CLIENTS.find((c) => c.id === client) || MCP_CLIENTS[0]
  const snippet = useMemo(() => mcpSnippet(client, server, flags), [client, server.command, server.args.join(' '), flags.join(' ')])
  const contextNames = contexts.map((c) => c?.name).filter(Boolean)

  if (!mounted) return null

  return createPortal(
    <div
      className={`settings-dialog-backdrop ${closing ? 'is-closing' : ''}`}
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose?.()
      }}
    >
      <div
        ref={dialogRef}
        className={`settings-dialog mcp-dialog ${closing ? 'is-closing' : ''}`}
        role="dialog"
        aria-modal="true"
        aria-labelledby="mcp-dialog-title"
        tabIndex={-1}
      >
        <header className="mcp-head">
          <div>
            <h2 id="mcp-dialog-title">Use Klew from AI assistants</h2>
            <p className="muted">
              Klew&rsquo;s MCP server gives Claude, Cursor and other clients read-only access to investigations,
              events, workload changes, resources and pod logs, using your kubeconfig.
            </p>
          </div>
          <button type="button" className="settings-close" aria-label="Close" onClick={onClose}>×</button>
        </header>

        <div className="mcp-body">
          <section className="mcp-section" aria-labelledby="mcp-server-h">
            <h3 id="mcp-server-h">Server</h3>
            {info?.translocated && (
              <p className="mcp-warn" role="status">
                Klew is running from a temporary location, so a client could not start it later.
                Move Klew to Applications and open it from there to use the built-in server.
              </p>
            )}
            <div className="mcp-sources">
              <SourceOption value="app" current={effectiveSource} onChange={setSource} title="Built into this app">
                {appUsable
                  ? 'Nothing else to install. Includes the events this app records, older than the API keeps.'
                  : 'Unavailable from this location.'}
              </SourceOption>
              <SourceOption value="standalone" current={effectiveSource} onChange={setSource} title="Standalone klew command">
                {info?.standalonePath
                  ? <>Installed at <code>{info.standalonePath}</code>.</>
                  : 'For machines without the app, such as Linux or a devbox. Not installed here.'}
              </SourceOption>
            </div>
            {effectiveSource === 'app' && elsewhere && (
              <p className="mcp-hint muted">
                This copy of Klew runs from <code>{info.executable}</code>. The configuration below uses{' '}
                <code>{MCP_APP_PATH}</code>, so move Klew to Applications or change the path.
              </p>
            )}
            {effectiveSource === 'standalone' && !info?.standalonePath && (
              <>
                <p className="mcp-step">Install it with Homebrew (macOS or Linux):</p>
                <CopyBlock text={`brew install ${MCP_BREW_FORMULA}`} label="Install command" />
              </>
            )}
          </section>

          <section className="mcp-section" aria-labelledby="mcp-options-h">
            <h3 id="mcp-options-h">Options</h3>
            <div className="mcp-options">
              <label className="mcp-field">
                <span>Default context</span>
                <select value={context} onChange={(e) => setContext(e.target.value)}>
                  <option value="">Kubeconfig current context</option>
                  {contextNames.map((n) => <option key={n} value={n}>{n}</option>)}
                </select>
              </label>
              <label className="mcp-check">
                <input type="checkbox" checked={noLogSamples} onChange={(e) => setNoLogSamples(e.target.checked)} />
                <span>
                  <span className="mcp-check-title">Leave raw log text out <code>-no-log-samples</code></span>
                  <small className="muted">Patterns and counts are still returned; the pod logs tool is not offered.</small>
                </span>
              </label>
              <label className="mcp-check">
                <input type="checkbox" checked={noHistory} onChange={(e) => setNoHistory(e.target.checked)} />
                <span>
                  <span className="mcp-check-title">Ignore recorded events <code>-no-history</code></span>
                  <small className="muted">Only events the API still keeps, about the last hour.</small>
                </span>
              </label>
            </div>
          </section>

          <section className="mcp-section" aria-labelledby="mcp-client-h">
            <h3 id="mcp-client-h">Add to your client</h3>
            <div className="mcp-tabs" role="tablist" aria-label="Client">
              {MCP_CLIENTS.map((c) => (
                <button
                  key={c.id}
                  type="button"
                  role="tab"
                  aria-selected={client === c.id}
                  className={`mcp-tab ${client === c.id ? 'is-active' : ''}`}
                  onClick={() => setClient(c.id)}
                >
                  {c.label}
                </button>
              ))}
            </div>
            <CopyBlock text={snippet} label={`${clientMeta.label} configuration`} />
            <p className="mcp-hint muted">{clientMeta.hint}</p>
            {effectiveSource === 'standalone' && clientMeta.kind === 'json' && (
              <p className="mcp-hint muted">
                If the client cannot find klew, replace it with the full path <code>command -v klew</code> prints.
              </p>
            )}
            {clientMeta.existing && (
              <div className="mcp-existing">
                <p className="mcp-hint muted">{clientMeta.existing.text}</p>
                <CopyBlock text={clientMeta.existing.check} label="Check command" />
              </div>
            )}
          </section>

          <section className="mcp-section mcp-sees" aria-labelledby="mcp-sees-h">
            <h3 id="mcp-sees-h">What the assistant sees</h3>
            <ul>
              <li>Every tool is read-only. Suggested commands are text for you to review; nothing is run.</li>
              <li>Results become part of the conversation, which your client sends to its AI provider.</li>
              <li>Credential shapes and emails are masked on a best-effort basis. Secret values are never returned.</li>
            </ul>
            <button type="button" className="btn btn-ghost btn-sm" onClick={() => BrowserOpenURL(MCP_DOCS_URL)}>
              Full guide ↗
            </button>
          </section>
        </div>
      </div>
    </div>,
    document.body,
  )
}
