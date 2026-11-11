import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { MCPSetupInfo } from '../../../wailsjs/go/main/App'
import { BrowserOpenURL } from '../../../wailsjs/runtime/runtime'
import { useTerminalLaunch } from '../../context/TerminalLaunchContext.jsx'
import { LogoMark } from '../Logo.jsx'
import {
  KLEW_CLI_DOCS_URL,
  KLEW_CLI_GO_INSTALL,
  KLEW_CLI_BREW_UPDATE,
  KLEW_CLI_INSTALL,
  installThenInvestigateLine,
  investigateCommand,
} from '../../lib/klewCli.js'

function copyText(text) {
  if (!text || !navigator.clipboard?.writeText) return Promise.resolve(false)
  return navigator.clipboard.writeText(text).then(() => true).catch(() => false)
}

function CopyLine({ text, copiedKey, copied, onCopied, leading = null }) {
  return (
    <div className="klew-cli-code">
      {leading}
      <code>{text}</code>
      <button
        type="button"
        className="btn btn-ghost btn-sm"
        onClick={() => copyText(text).then((ok) => ok && onCopied(copiedKey))}
      >
        {copied === copiedKey ? 'Copied' : 'Copy'}
      </button>
    </div>
  )
}

function InvestigateBlock({ investigateCmd, copiedKey, copied, onCopied }) {
  return (
    <>
      <div className="klew-cli-method-chips">
        <span className="klew-cli-chip tone-run klew-cli-chip-brand">
          <LogoMark className="klew-cli-chip-logo" />
          Investigate
        </span>
      </div>
      <CopyLine
        text={investigateCmd}
        copiedKey={copiedKey}
        copied={copied}
        onCopied={onCopied}
        leading={<LogoMark className="klew-cli-code-logo" />}
      />
    </>
  )
}

const UPDATE_METHODS = [
  { id: 'brew', label: 'Homebrew', cmd: KLEW_CLI_BREW_UPDATE, title: 'Update klew' },
  { id: 'go', label: 'Go', cmd: KLEW_CLI_GO_INSTALL, title: 'go install klew' },
]

/** Installed view: the same chips, as update / reinstall. */
function UpdateChips({ onInstall, canLaunch }) {
  return (
    <div className="klew-cli-install-row klew-cli-update-row">
      <span className="klew-cli-install-label">Update or reinstall with</span>
      {UPDATE_METHODS.map((m) => (
        <button
          key={m.id}
          type="button"
          className="klew-cli-install-chip"
          onClick={() => onInstall(m)}
          disabled={!canLaunch}
          title={canLaunch ? `Run "${m.cmd}" in the terminal` : 'Terminal unavailable'}
        >
          {m.label}
        </button>
      ))}
    </div>
  )
}

const INSTALL_METHODS = [
  { id: 'brew', label: 'Homebrew', cmd: KLEW_CLI_INSTALL, title: 'Install klew' },
  { id: 'go', label: 'Go', cmd: KLEW_CLI_GO_INSTALL, title: 'go install klew' },
]

/**
 * Install chips: a click runs that install in the terminal dock and, once it
 * succeeds, leaves the investigate command at the prompt. Copy lines stay for
 * anyone who prefers to run the commands themselves.
 */
function InstallChips({ investigateCmd, copied, onCopied, onInstall, canLaunch }) {
  return (
    <div className="klew-cli-method">
      <div className="klew-cli-install-row">
        <span className="klew-cli-install-label">Install with</span>
        {INSTALL_METHODS.map((m) => (
          <button
            key={m.id}
            type="button"
            className="klew-cli-install-chip"
            onClick={() => onInstall(m)}
            disabled={!canLaunch}
            title={canLaunch ? `Run "${m.cmd}" in the terminal` : 'Terminal unavailable: copy the command below'}
          >
            {m.label}
          </button>
        ))}
      </div>
      {INSTALL_METHODS.map((m) => (
        <CopyLine
          key={m.id}
          text={m.cmd}
          copiedKey={`${m.id}-install`}
          copied={copied}
          onCopied={onCopied}
        />
      ))}
      <p className="klew-cli-popover-body muted klew-cli-then">
        After it installs, this is waiting at the prompt:
      </p>
      <InvestigateBlock
        investigateCmd={investigateCmd}
        copiedKey="after-investigate"
        copied={copied}
        onCopied={onCopied}
      />
    </div>
  )
}

/**
 * Contexts-strip control for the standalone `klew` command.
 * Always opens a popover (so the click is never a silent no-op).
 */
