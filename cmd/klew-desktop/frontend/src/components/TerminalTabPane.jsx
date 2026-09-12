import { useCallback, useEffect, useRef, useState } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { SearchAddon } from '@xterm/addon-search'
import '@xterm/xterm/css/xterm.css'
import { buildTerminalFindOptions, countTerminalMatches } from '../lib/terminalFind'
import {
  CloseTerminal,
  ResizeTerminal,
  StartTerminal,
  WriteTerminal,
} from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { shellLabel } from '../lib/shellLabel'
import { normalizeTerminalAppearance, terminalXtermTheme } from '../lib/terminalAppearance'

function connectingLabel(contextName, namespace) {
  const scope = namespace ? `${contextName} / ${namespace}` : contextName
  return `Connecting to ${scope}`
}

function TerminalConnectingOverlay({ contextName, namespace }) {
  const [dots, setDots] = useState('')

  useEffect(() => {
    let frame = 0
    const id = window.setInterval(() => {
      frame = (frame + 1) % 4
      setDots('.'.repeat(frame))
    }, 400)
    return () => window.clearInterval(id)
  }, [])

  return (
    <div className="terminal-connecting" role="status" aria-live="polite">
      <span className="terminal-connecting-prompt" aria-hidden="true">●</span>
      <span className="terminal-connecting-text">
        {connectingLabel(contextName, namespace)}
        <span className="terminal-connecting-dots" aria-hidden="true">{dots || '\u00a0'}</span>
      </span>
      <span className="terminal-connecting-cursor" aria-hidden="true">▌</span>
    </div>
  )
}

/**
 * One xterm instance + PTY session for a terminal tab.
 */
