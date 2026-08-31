import { useCallback, useEffect, useRef } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import {
  CloseTerminal,
  ResizeTerminal,
  StartTerminal,
  WriteTerminal,
} from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { shellLabel } from '../lib/shellLabel'
import { normalizeTerminalAppearance, terminalXtermTheme } from '../lib/terminalAppearance'

/**
 * One xterm instance + PTY session for a terminal tab.
 */
export function TerminalTabPane({
  tab,
  active,
  open,
  cluster,
  shellPref = '',
  appearance = 'midnight',
  onStateChange,
}) {
  const containerRef = useRef(null)
  const termRef = useRef(null)
  const fitRef = useRef(null)
  const sessionRef = useRef(null)
  const pendingInputRef = useRef('')
  const startGenRef = useRef(0)
  const shellPrefRef = useRef(shellPref)
  const appearanceRef = useRef(normalizeTerminalAppearance(appearance))
  const activeRef = useRef(active)
  const mountedRef = useRef(false)
  activeRef.current = active
  shellPrefRef.current = shellPref
  appearanceRef.current = normalizeTerminalAppearance(appearance)

  const contextName = tab.contextName || cluster?.selectedContext || cluster?.currentContext || ''
  const namespace = tab.namespace || cluster?.selectedNamespace || ''
  const kubeconfig = cluster?.kubeconfigPath || ''

  const report = useCallback((patch) => {
    onStateChange?.(tab.id, patch)
  }, [onStateChange, tab.id])

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
    report({ sessionId: null, ready: false })
  }, [report])

  const startSession = useCallback(async () => {
    const gen = ++startGenRef.current
    report({ error: '', ready: false, shell: '' })

    if (!contextName) {
      report({ error: 'Select a cluster context before opening the terminal.' })
      await closeSession()
      return
    }

    const term = termRef.current
    const fit = fitRef.current
    if (!term || !fit) {
      report({ error: 'Terminal view is not ready yet. Try Restart.', ready: false })
      return
    }

    await closeSession()
    if (gen !== startGenRef.current) return

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
      term.reset()
      flushPendingInput(info.id)
      term.focus()
      report({
        sessionId: info.id,
        ready: true,
        error: '',
        shell: shellLabel(info.shell),
      })
    } catch (err) {
      if (gen !== startGenRef.current) return
      report({ sessionId: null, ready: false, error: String(err), shell: '' })
    }
  }, [closeSession, contextName, namespace, kubeconfig, report, flushPendingInput])

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
    term.loadAddon(fit)
    term.open(el)
    fit.fit()
    termRef.current = term
    fitRef.current = fit
    mountedRef.current = true

    const onData = term.onData((data) => {
      sendInput(data)
    })

    const offOut = EventsOn('terminal:output', (payload) => {
      if (!payload || payload.id !== sessionRef.current) return
      term.write(payload.data || '')
    })
    const offExit = EventsOn('terminal:exit', (payload) => {
      if (!payload || payload.id !== sessionRef.current) return
      sessionRef.current = null
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
      term.dispose()
      termRef.current = null
      fitRef.current = null
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
      term.focus()
    })
    return () => cancelAnimationFrame(frame)
  }, [open, active])

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

  return (
    <div
      className={`terminal-tab-pane ${active ? 'is-active' : ''}`}
      ref={containerRef}
      role="tabpanel"
      aria-hidden={!active}
    />
  )
}
