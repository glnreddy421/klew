import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  StartInvestigation,
  StopInvestigation,
  DiscoverMatches,
  GetView,
  OpenNewWindow,
  StartLogTail,
  StopLogTail,
  PauseLogTail,
  ResumeLogTail,
  ClearLogs,
  SetAutoRefresh,
  SetPollEverySec,
} from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime/runtime'
import { emptyView } from './lib/constants'
import { useCluster } from './hooks/useCluster'
import { useClusterStatus } from './hooks/useClusterStatus'
import { HOME_NAV, normalizeNavEntry, useNavigationHistory } from './hooks/useNavigationHistory'
import { useStreamPanel, PANEL_NORMAL } from './hooks/useStreamPanel'
import { useTheme } from './hooks/useTheme'
import { usePreferences } from './hooks/usePreferences'
import { startOptionsFromPreferences } from './lib/preferences'
import { resolveTerminalShellPref } from './lib/terminalShell'
import { useIdleAutoStop, formatIdleDuration } from './hooks/useIdleAutoStop'
import { AppShell } from './components/shell/AppShell.jsx'
import { ResourcesWorkbenchRoot } from './views/ResourcesWorkbenchView'
import { ScopeBrowseProvider } from './context/ScopeBrowseContext.jsx'
import { defaultRelations } from './components/shell/explorers/ExplorerPanels.jsx'
import { LiveStreamPanel } from './components/LiveStreamPanel'
import { TerminalShellModal } from './components/TerminalShellModal'
import { ScopePickerModal } from './components/incident/ScopePickerModal'
import { MainContent } from './views/MainContent'
import {
  pickDefaultFocus,
  deriveMatchRows,
  getMatchedObjects,
  buildInvestigationQuery,
} from './lib/matches'
import { buildFocusScope, emptyFocusScope } from './lib/focusScope'
import { isEditableTarget } from './lib/keyboard'
import {
  isBlankInvestigationQuery,
  normalizeInvestigationQuery,
} from './lib/investigationQuery'