export function KlewCliButton({
  contextName = '',
  namespace = '',
  className = '',
  buttonClassName = 'btn btn-sm',
  children = 'CLI',
}) {
  const launch = useTerminalLaunch()
  const [info, setInfo] = useState(null)
  const [loadingInfo, setLoadingInfo] = useState(false)
  const [open, setOpen] = useState(false)
  const [copied, setCopied] = useState('')
  const [coords, setCoords] = useState({ top: 72, right: 16 })
  const anchorRef = useRef(null)
  const panelRef = useRef(null)

  const standalonePath = String(info?.standalonePath || '').trim()
  const installed = Boolean(standalonePath)
  const investigate = investigateCommand({
    klewPath: 'klew',
    context: contextName,
    namespace,
  })

  // Re-check on every open, so a just-finished install shows as installed.
  useEffect(() => {
    if (!open) return undefined
    let cancelled = false
    setLoadingInfo(true)
    MCPSetupInfo()
      .then((next) => {
        if (!cancelled) setInfo(next || {})
      })
      .catch(() => {
        if (!cancelled) setInfo({})
      })
      .finally(() => {
        if (!cancelled) setLoadingInfo(false)
      })
    return () => { cancelled = true }
  }, [open])

  useEffect(() => {
    if (!copied) return undefined
    const t = setTimeout(() => setCopied(''), 1400)
    return () => clearTimeout(t)
  }, [copied])

  useLayoutEffect(() => {
    if (!open || !anchorRef.current) return
    const rect = anchorRef.current.getBoundingClientRect()
    setCoords({
      top: Math.round(rect.bottom + 6),
      right: Math.max(12, Math.round(window.innerWidth - rect.right)),
    })
  }, [open])

  useEffect(() => {
    if (!open) return undefined
    function onDoc(e) {
      if (anchorRef.current?.contains(e.target)) return
      if (panelRef.current?.contains(e.target)) return
      setOpen(false)
    }
    function onKey(e) {
      if (e.key === 'Escape') setOpen(false)
    }
    const t = window.setTimeout(() => {
      document.addEventListener('mousedown', onDoc)
    }, 0)
    document.addEventListener('keydown', onKey, true)
    return () => {
      window.clearTimeout(t)
      document.removeEventListener('mousedown', onDoc)
      document.removeEventListener('keydown', onKey, true)
    }
  }, [open])

  function installInTerminal(method) {
    if (!launch) return
    setOpen(false)
    launch({
      title: method.title,
      context: contextName || '',
      namespace: '',
      initialInput: installThenInvestigateLine(method.cmd, investigate),
    })
  }

  function runInTerminal(line, title) {
    if (!launch) return
    setOpen(false)
    launch({
      title,
      context: contextName || '',
      namespace: installed ? (namespace || '') : '',
      initialInput: `${line} `,
    })
  }

  return (
    <div className={['klew-cli-anchor', className].filter(Boolean).join(' ')} ref={anchorRef}>
      <button
        type="button"
        className={[buttonClassName, open ? 'is-open' : ''].filter(Boolean).join(' ')}
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        aria-haspopup="dialog"
        title="klew CLI"
      >
        {children}
      </button>

      {open && createPortal(
        <div
          ref={panelRef}
          className="klew-cli-popover"
          role="dialog"
          aria-label="klew CLI"
          style={{ top: coords.top, right: coords.right }}
        >
          {loadingInfo && !info ? (
            <p className="klew-cli-popover-body muted">Checking for klew…</p>
          ) : installed ? (
            <>
              <p className="klew-cli-popover-body muted">
                Opens the terminal dock with this command ready to edit and run.
              </p>
              <InvestigateBlock
                investigateCmd={investigate}
                copiedKey="investigate"
                copied={copied}
                onCopied={setCopied}
              />
              <div className="klew-cli-popover-actions">
                {launch && (
                  <button
                    type="button"
                    className="btn btn-sm"
                    onClick={() => runInTerminal(investigate, 'klew investigate')}
                  >
                    Open terminal
                  </button>
                )}
                <button
                  type="button"
                  className="btn btn-ghost btn-sm"
                  onClick={() => BrowserOpenURL(KLEW_CLI_DOCS_URL)}
                >
                  CLI docs ↗
                </button>
              </div>
              <UpdateChips onInstall={installInTerminal} canLaunch={Boolean(launch)} />
            </>
          ) : (
            <>
              <p className="klew-cli-popover-title">klew is not installed</p>
              <p className="klew-cli-popover-body muted">
                Desktop Investigate works without it. Pick how to install; it runs in the terminal dock.
              </p>
              <InstallChips
                investigateCmd={investigate}
                copied={copied}
                onCopied={setCopied}
                canLaunch={Boolean(launch)}
                onInstall={installInTerminal}
              />
              <div className="klew-cli-popover-actions">
                <button
                  type="button"
                  className="btn btn-ghost btn-sm"
                  onClick={() => BrowserOpenURL(KLEW_CLI_DOCS_URL)}
                >
                  CLI docs ↗
                </button>
              </div>
            </>
          )}
        </div>,
        document.body,
      )}
    </div>
  )
}
