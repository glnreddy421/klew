import { useCallback, useEffect, useRef } from 'react'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { SearchAddon } from '@xterm/addon-search'
import { Unicode11Addon } from '@xterm/addon-unicode11'
import '@xterm/xterm/css/xterm.css'
import { buildTerminalFindOptions, countTerminalMatches, runTerminalSearch } from '../lib/terminalFind'
import {
  CloseTerminal,
  ResizeTerminal,
  StartTerminal,
  WriteTerminal,
} from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { shellLabel } from '../lib/shellLabel'
import { terminalNotice } from '../lib/terminalBanner'
import { terminalXtermTheme } from '../lib/terminalAppearance'
import { waitForShellQuiet } from '../lib/shellReady'
import { createSerialWriter } from '../lib/serialWriter'

const MAX_EARLY_OUTPUT_CHUNKS = 256

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
  appTheme = '',
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
  const restartSeenRef = useRef(tab.restartToken)
  const startingRef = useRef(false)
  const earlyOutputRef = useRef([])
  const lastOutputAtRef = useRef(0)
  const shellPrefRef = useRef(shellPref)
  const activeRef = useRef(active)
  const mountedRef = useRef(false)
  const openedRef = useRef(false)
  activeRef.current = active
  shellPrefRef.current = shellPref
  const appThemeRef = useRef(appTheme)
  appThemeRef.current = appTheme

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
        return runTerminalSearch(() => search.findNext(
          query,
          buildTerminalFindOptions(appThemeRef.current, overrides),
        ))
      },
      findPrevious: (query, overrides) => {
        fit?.fit()
        return runTerminalSearch(() => search.findPrevious(
          query,
          buildTerminalFindOptions(appThemeRef.current, overrides),
        ))
      },
      clearDecorations: () => search.clearDecorations(),
      onResults: (cb) => search.onDidChangeResults(cb),
      countMatches: (query, options) => countTerminalMatches(term, query, options),
    })
  }, [])

  // Pin to the tab's context for the life of the session. Do not follow the
  // app's active context — switching clusters must not retarget an open shell.
  const contextName = String(tab.contextName || '').trim()
    || String(cluster?.selectedContext || cluster?.currentContext || '').trim()
  const namespace = tab.namespace || ''
  const kubeconfig = cluster?.kubeconfigPath || ''

  const report = useCallback((patch) => {
    onStateChange?.(tab.id, patch)
  }, [onStateChange, tab.id])

  // One write in flight at a time, so key presses and pastes reach the shell in order.
  const writerRef = useRef(null)
  if (!writerRef.current) {
    writerRef.current = createSerialWriter((data) => {
      const id = sessionRef.current
      return id ? WriteTerminal(id, data) : Promise.resolve()
    })
  }

  const flushPendingInput = useCallback((id) => {
    if (!id || !pendingInputRef.current) return
    const data = pendingInputRef.current
    pendingInputRef.current = ''
    writerRef.current.push(data)
  }, [])

  const sendInput = useCallback((data) => {
    if (!sessionRef.current) {
      pendingInputRef.current += data
      return
    }
    writerRef.current.push(data)
  }, [])

  // Tell the shell the size xterm actually has. Only a visible pane can be
  // measured; a hidden one keeps its last size until it is shown again.
  const lastSizeRef = useRef('')
  const syncSize = useCallback(() => {
    const el = containerRef.current
    const term = termRef.current
    const fit = fitRef.current
    if (!el || !term || !fit || !el.clientWidth || !el.clientHeight) return
    fit.fit()
    const id = sessionRef.current
    if (!id || !term.cols || !term.rows) return
    const key = `${id}:${term.cols}x${term.rows}`
    if (key === lastSizeRef.current) return
    lastSizeRef.current = key
    ResizeTerminal(id, term.cols, term.rows).catch(() => {})
  }, [])

  const closeSession = useCallback(async () => {
    const id = sessionRef.current
    sessionRef.current = null
    writerRef.current?.clear()
    lastSizeRef.current = ''
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

    const term = termRef.current
    const fit = fitRef.current
    if (!term || !fit) {
      report({ error: 'Terminal view is not ready yet. Try Restart.', ready: false })
      return
    }

    await closeSession()
    if (gen !== startGenRef.current) return

    term.reset()

    if (activeRef.current && fit) fit.fit()
    const cols = Math.max(term.cols || 0, 80)
    const rows = Math.max(term.rows || 0, 24)

    earlyOutputRef.current = []
    lastOutputAtRef.current = 0
    startingRef.current = true
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
      startingRef.current = false
      sessionRef.current = info.id
      // The pane may have changed size while the shell started (the dock opening).
      lastSizeRef.current = `${info.id}:${cols}x${rows}`
      syncSize()
      // The shell prints its prompt before StartTerminal resolves; replay it.
      const early = earlyOutputRef.current.filter((c) => c.id === info.id)
      earlyOutputRef.current = []
      const notice = terminalNotice(info)
      if (notice) term.write(notice)
      for (const chunk of early) term.write(chunk.data)
      flushPendingInput(info.id)
      term.focus()
      report({
        sessionId: info.id,
        ready: true,
        error: '',
        shell: shellLabel(info.shell),
        shellNamespace: info.namespace || '',
        // Persist the context this PTY was started with (including fallback).
        contextName: info.context || contextName || tab.contextName || '',
      })
      if (tab.initialInput && !tab.initialInputSent) {
        // Let the shell finish starting first, or zsh draws the line garbled.
        // A size change makes the shell redraw its prompt; wait that out too.
        await waitForShellQuiet(() => lastOutputAtRef.current)
        syncSize()
        await waitForShellQuiet(() => lastOutputAtRef.current, { quietMs: 200, maxMs: 2000 })
        if (gen !== startGenRef.current || sessionRef.current !== info.id) return
        writerRef.current.push(tab.initialInput)
        report({ initialInputSent: true })
      }
    } catch (err) {
      if (gen !== startGenRef.current) return
      startingRef.current = false
      earlyOutputRef.current = []
      report({ sessionId: null, ready: false, error: String(err), shell: '' })
    }
  }, [
    closeSession,
    contextName,
    namespace,
    kubeconfig,
    report,
    flushPendingInput,
    syncSize,
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
      theme: terminalXtermTheme(appThemeRef.current),
      scrollback: 5000,
      disableStdin: false,
      // SearchAddon highlights matches via registerDecoration, a proposed API in
      // xterm 5 that throws unless this is set — find would match nothing.
      allowProposedApi: true,
    })
    const fit = new FitAddon()
    const search = new SearchAddon()
    term.loadAddon(fit)
    term.loadAddon(search)
    // Modern character widths (emoji, symbols). With xterm's Unicode 6 default,
    // full-screen programs like Claude Code that assume newer widths drift and
    // leave stray characters.
    term.loadAddon(new Unicode11Addon())
    term.unicode.activeVersion = '11'
    termRef.current = term
    fitRef.current = fit
    searchRef.current = search
    mountedRef.current = true
    openedRef.current = false
    restartSeenRef.current = tab.restartToken

    // xterm measures its font when opened; inside a display:none pane that is 0×0 and
    // the terminal stays blank. Attach (and start the shell) once the pane has a size.
    let boot = 0
    const attach = () => {
      if (openedRef.current || !el.clientWidth || !el.clientHeight) return
      openedRef.current = true
      term.open(el)
      fit.fit()
      publishSearchApi()
      boot = requestAnimationFrame(() => {
        if (mountedRef.current) startSession()
      })
    }

    const onData = term.onData((data) => {
      sendInput(data)
    })

    const offOut = EventsOn('terminal:output', (payload) => {
      if (!payload) return
      if (payload.id !== sessionRef.current) {
        if (startingRef.current && earlyOutputRef.current.length < MAX_EARLY_OUTPUT_CHUNKS) {
          earlyOutputRef.current.push({ id: payload.id, data: payload.data || '' })
          lastOutputAtRef.current = Date.now()
        }
        return
      }
      lastOutputAtRef.current = Date.now()
      term.write(payload.data || '')
    })
    const offExit = EventsOn('terminal:exit', (payload) => {
      if (!payload || payload.id !== sessionRef.current) return
      sessionRef.current = null
      report({ sessionId: null, ready: false, shell: '' })
    })

    const ro = new ResizeObserver(() => {
      if (!openedRef.current) {
        attach()
        return
      }
      // Any visible pane, active or not: a terminal opened from Home is never
      // marked active, and its shell kept the startup size.
      syncSize()
    })
    ro.observe(el)
    attach()

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
    if (!open || !active || !openedRef.current || !termRef.current || !fitRef.current) return
    const term = termRef.current
    const frame = requestAnimationFrame(() => {
      syncSize()
      if (focused) term.focus()
    })
    return () => cancelAnimationFrame(frame)
  }, [open, active, focused])

  // The mount effect already boots a session; only explicit restarts after that start another.
  useEffect(() => {
    if (!open || tab.restartToken == null) return undefined
    if (restartSeenRef.current === tab.restartToken) return undefined
    restartSeenRef.current = tab.restartToken
    // Not attached yet: the first session starts on attach.
    if (!openedRef.current) return undefined
    const frame = requestAnimationFrame(() => {
      startSession()
    })
    return () => cancelAnimationFrame(frame)
  // eslint-disable-next-line react-hooks/exhaustive-deps -- restart token only
  }, [open, tab.restartToken])

  useEffect(() => {
    const term = termRef.current
    if (!term || !open) return
    const theme = terminalXtermTheme(appTheme)
    term.options.theme = theme
    if (typeof term.setOption === 'function') {
      term.setOption('theme', theme)
    }
    if (openedRef.current) term.refresh(0, Math.max(term.rows - 1, 0))
  }, [appTheme, open])

  return (
    <div
      className={`terminal-tab-pane ${active ? 'is-active' : ''}`}
      role="tabpanel"
      aria-hidden={!active}
    >
      <div ref={containerRef} className="terminal-xterm-host" />
    </div>
  )
}