export default function App() {
  const [view, setView] = useState(emptyView())
  const [tab, setTab] = useState('incident')
  const [nodesFocus, setNodesFocus] = useState('cluster')
  const [settingsSection, setSettingsSection] = useState('general')
  const navigation = useNavigationHistory(HOME_NAV)

  const applyNavEntry = useCallback((entry) => {
    if (!entry) return
    setTab(entry.tab)
    setNodesFocus(entry.nodesFocus || HOME_NAV.nodesFocus)
    setSettingsSection(entry.settingsSection || HOME_NAV.settingsSection)
  }, [])

  const navigateTo = useCallback((target) => {
    const entry = normalizeNavEntry(
      typeof target === 'object' ? target : { tab: target },
      { tab, nodesFocus, settingsSection },
    )
    applyNavEntry(entry)
    navigation.push(entry)
  }, [tab, nodesFocus, settingsSection, applyNavEntry, navigation])

  const handleNavBack = useCallback(() => {
    applyNavEntry(navigation.back())
  }, [navigation, applyNavEntry])

  const handleNavForward = useCallback(() => {
    applyNavEntry(navigation.forward())
  }, [navigation, applyNavEntry])

  const handleNavHome = useCallback(() => {
    navigateTo(HOME_NAV)
  }, [navigateTo])
  const [query, setQuery] = useState('')
  const [running, setRunning] = useState(false)
  const [starting, setStarting] = useState(false)
  const [activeQuery, setActiveQuery] = useState('')
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [gatherBusy, setGatherBusy] = useState(false)
  const [gatherError, setGatherError] = useState('')
  const [scopePicker, setScopePicker] = useState({
    open: false,
    matches: [],
    query: '',
    mode: 'initial',
  })

  const [focusKey, setFocusKey] = useState(null)
  const [focusPinned, setFocusPinned] = useState(false)
  const [inspectKey, setInspectKey] = useState(null)

  const { cluster, syncing, syncNow, setContext, setNamespace } = useCluster()
  const { clusterStatus } = useClusterStatus(cluster)
  const stream = useStreamPanel()
  const [terminalShellPickerOpen, setTerminalShellPickerOpen] = useState(false)
  const [terminalMounted, setTerminalMounted] = useState(false)
  const [pendingNavAfterShell, setPendingNavAfterShell] = useState(null)
  const [terminalShellRestartToken, setTerminalShellRestartToken] = useState(0)
  const { themeId, setTheme } = useTheme()
  const { prefs, setPreferences } = usePreferences()
  const activeQueryRef = useRef('')

  useEffect(() => {
    activeQueryRef.current = activeQuery
  }, [activeQuery])

  // Live investigations emit state very often (log lines). Coalesce UI updates so
  // React is not forced to reconcile the whole shell on every tick.
  const pendingViewRef = useRef(null)
  const viewFlushTimerRef = useRef(null)

  const flushView = useCallback(() => {
    viewFlushTimerRef.current = null
    const payload = pendingViewRef.current
    pendingViewRef.current = null
    if (!payload) return
    const incoming = normalizeInvestigationQuery(payload?.summary?.query ?? '')
    const expected = normalizeInvestigationQuery(activeQueryRef.current)
    if (expected && incoming && incoming !== expected) {
      return
    }
    setView(payload)
  }, [])

  const applyView = useCallback((payload) => {
    pendingViewRef.current = payload
    if (viewFlushTimerRef.current != null) return
    viewFlushTimerRef.current = window.setTimeout(flushView, 350)
  }, [flushView])

  useEffect(() => {
    GetView().then((payload) => {
      pendingViewRef.current = payload
      flushView()
    }).catch(() => {})
    const off = EventsOn('state', applyView)
    return () => {
      off?.()
      if (viewFlushTimerRef.current != null) {
        clearTimeout(viewFlushTimerRef.current)
        viewFlushTimerRef.current = null
      }
    }
  }, [applyView, flushView])

  useEffect(() => {
    function onKeyDown(e) {
      if (!(e.metaKey || e.ctrlKey) || e.key.toLowerCase() !== 'r') return
      if (isEditableTarget(e.target)) return
      if (syncing || running) return
      e.preventDefault()
      syncNow()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [syncNow, syncing, running])

  useEffect(() => {
    const offSettings = EventsOn('menu:settings', () => {
      navigateTo({ tab: 'settings', settingsSection: 'general' })
    })
    const offHelp = EventsOn('menu:help', () => {
      navigateTo({ tab: 'settings', settingsSection: 'help' })
    })
    return () => {
      offSettings?.()
      offHelp?.()
    }
  }, [navigateTo])

  const applyTerminalShell = useCallback((choice) => {
    setPreferences({
      terminalShell: resolveTerminalShellPref(choice),
      terminalShellPrompted: true,
    })
    setTerminalShellPickerOpen(false)
    if (terminalMounted) {
      setTerminalShellRestartToken(Date.now())
    }
    if (pendingNavAfterShell) {
      if (pendingNavAfterShell.tab === 'terminal') {
        setTerminalMounted(true)
      }
      navigateTo(pendingNavAfterShell)
      setPendingNavAfterShell(null)
    }
  }, [setPreferences, terminalMounted, pendingNavAfterShell, navigateTo])

  const openShellPicker = useCallback((afterNav = null) => {
    setPendingNavAfterShell(afterNav)
    setTerminalShellPickerOpen(true)
  }, [])

  const navigateToTerminal = useCallback(() => {
    if (!prefs.terminalShellPrompted) {
      openShellPicker({ tab: 'terminal' })
      return
    }
    setTerminalMounted(true)
    navigateTo('terminal')
  }, [prefs.terminalShellPrompted, openShellPicker, navigateTo])

  const handleOpenTerminalShellPicker = useCallback(() => {
    openShellPicker(null)
  }, [openShellPicker])

  const handleTerminalShellConfirm = useCallback((choice) => {
    applyTerminalShell(choice)
  }, [applyTerminalShell])

  const handleTerminalShellChange = useCallback(() => {
    if (terminalMounted) {
      setTerminalShellRestartToken(Date.now())
    }
  }, [terminalMounted])

  const handleNavigate = useCallback((target) => {
    const entry = normalizeNavEntry(
      typeof target === 'object' ? target : { tab: target },
      { tab, nodesFocus, settingsSection },
    )
    if (entry.tab === 'terminal') {
      navigateToTerminal()
      return
    }
    navigateTo(entry)
  }, [tab, nodesFocus, settingsSection, navigateTo, navigateToTerminal])

  useEffect(() => {
    const offTerminal = EventsOn('menu:terminal', () => {
      navigateToTerminal()
    })
    return () => offTerminal?.()
  }, [navigateToTerminal])

  const logTailEngaged = (view.state?.logTailPods?.length ?? 0) > 0
  const logTailPaused = Boolean(view.state?.logTailPaused)

  const finishStopRef = useRef(async () => {})

  finishStopRef.current = async ({ idleMessage } = {}) => {
    await StopInvestigation()
    setRunning(false)
    setStarting(false)
    setGatherError('')
    setActiveQuery('')
    setFocusKey(null)
    setFocusPinned(false)
    setView(emptyView())
    stream.resetFilters()
    stream.setSearch('')
    stream.setFollow(Boolean(prefs.followLogsByDefault))
    if (!prefs.rememberLastQuery) {
      setQuery('')
    }
    if (idleMessage) {
      setError('')
      setNotice(idleMessage)
    }
  }

  const handleIdleAutoStop = useCallback(async () => {
    const label = formatIdleDuration(prefs.idleAutoStopMin)
    await finishStopRef.current({
      idleMessage: `Investigation ended after ${label} of inactivity.`,
    })
  }, [prefs.idleAutoStopMin])

  const { bumpActivity } = useIdleAutoStop({
    enabled: prefs.idleAutoStop,
    idleMinutes: prefs.idleAutoStopMin,
    active: running,
    onIdle: handleIdleAutoStop,
  })

  useEffect(() => {
    if (!running) return
    SetAutoRefresh(prefs.autoRefresh).catch(() => {})
    SetPollEverySec(prefs.refreshSec).catch(() => {})
  }, [running, prefs.autoRefresh, prefs.refreshSec])

  const matchRows = useMemo(
    () => deriveMatchRows(view, getMatchedObjects(view)),
    [view],
  )

  const focusRow = useMemo(() => {
    const found = matchRows.find((r) => r.key === focusKey)
    if (found) return found
    // Chain rows (ConfigMap/Secret/Pod) may not be in original matches — synthesize a focus ref.
    if (focusPinned && focusKey) {
      const [kind, ...rest] = focusKey.split('/')
      const name = rest.join('/')
      if (kind && name) {
        return {
          key: focusKey,
          ref: { kind, name },
          kind,
          name,
          status: 'healthy',
          signal: null,
          score: 0,
        }
      }
    }
    return null
  }, [matchRows, focusKey, focusPinned])

  const drillDown = useMemo(
    () => (focusPinned && focusRow ? buildFocusScope(view, focusRow) : emptyFocusScope()),
    [focusPinned, focusRow, view],
  )

  const handleFocusChange = useCallback((key, opts = {}) => {
    setFocusKey(key)
    setFocusPinned(Boolean(opts.pinned))
    if (opts.pinned) {
      stream.openPanel(PANEL_NORMAL)
      stream.setFollow(true)
    }
  }, [stream])

  const handleClearFocus = useCallback(() => {
    setFocusPinned(false)
    setFocusKey(null)
  }, [])

  const runInvestigation = useCallback(async (q) => {
    const trimmed = normalizeInvestigationQuery(q)
    setNotice('')
    setView(emptyView())
    const opts = startOptionsFromPreferences(prefs, {
      query: trimmed,
      namespace: cluster.selectedNamespace,
      context: cluster.selectedContext,
      kubeconfig: cluster.kubeconfigPath,
    })
    // StartInvestigation stops any prior session — no separate Stop click needed.
    await StartInvestigation(opts)
    setRunning(true)
    setGatherError('')
    setActiveQuery(trimmed)
    setFocusKey(null)
    setFocusPinned(false)
    stream.resetFilters()
    if (prefs.openStreamOnInvestigate) {
      stream.openPanel(PANEL_NORMAL)
    }
    const next = await GetView()
    applyView(next)
    bumpActivity()
  }, [cluster, stream, prefs, applyView, bumpActivity])

  const handleStartGather = useCallback(async ({ podNames, lineSearch }) => {
    setGatherBusy(true)
    setGatherError('')
    const names = [...(podNames || [])]
    if (!names.length) {
      setGatherError('Select at least one pod')
      setGatherBusy(false)
      return
    }
    try {
      await StartLogTail({ podNames: names })
      const q = String(lineSearch || '').trim()
      stream.setSearch(q)
      stream.setMode('logs')
      const next = await GetView()
      applyView(next)
      const allowed = next.state?.logTailPods?.length ? next.state.logTailPods : names
      stream.selectPods(allowed, { pinned: true })
      stream.setFollow(!q && Boolean(prefs.followLogsByDefault))
      stream.openPanel(PANEL_NORMAL)
    } catch (err) {
      setGatherError(String(err))
    } finally {
      setGatherBusy(false)
    }
  }, [applyView, stream, prefs.followLogsByDefault])

  const handleStopGather = useCallback(async () => {
    try {
      await StopLogTail()
    } catch {
      /* ignore */
    }
    const next = await GetView()
    applyView(next)
  }, [applyView])

  const handleToggleLogTailPause = useCallback(async () => {
    setGatherError('')
    try {
      if (logTailPaused) {
        await ResumeLogTail()
        stream.setFollow(!stream.search.trim() && Boolean(prefs.followLogsByDefault))
      } else {
        await PauseLogTail()
        stream.setFollow(false)
      }
      const next = await GetView()
      applyView(next)
    } catch (err) {
      setGatherError(String(err))
    }
  }, [applyView, logTailPaused, stream, prefs.followLogsByDefault])

  const handleClearLogs = useCallback(async () => {
    try {
      await ClearLogs()
      await StopLogTail()
      stream.setSearch('')
      const next = await GetView()
      applyView(next)
    } catch (err) {
      setGatherError(String(err))
    }
  }, [applyView, stream])

  function beginInvestigationTransition(nextQuery) {
    setView(emptyView())
    setActiveQuery(normalizeInvestigationQuery(nextQuery))
    setFocusKey(null)
    setFocusPinned(false)
    stream.resetFilters()
    stream.setSearch('')
  }

  async function onStart(e) {
    e?.preventDefault?.()
    if (starting) return
    if (!cluster.selectedNamespace) return

    const q = normalizeInvestigationQuery(query)
    if (q !== query) {
      setQuery(q)
    }

    setError('')
    navigateTo('incident')
    beginInvestigationTransition(q)
    setStarting(true)
    try {
      const matches = await DiscoverMatches({
        query: q,
        namespace: cluster.selectedNamespace,
        context: cluster.selectedContext,
        kubeconfig: cluster.kubeconfigPath,
      })

      if (isBlankInvestigationQuery(q)) {
        setScopePicker({
          open: true,
          matches,
          query: '',
          mode: running ? 'narrow' : 'initial',
        })
        return
      }

      if (!matches.length) {
        setError(`No resources matched "${q}" in ${cluster.selectedNamespace}.`)
        setActiveQuery('')
        setView(emptyView())
        return
      }

      setRunning(true)
      await runInvestigation(q)
    } catch (err) {
      setError(String(err))
      setRunning(false)
      setActiveQuery('')
      setView(emptyView())
    } finally {
      setStarting(false)
    }
  }

  async function onScopeConfirm({ selectedKeys, investigateAll }) {
    const { query: pickerQuery, matches } = scopePicker
    setScopePicker((prev) => ({ ...prev, open: false }))
    const effectiveQuery = buildInvestigationQuery(
      pickerQuery,
      investigateAll ? null : selectedKeys,
      matches,
    )
    setError('')
    beginInvestigationTransition(effectiveQuery)
    setStarting(true)
    setRunning(true)
    navigateTo('incident')
    try {
      await runInvestigation(effectiveQuery)
    } catch (err) {
      setError(String(err))
      setRunning(false)
      setActiveQuery('')
      setView(emptyView())
    } finally {
      setStarting(false)
    }
  }

  function onScopeCancel() {
    setScopePicker((prev) => ({ ...prev, open: false }))
  }

  async function onStop() {
    setNotice('')
    await finishStopRef.current()
  }

  async function onContextChange(name) {
    if (!name) return
    const current = cluster.selectedContext || cluster.currentContext || ''
    // One cluster per window: while investigating, open another context in a clone.
    if ((running || starting) && name !== current) {
      try {
        await OpenNewWindow({
          context: name,
          namespace: '',
          kubeconfig: cluster.kubeconfigPath || '',
        })
      } catch (err) {
        setError(String(err))
      }
      return
    }
    try {
      await setContext(name)
    } catch (err) {
      setError(String(err))
    }
  }

  async function onNamespaceChange(name) {
    try {
      await setNamespace(name)
    } catch (err) {
      setError(String(err))
    }
  }

  async function onNewWindow() {
    try {
      await OpenNewWindow({
        context: cluster.selectedContext || cluster.currentContext || '',
        namespace: cluster.selectedNamespace || '',
        kubeconfig: cluster.kubeconfigPath || '',
      })
    } catch (err) {
      setError(String(err))
    }
  }

  const [explorerFilters, setExplorerFilters] = useState({})
  const [graphRelations, setGraphRelations] = useState(() => defaultRelations())

  const timeWindowLabel = prefs?.windowMin ? `Last ${prefs.windowMin}m` : 'Last 15m'
  const live = running && prefs?.autoRefresh !== false

  const inspectRowForToolbar = useMemo(() => {
    const rows = deriveMatchRows(view, getMatchedObjects(view))
    if (!inspectKey) return null
    return rows.find((r) => r.key === inspectKey) || null
  }, [view, inspectKey])

  function wrapShell(payload) {
    const shell = (
      <AppShell
        tab={tab}
        onTabChange={handleNavigate}
        onOpenSettings={() => navigateTo({ tab: 'settings', settingsSection: 'general' })}
        onOpenHelp={() => navigateTo({ tab: 'settings', settingsSection: 'help' })}
        topBarProps={{
          cluster,
          syncing,
          onSync: syncNow,
          onContextChange,
          onNamespaceChange,
          query,
          onQueryChange: setQuery,
          onQueryClear: () => setQuery(''),
          running,
          starting,
          activeQuery,
          onStart,
          onStop,
          prefs,
          onPrefsChange: setPreferences,
          live,
          onNavBack: handleNavBack,
          onNavForward: handleNavForward,
          onNavHome: handleNavHome,
          canNavBack: navigation.canGoBack,
          canNavForward: navigation.canGoForward,
        }}
        showExplorer={running || starting}
        showInspector={payload.showInspector}
        inspector={payload.inspector}
        view={view}
        cluster={cluster}
        activeQuery={activeQuery}
        timeWindowLabel={timeWindowLabel}
        live={live}
        prefs={prefs}
        onPrefsChange={setPreferences}
        inspectRow={inspectRowForToolbar}
        explorerFilters={explorerFilters}
        onExplorerFiltersChange={setExplorerFilters}
        graphRelations={graphRelations}
        onGraphRelationsChange={setGraphRelations}
      >
        {payload.workspace}
      </AppShell>
    )

    if (payload.resourcesWrap) {
      const rw = payload.resourcesWrap
      return (
        <ScopeBrowseProvider
          view={rw.view}
          catalog={rw.catalog}
          cluster={rw.cluster}
          rows={rw.rows}
          chain={rw.focusPinned}
        >
          <ResourcesWorkbenchRoot {...rw}>
            {shell}
          </ResourcesWorkbenchRoot>
        </ScopeBrowseProvider>
      )
    }

    return shell
  }

  const showStream = tab !== 'evidence' && tab !== 'graph' && tab !== 'patterns' && tab !== 'settings' && tab !== 'terminal'
  const maximized = stream.panelState === 'maximized'

  const filterLogsFromPatterns = useCallback(async (term) => {
    const q = String(term || '').trim()
    stream.setSearch(q)
    stream.setMode('logs')
    if (q) {
      stream.setFollow(false)
      stream.openPanel(PANEL_NORMAL)
      navigateTo('incident')
      if (running && !logTailEngaged) {
        const podNames = (view.state?.snapshot?.pods || [])
          .map((p) => p?.name)
          .filter(Boolean)
        if (!podNames.length) return
        try {
          await StartLogTail({ podNames })
          const next = await GetView()
          applyView(next)
        } catch (err) {
          setGatherError(String(err))
        }
      }
    }
  }, [stream, running, logTailEngaged, view.state?.snapshot?.pods, applyView])

  return (
    <>
      {notice && <div className="banner-info banner-float">{notice}</div>}
      {error && <div className="banner-error banner-float">{error}</div>}

      <div className={`app-root ${maximized ? 'stream-maximized' : ''}`}>
      <MainContent
        tab={tab}
        view={view}
        running={running}
        starting={starting}
        scopePickerOpen={scopePicker.open}
        activeQuery={activeQuery}
        cluster={cluster}
        clusterStatus={clusterStatus}
        syncing={syncing}
        themeId={themeId}
        onThemeChange={setTheme}
        onOpenSettings={() => navigateTo({ tab: 'settings', settingsSection: 'general' })}
        onOpenEvidence={() => navigateTo('evidence')}
        prefs={prefs}
        onPrefsChange={setPreferences}
        onClusterRefresh={syncNow}
        onTerminalShellChange={handleTerminalShellChange}
        onOpenTerminalShellPicker={handleOpenTerminalShellPicker}
        terminalShellRestartToken={terminalShellRestartToken}
        terminalMounted={terminalMounted}
        onTerminalMounted={() => setTerminalMounted(true)}
        settingsSection={settingsSection}
        onSettingsSectionChange={setSettingsSection}
        focusKey={focusKey}
        focusPinned={focusPinned}
        drillDown={drillDown}
        onFocusChange={handleFocusChange}
        onClearFocus={handleClearFocus}
        onFilterLogsFromPatterns={filterLogsFromPatterns}
        inspectKey={inspectKey}
        onInspectKeyChange={setInspectKey}
        onNavigate={handleNavigate}
        nodesFocus={nodesFocus}
        explorerFilters={explorerFilters}
        onExplorerFiltersChange={setExplorerFilters}
        graphRelations={graphRelations}
        onGraphRelationsChange={setGraphRelations}
        renderShell={wrapShell}
      />

      <div className="stream-dock">
        {showStream && (
          <LiveStreamPanel
            evidence={view.evidence}
            snapshotPods={view.state?.snapshot?.pods}
            query={running ? activeQuery : query}
            running={running}
            logTailEngaged={logTailEngaged}
            logTailPaused={logTailPaused}
            tailPods={view.state?.logTailPods}
            dropped={view.dropped}
            updatedAt={view.updatedAt}
            lastEventAt={view.state?.counters?.lastEventAt}
            panelState={stream.panelState}
            height={stream.height}
            mode={stream.mode}
            search={stream.search}
            selectedPods={stream.selectedPods}
            follow={stream.follow}
            streamFontSize={prefs.streamFontSize}
            streamDense={prefs.streamDense}
            streamWrapLines={prefs.streamWrapLines}
            onModeChange={stream.setMode}
            onSearchChange={stream.setSearch}
            onTogglePod={stream.togglePod}
            onSelectMatched={stream.selectMatchedPods}
            onSelectAllPods={stream.selectAllPods}
            onFollowChange={stream.setFollow}
            onToggleLogTailPause={handleToggleLogTailPause}
            onMinimize={stream.minimize}
            onMaximize={stream.maximize}
            onRestore={stream.restore}
            onClose={stream.close}
            onOpen={stream.openPanel}
            onResizeStart={stream.startResize}
            onStartGather={handleStartGather}
            onStopGather={handleStopGather}
            onClearLogs={handleClearLogs}
            gatherBusy={gatherBusy}
            gatherError={gatherError}
          />
        )}
      </div>
      </div>

      <TerminalShellModal
        open={terminalShellPickerOpen}
        initialChoice={prefs.terminalShell || 'system'}
        confirmLabel={pendingNavAfterShell?.tab === 'terminal' ? 'Open terminal' : 'Save'}
        onConfirm={handleTerminalShellConfirm}
        onCancel={() => setTerminalShellPickerOpen(false)}
      />

      <ScopePickerModal
        open={scopePicker.open}
        query={scopePicker.query}
        namespace={cluster.selectedNamespace}
        contextLabel={cluster.selectedContext}
        matches={scopePicker.matches}
        mode={scopePicker.mode}
        onConfirm={onScopeConfirm}
        onCancel={onScopeCancel}
      />
    </>
  )
}