export function TerminalTabPane({
  tab,
  active,
  focused = true,
  open,
  cluster,
  shellPref = '',
  appearance = 'midnight',
  onStateChange,
  onSearchRegister,
}) {
  const containerRef = useRef(null)
  const termRef = useRef(null)
  const fitRef = useRef(null)
  const searchRef = useRef(null)
  const sessionRef = useRef(null)
  const pendingInputRef = useRef('')
  const startGenRef = useRef(0)
  const shellPrefRef = useRef(shellPref)
  const appearanceRef = useRef(normalizeTerminalAppearance(appearance))
  const activeRef = useRef(active)
  const mountedRef = useRef(false)
  const connectingRef = useRef(false)
  const bootWrittenRef = useRef(false)
  const [showConnecting, setShowConnecting] = useState(false)
  activeRef.current = active
  shellPrefRef.current = shellPref
  appearanceRef.current = normalizeTerminalAppearance(appearance)

  const onSearchRegisterRef = useRef(onSearchRegister)
  onSearchRegisterRef.current = onSearchRegister

  const publishSearchApi = useCallback(() => {
    const search = searchRef.current
    const term = termRef.current
    const fit = fitRef.current
    if (!search || !term) return
    onSearchRegisterRef.current?.({
      findNext: (query, overrides) => {
        fit?.fit()
        return search.findNext(
          query,
          buildTerminalFindOptions(appearanceRef.current, overrides),
        )
      },
      findPrevious: (query, overrides) => {
        fit?.fit()
        return search.findPrevious(
          query,
          buildTerminalFindOptions(appearanceRef.current, overrides),
        )
      },
      clearDecorations: () => search.clearDecorations(),
      onResults: (cb) => search.onDidChangeResults(cb),
      countMatches: (query, options) => countTerminalMatches(term, query, options),
    })
  }, [])

  const contextName = tab.contextName || cluster?.selectedContext || cluster?.currentContext || ''
  const namespace = tab.namespace || cluster?.selectedNamespace || ''
  const kubeconfig = cluster?.kubeconfigPath || ''

  const report = useCallback((patch) => {
    onStateChange?.(tab.id, patch)
  }, [onStateChange, tab.id])

  const writeBootStatus = useCallback((term) => {
    if (!term) return
    term.write('\r\n\x1b[2m')
    term.write(`${connectingLabel(contextName, namespace)}…`)
    term.write('\x1b[0m')
    bootWrittenRef.current = true
  }, [contextName, namespace])

  const clearBootStatus = useCallback((term) => {
    if (!term || !bootWrittenRef.current) return
    term.write('\x1b[1A\x1b[2K')
    bootWrittenRef.current = false
  }, [])

  const beginConnecting = useCallback(() => {
    connectingRef.current = true
    setShowConnecting(true)
  }, [])

  const endConnecting = useCallback(() => {
    if (!connectingRef.current) return
    connectingRef.current = false
    setShowConnecting(false)
    clearBootStatus(termRef.current)
  }, [clearBootStatus])

  const flushPendingInput = useCallback((id) => {
    if (!id || !pendingInputRef.current) return
    const data = pendingInputRef.current
    pendingInputRef.current = ''
    WriteTerminal(id, data).catch(() => {})
  }, [])

  const sendInput = useCallback((data) => {
    const id = sessionRef.current
    if (!id) {
      pendingInputRef.current += data
      return
    }
    WriteTerminal(id, data).catch(() => {})
  }, [])

  const closeSession = useCallback(async () => {
    const id = sessionRef.current
    sessionRef.current = null
    if (id) {
      try {
        await CloseTerminal(id)
      } catch {
        /* ignore */
      }
    }
    endConnecting()
    report({ sessionId: null, ready: false })
  }, [report, endConnecting])

  const startSession = useCallback(async () => {
    const gen = ++startGenRef.current
    report({ error: '', ready: false, shell: '' })

    if (!contextName) {
      endConnecting()
      report({ error: 'Select a cluster context before opening the terminal.' })
      await closeSession()
      return
    }

    const term = termRef.current
    const fit = fitRef.current
    if (!term || !fit) {
      endConnecting()
      report({ error: 'Terminal view is not ready yet. Try Restart.', ready: false })
      return
    }

    await closeSession()
    if (gen !== startGenRef.current) return

    term.reset()
    beginConnecting()
    writeBootStatus(term)

    if (activeRef.current && fit) fit.fit()
    const cols = Math.max(term.cols || 0, 80)
    const rows = Math.max(term.rows || 0, 24)

    try {
      const info = await StartTerminal({
        kubeconfig,
        context: contextName,
        namespace,
        shell: shellPrefRef.current || '',
        cols,
        rows,
      })
      if (gen !== startGenRef.current) {
        await CloseTerminal(info.id).catch(() => {})
        return
      }
      sessionRef.current = info.id
      flushPendingInput(info.id)
      if (tab.initialInput && !tab.initialInputSent) {
        await WriteTerminal(info.id, tab.initialInput).catch(() => {})
        report({ initialInputSent: true })
      }
      term.focus()
      report({
        sessionId: info.id,
        ready: true,
        error: '',
        shell: shellLabel(info.shell),
      })
    } catch (err) {
      if (gen !== startGenRef.current) return
      endConnecting()
      report({ sessionId: null, ready: false, error: String(err), shell: '' })
    }
  }, [
    closeSession,
    contextName,
    namespace,
    kubeconfig,
    report,
    flushPendingInput,
    beginConnecting,
    writeBootStatus,
    endConnecting,
    tab.initialInput,
    tab.initialInputSent,
  ])

  useEffect(() => {
    const el = containerRef.current
    if (!el || !open) return undefined

    const term = new Terminal({
      cursorBlink: true,
      fontSize: 12,
      fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
      theme: terminalXtermTheme(appearanceRef.current),
      scrollback: 5000,
      disableStdin: false,
    })
    const fit = new FitAddon()
    const search = new SearchAddon()
    term.loadAddon(fit)
    term.loadAddon(search)
    term.open(el)
    fit.fit()
    termRef.current = term
    fitRef.current = fit
    searchRef.current = search
    publishSearchApi()
    mountedRef.current = true

    const onData = term.onData((data) => {
      sendInput(data)
    })

    const offOut = EventsOn('terminal:output', (payload) => {
      if (!payload || payload.id !== sessionRef.current) return
      if (connectingRef.current) {
        connectingRef.current = false
        setShowConnecting(false)
        if (bootWrittenRef.current) {
          term.write('\x1b[1A\x1b[2K')
          bootWrittenRef.current = false
        }
      }
      term.write(payload.data || '')
    })
    const offExit = EventsOn('terminal:exit', (payload) => {
      if (!payload || payload.id !== sessionRef.current) return
      sessionRef.current = null
      connectingRef.current = false
      setShowConnecting(false)
      bootWrittenRef.current = false
      report({ sessionId: null, ready: false, shell: '' })
    })

    const ro = new ResizeObserver(() => {
      if (!activeRef.current) return
      fit.fit()
      const id = sessionRef.current
      if (id && term.cols && term.rows) {
        ResizeTerminal(id, term.cols, term.rows).catch(() => {})
      }
    })
    ro.observe(el)

    // Defer first session until xterm has laid out (avoids 0-size PTY on open).
    const boot = requestAnimationFrame(() => {
      if (mountedRef.current) startSession()
    })

    return () => {
      mountedRef.current = false
      cancelAnimationFrame(boot)
      startGenRef.current += 1
      onData.dispose()
      offOut?.()
      offExit?.()
      ro.disconnect()
      closeSession()
      onSearchRegister?.(null)
      term.dispose()
      termRef.current = null
      fitRef.current = null
      searchRef.current = null
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps -- mount once per tab
  }, [open, tab.id])

  useEffect(() => {
    if (!open || !active || !termRef.current || !fitRef.current) return
    const fit = fitRef.current
    const term = termRef.current
    const frame = requestAnimationFrame(() => {
      fit.fit()
      const id = sessionRef.current
      if (id && term.cols && term.rows) {
        ResizeTerminal(id, term.cols, term.rows).catch(() => {})
      }
      if (focused) term.focus()
    })
    return () => cancelAnimationFrame(frame)
  }, [open, active, focused])

  useEffect(() => {
    if (!open || tab.restartToken == null) return undefined
    const frame = requestAnimationFrame(() => {
      startSession()
    })
    return () => cancelAnimationFrame(frame)
  // eslint-disable-next-line react-hooks/exhaustive-deps -- restart token only
  }, [open, tab.restartToken])

  useEffect(() => {
    const term = termRef.current
    if (!term || !open) return
    const theme = terminalXtermTheme(appearance)
    term.options.theme = theme
    if (typeof term.setOption === 'function') {
      term.setOption('theme', theme)
    }
    term.refresh(0, Math.max(term.rows - 1, 0))
  }, [appearance, open])

  useEffect(() => {
    if (!showConnecting) return undefined
    const timeout = window.setTimeout(() => {
      endConnecting()
    }, 8000)
    return () => window.clearTimeout(timeout)
  }, [showConnecting, endConnecting, tab.id])

  return (
    <div
      className={[
        'terminal-tab-pane',
        active ? 'is-active' : '',
        showConnecting ? 'is-connecting' : '',
      ].filter(Boolean).join(' ')}
      role="tabpanel"
      aria-hidden={!active}
    >
      <div ref={containerRef} className="terminal-xterm-host" />
      {showConnecting && active && (
        <TerminalConnectingOverlay contextName={contextName} namespace={namespace} />
      )}
    </div>
  )
}
