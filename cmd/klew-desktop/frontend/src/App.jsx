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
  SetKubectlOptions,
} from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime/runtime'
import { emptyView } from './lib/constants'
import { TerminalLaunchProvider } from './context/TerminalLaunchContext.jsx'
import { useCluster } from './hooks/useCluster'
import { useClusterStatus } from './hooks/useClusterStatus'
import { useClusterConnection } from './hooks/useClusterConnection'
import { useResourceCatalog } from './hooks/useResourceCatalog'
import {
  allBrowseScope,
  browseScopeLabel,
  discoverOptionsFromScope,
  investigationLockScope,
  investigationNamespace,
  normalizeInvestigationScope,
  normalizeBrowseScope,
  RESOURCES_BROWSE_LENS,
  scopeSupportsInvestigate,
  singleBrowseScope,
} from './lib/browseScope.js'
import { HOME_NAV, normalizeNavEntry, useNavigationHistory } from './hooks/useNavigationHistory'
import { useStreamPanel, PANEL_NORMAL, PANEL_CLOSED } from './hooks/useStreamPanel'
import { useTerminalPanel } from './hooks/useTerminalPanel'
import { useConsoleDock } from './hooks/useConsoleDock'
import { useTheme } from './hooks/useTheme'
import { usePreferences } from './hooks/usePreferences'
import { startOptionsFromPreferences } from './lib/preferences'
import { applyUiFont, scheduleFontCacheRefresh } from './lib/fonts'
import { registerSettingsRefreshHandler, scheduleSettingsCacheRefresh } from './lib/settingsCache'
import { applyTheme } from './lib/themes'
import { resolveTerminalShellPref } from './lib/terminalShell'
import { buildKubectlLogsCommand, podLogsTabTitle } from './lib/podLogsTerminal'
import { useIdleAutoStop, formatIdleDuration } from './hooks/useIdleAutoStop'
import { AppShell } from './components/shell/AppShell.jsx'
import { ResourcesWorkbenchRoot } from './views/ResourcesWorkbenchView'
import { ScopeBrowseProvider } from './context/ScopeBrowseContext.jsx'
import { defaultRelations } from './components/shell/explorers/ExplorerPanels.jsx'
import { ConsoleDock } from './components/ConsoleDock'
import { TerminalShellModal } from './components/TerminalShellModal'
import { ScopePickerModal } from './components/incident/ScopePickerModal'
import { MainContent } from './views/MainContent'
import { ClusterConnectionBanner } from './components/shell/ClusterConnectionBanner'
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
  const [tab, setTab] = useState('resources')
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

  const {
    cluster,
    syncing,
    connecting,
    connectingTarget,
    syncNow,
    setContext,
    setNamespace,
  } = useCluster()
  const [investigationScope, setInvestigationScope] = useState(() => singleBrowseScope(''))
  const [browseScope, setBrowseScope] = useState(() => allBrowseScope())
  const savedBrowseScopeRef = useRef(null)
  const [investigationNs, setInvestigationNs] = useState('')
  const [lockedResourcesScope, setLockedResourcesScope] = useState(null)
  const [savedBrowseScopeLabel, setSavedBrowseScopeLabel] = useState('')
  const [resourcesBrowseLens, setResourcesBrowseLens] = useState(RESOURCES_BROWSE_LENS.MATCHES)
  const prevContextRef = useRef('')

  useEffect(() => {
    const ctx = cluster.selectedContext || cluster.currentContext || ''
    if (prevContextRef.current && prevContextRef.current !== ctx) {
      const ns = cluster.selectedNamespace || ''
      setInvestigationScope(singleBrowseScope(ns))
      setBrowseScope(allBrowseScope())
    }
    prevContextRef.current = ctx
  }, [cluster.selectedContext, cluster.currentContext, cluster.selectedNamespace])

  useEffect(() => {
    const ns = cluster.selectedNamespace || ''
    if (!ns) return
    setInvestigationScope((prev) => {
      const current = normalizeBrowseScope(prev)
      if (current.namespace) return prev
      return singleBrowseScope(ns)
    })
    setBrowseScope((prev) => {
      const current = normalizeBrowseScope(prev)
      if (current.mode === 'all' || current.mode === 'multi') return prev
      if (current.namespace) return prev
      return singleBrowseScope(ns)
    })
  }, [cluster.selectedContext, cluster.selectedNamespace])

  const normalizedInvestigationScope = useMemo(
    () => normalizeInvestigationScope(investigationScope, { fallbackNamespace: cluster.selectedNamespace }),
    [investigationScope, cluster.selectedNamespace],
  )
  const normalizedBrowseScope = useMemo(() => normalizeBrowseScope(browseScope), [browseScope])

  const investigationScopeLocked = running || starting
  const resourcesInvestigationActive = Boolean(lockedResourcesScope)
  const resourcesNamespaceLocked = resourcesInvestigationActive
    && resourcesBrowseLens === RESOURCES_BROWSE_LENS.MATCHES

  const displayInvestigationScope = useMemo(() => {
    if (investigationScopeLocked && investigationNs) return singleBrowseScope(investigationNs)
    return normalizedInvestigationScope
  }, [investigationScopeLocked, investigationNs, normalizedInvestigationScope])

  const effectiveBrowseScope = useMemo(() => {
    if (resourcesNamespaceLocked && lockedResourcesScope) {
      return normalizeBrowseScope(lockedResourcesScope)
    }
    return normalizedBrowseScope
  }, [resourcesNamespaceLocked, lockedResourcesScope, normalizedBrowseScope])

  const lockResourcesForInvestigation = useCallback((scope, sessionNs = '') => {
    setResourcesBrowseLens(RESOURCES_BROWSE_LENS.MATCHES)
    setLockedResourcesScope(investigationLockScope(scope, sessionNs))
    if (!savedBrowseScopeRef.current) {
      savedBrowseScopeRef.current = normalizedBrowseScope
      setSavedBrowseScopeLabel(browseScopeLabel(normalizedBrowseScope, { namespaces: cluster.namespaces }))
    }
  }, [normalizedBrowseScope, cluster.namespaces])

  const unlockResourcesFromInvestigation = useCallback(() => {
    const saved = savedBrowseScopeRef.current
    const lens = resourcesBrowseLens
    savedBrowseScopeRef.current = null
    setLockedResourcesScope(null)
    setInvestigationNs('')
    setSavedBrowseScopeLabel('')
    setResourcesBrowseLens(RESOURCES_BROWSE_LENS.MATCHES)
    if (saved && lens === RESOURCES_BROWSE_LENS.MATCHES) setBrowseScope(saved)
  }, [resourcesBrowseLens])

  const scopedCluster = useMemo(
    () => ({
      ...cluster,
      scope: effectiveBrowseScope,
      browseScope: effectiveBrowseScope,
    }),
    [cluster, effectiveBrowseScope],
  )
  const resourceCatalog = useResourceCatalog(cluster, effectiveBrowseScope)
  const { clusterStatus, statusLoading, refreshClusterStatus } = useClusterStatus(cluster)
  const { connection, reconnect, dismiss, reconnectBusy } = useClusterConnection({
    cluster,
    syncing,
    connecting,
    connectingTarget,
    clusterStatus,
    statusLoading,
    syncNow,
    refreshClusterStatus,
  })
  const stream = useStreamPanel()
  const terminal = useTerminalPanel()
  const consoleDock = useConsoleDock({ stream, terminal })
  const [terminalShellPickerOpen, setTerminalShellPickerOpen] = useState(false)
  const [pendingShellPickerAction, setPendingShellPickerAction] = useState(null)
  const [terminalShellRestartToken, setTerminalShellRestartToken] = useState(0)
  const [terminalLaunchRequest, setTerminalLaunchRequest] = useState(null)
  const pendingPodLogsLaunchRef = useRef(null)
  const { themeId, setTheme } = useTheme()
  const { prefs, setPreferences, reloadPreferences } = usePreferences()
  const activeQueryRef = useRef('')

  useEffect(() => {
    applyUiFont(prefs.uiFont).catch(() => {})
  }, [prefs.uiFont])

  useEffect(() => {
    return scheduleFontCacheRefresh(() => prefs.uiFont)
  }, [prefs.uiFont])

  useEffect(() => {
    return registerSettingsRefreshHandler((snapshot) => {
      reloadPreferences()
      if (snapshot?.themeId) {
        applyTheme(snapshot.themeId)
        setTheme(snapshot.themeId)
      }
      if (snapshot?.prefs?.uiFont) {
        applyUiFont(snapshot.prefs.uiFont).catch(() => {})
      }
    })
  }, [reloadPreferences, setTheme])

  useEffect(() => scheduleSettingsCacheRefresh(), [])

  useEffect(() => {
    if (typeof SetKubectlOptions !== 'function') return
    SetKubectlOptions(
      prefs.useBundledKubectl !== false,
      prefs.useBundledKubectl ? '' : (prefs.kubectlPath || ''),
      prefs.matchClusterKubectl !== false,
    ).catch(() => {})
  }, [prefs.useBundledKubectl, prefs.kubectlPath, prefs.matchClusterKubectl])

  useEffect(() => {
    activeQueryRef.current = activeQuery
  }, [activeQuery])

  useEffect(() => {
    if (!error) return undefined
    const t = window.setTimeout(() => setError(''), 5000)
    return () => window.clearTimeout(t)
  }, [error])

  useEffect(() => {
    if (!notice) return undefined
    const t = window.setTimeout(() => setNotice(''), 5000)
    return () => window.clearTimeout(t)
  }, [notice])

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
    if (terminal.open) {
      setTerminalShellRestartToken(Date.now())
    }
    if (pendingShellPickerAction === 'openTerminal') {
      consoleDock.openTerminal()
      setPendingShellPickerAction(null)
    }
    if (pendingShellPickerAction === 'openPodLogs') {
      const target = pendingPodLogsLaunchRef.current
      pendingPodLogsLaunchRef.current = null
      if (target) {
        setTerminalLaunchRequest({
          id: Date.now(),
          title: podLogsTabTitle(target.podName, target.container),
          namespace: target.namespace || cluster.selectedNamespace || '',
          initialInput: buildKubectlLogsCommand(target),
        })
      }
      consoleDock.openTerminal()
      setPendingShellPickerAction(null)
    }
  }, [setPreferences, terminal, pendingShellPickerAction, consoleDock, cluster.selectedNamespace])

  const openShellPicker = useCallback((action = null) => {
    setPendingShellPickerAction(action)
    setTerminalShellPickerOpen(true)
  }, [])

  const toggleTerminal = useCallback(() => {
    if (!prefs.terminalShellPrompted) {
      openShellPicker('openTerminal')
      return
    }
    consoleDock.openTerminal()
  }, [prefs.terminalShellPrompted, openShellPicker, consoleDock])

  const handleOpenTerminalShellPicker = useCallback(() => {
    openShellPicker(null)
  }, [openShellPicker])

  const handleTerminalShellConfirm = useCallback((choice) => {
    applyTerminalShell(choice)
  }, [applyTerminalShell])

  const handleTerminalShellChange = useCallback(() => {
    if (terminal.open) {
      setTerminalShellRestartToken(Date.now())
    }
  }, [terminal.open])

  const handleOpenPodLogs = useCallback((target) => {
    if (!target?.podName) return
    if (!prefs.terminalShellPrompted) {
      pendingPodLogsLaunchRef.current = target
      openShellPicker('openPodLogs')
      return
    }
    setTerminalLaunchRequest({
      id: Date.now(),
      title: podLogsTabTitle(target.podName, target.container),
      namespace: target.namespace || cluster.selectedNamespace || '',
      initialInput: buildKubectlLogsCommand(target),
    })
    consoleDock.openTerminal()
  }, [prefs.terminalShellPrompted, openShellPicker, consoleDock, cluster.selectedNamespace])

  const launchTerminalCommand = useCallback((opts) => {
    if (!opts?.initialInput) return
    setTerminalLaunchRequest({
      id: Date.now(),
      title: opts.title || 'Terminal',
      namespace: opts.namespace || cluster.selectedNamespace || '',
      initialInput: opts.initialInput,
    })
    consoleDock.openTerminal()
  }, [consoleDock, cluster.selectedNamespace])

  const handleNavigate = useCallback((target) => {
    const entry = normalizeNavEntry(
      typeof target === 'object' ? target : { tab: target },
      { tab, nodesFocus, settingsSection },
    )
    if (entry.tab === 'terminal') {
      toggleTerminal()
      return
    }
    if (entry.tab === 'live-logs') {
      consoleDock.openStream()
      return
    }
    navigateTo(entry)
  }, [tab, nodesFocus, settingsSection, navigateTo, toggleTerminal, consoleDock])

  useEffect(() => {
    const offTerminal = EventsOn('menu:terminal', () => {
      toggleTerminal()
    })
    const offLiveLogs = EventsOn('menu:live-logs', () => {
      consoleDock.openStream()
    })
    const offConsoleSplit = EventsOn('menu:console-split', () => {
      consoleDock.openSplit()
    })
    return () => {
      offTerminal?.()
      offLiveLogs?.()
      offConsoleSplit?.()
    }
  }, [toggleTerminal, consoleDock])

  const logTailEngaged = (view.state?.logTailPods?.length ?? 0) > 0
  const logTailPaused = Boolean(view.state?.logTailPaused)

  const finishStopRef = useRef(async () => {})

  finishStopRef.current = async ({ idleMessage } = {}) => {
    await StopInvestigation()
    unlockResourcesFromInvestigation()
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
  }, [])

  const handleClearFocus = useCallback(() => {
    setFocusPinned(false)
    setFocusKey(null)
  }, [])

  const runInvestigation = useCallback(async (q, matches = []) => {
    const trimmed = normalizeInvestigationQuery(q)
    const ns = investigationNamespace(normalizedInvestigationScope, matches)
    if (!ns) {
      throw new Error('Could not determine namespace for investigation')
    }
    setNotice('')
    setView(emptyView())
    const opts = startOptionsFromPreferences(prefs, {
      query: trimmed,
      namespace: ns,
      context: cluster.selectedContext,
      kubeconfig: cluster.kubeconfigPath,
    })
    // StartInvestigation stops any prior session — no separate Stop click needed.
    lockResourcesForInvestigation(normalizedInvestigationScope, ns)
    setInvestigationNs(ns)
    await StartInvestigation(opts)
    setRunning(true)
    setGatherError('')
    setActiveQuery(trimmed)
    setFocusKey(null)
    setFocusPinned(false)
    stream.resetFilters()
    if (prefs.openStreamOnInvestigate) {
      consoleDock.openStream()
    }
    const next = await GetView()
    applyView(next)
    bumpActivity()
  }, [cluster, normalizedInvestigationScope, lockResourcesForInvestigation, stream, consoleDock, prefs, applyView, bumpActivity])

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
      consoleDock.openStream()
    } catch (err) {
      setGatherError(String(err))
    } finally {
      setGatherBusy(false)
    }
  }, [applyView, stream, consoleDock, prefs.followLogsByDefault])

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
    if (!scopeSupportsInvestigate(normalizedInvestigationScope)) return

    const q = normalizeInvestigationQuery(query)
    if (q !== query) {
      setQuery(q)
    }

    const scopeLabel = browseScopeLabel(normalizedInvestigationScope, { namespaces: cluster.namespaces })
    const discoverBase = discoverOptionsFromScope(normalizedInvestigationScope, {
      context: cluster.selectedContext,
      kubeconfig: cluster.kubeconfigPath,
    })

    setError('')
    navigateTo('resources')
    beginInvestigationTransition(q)
    lockResourcesForInvestigation(
      normalizedInvestigationScope,
      normalizedInvestigationScope.mode === 'single' ? normalizedInvestigationScope.namespace : '',
    )
    setStarting(true)

    let matches = []
    try {
      matches = await DiscoverMatches({
        query: q,
        ...discoverBase,
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
        setError(`No resources matched "${q}" in ${scopeLabel}.`)
        setActiveQuery('')
        setView(emptyView())
        savedBrowseScopeRef.current = null
        setLockedResourcesScope(null)
        setInvestigationNs('')
        setSavedBrowseScopeLabel('')
        return
      }
    } catch (err) {
      setError(String(err))
      setRunning(false)
      setActiveQuery('')
      setView(emptyView())
      savedBrowseScopeRef.current = null
      setLockedResourcesScope(null)
      setInvestigationNs('')
      setSavedBrowseScopeLabel('')
      return
    } finally {
      setStarting(false)
    }

    setRunning(true)
    try {
      await runInvestigation(q, matches)
    } catch (err) {
      setError(String(err))
      setRunning(false)
      setActiveQuery('')
      setView(emptyView())
      unlockResourcesFromInvestigation()
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
    navigateTo('resources')
    lockResourcesForInvestigation(
      normalizedInvestigationScope,
      normalizedInvestigationScope.mode === 'single' ? normalizedInvestigationScope.namespace : '',
    )
    setRunning(true)
    try {
      await runInvestigation(effectiveQuery, matches)
    } catch (err) {
      setError(String(err))
      setRunning(false)
      setActiveQuery('')
      setView(emptyView())
      unlockResourcesFromInvestigation()
    }
  }

  function onScopeCancel() {
    setScopePicker((prev) => ({ ...prev, open: false }))
    if (!running) {
      savedBrowseScopeRef.current = null
      setLockedResourcesScope(null)
      setInvestigationNs('')
      setSavedBrowseScopeLabel('')
    }
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

  function onInvestigationScopeChange(nextScope) {
    if (investigationScopeLocked) return
    const next = normalizeInvestigationScope(nextScope, { fallbackNamespace: cluster.selectedNamespace })
    if (!next.namespace) return
    setInvestigationScope(next)
    setNamespace(next.namespace).catch((err) => setError(String(err)))
  }

  function onBrowseScopeChange(nextScope) {
    if (resourcesNamespaceLocked) return
    setBrowseScope(normalizeBrowseScope(nextScope))
  }

  const onResourcesBrowseLensChange = useCallback((lens) => {
    setResourcesBrowseLens(lens)
  }, [])

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
        terminalOpen={consoleDock.expanded && (consoleDock.activeView === 'terminal' || consoleDock.activeView === 'split')}
        liveLogsOpen={consoleDock.expanded && (consoleDock.activeView === 'stream' || consoleDock.activeView === 'split')}
        streamLive={{
          running,
          logTailEngaged,
          logTailPaused,
        }}
        onOpenSettings={() => navigateTo({ tab: 'settings', settingsSection: 'general' })}
        onOpenHelp={() => navigateTo({ tab: 'settings', settingsSection: 'help' })}
        topBarProps={{
          cluster,
          scope: displayInvestigationScope,
          scopeLocked: investigationScopeLocked,
          savedScopeLabel: '',
          investigationNs,
          onScopeChange: onInvestigationScopeChange,
          scopeVariant: 'investigate',
          syncing,
          onSync: syncNow,
          onContextChange,
          query,
          onQueryChange: setQuery,
          onQueryClear: () => setQuery(''),
          running,
          starting,
          activeQuery,
          onStart,
          onStop,
          onNewWindow,
          prefs,
          onPrefsChange: setPreferences,
          live,
          onNavBack: handleNavBack,
          onNavForward: handleNavForward,
          onNavHome: handleNavHome,
          canNavBack: navigation.canGoBack,
          canNavForward: navigation.canGoForward,
          connection,
          onReconnect: reconnect,
          reconnectBusy,
          connecting,
          connectingTarget,
        }}
        showExplorer={running || starting || scopePicker.open || tab === 'resources' || ['incident', 'patterns', 'failures', 'evidence', 'graph'].includes(tab)}
        investigationActive={running || starting}
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
        onOpenPodLogs={handleOpenPodLogs}
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
          browseLens={rw.resourcesBrowseLens}
        >
          <ResourcesWorkbenchRoot {...rw}>
            {shell}
          </ResourcesWorkbenchRoot>
        </ScopeBrowseProvider>
      )
    }

    return shell
  }

  const showConsoleDock = terminal.open || stream.panelState !== PANEL_CLOSED || consoleDock.expanded
  const maximized = consoleDock.maximized

  const filterLogsFromPatterns = useCallback(async (term) => {
    const q = String(term || '').trim()
    stream.setSearch(q)
    stream.setMode('logs')
    if (q) {
      stream.setFollow(false)
      consoleDock.openStream()
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
  }, [stream, consoleDock, running, logTailEngaged, view.state?.snapshot?.pods, applyView, navigateTo])

  return (
    <>
      {notice && (
        <div className="banner-info banner-float" role="status" onClick={() => setNotice('')}>
          {notice}
        </div>
      )}
      {error && (
        <div className="banner-error banner-float" role="alert" onClick={() => setError('')}>
          {error}
        </div>
      )}

      <TerminalLaunchProvider launch={launchTerminalCommand}>
      <div className={`app-root ${maximized ? 'stream-maximized' : ''}`}>
      <ClusterConnectionBanner
        connection={connection}
        onReconnect={reconnect}
        onDismiss={dismiss}
        onOpenSettings={() => navigateTo({ tab: 'settings', settingsSection: 'kubernetes' })}
        reconnectBusy={reconnectBusy}
      />
      <MainContent
        tab={tab}
        view={view}
        running={running}
        starting={starting}
        scopePickerOpen={scopePicker.open}
        activeQuery={activeQuery}
        cluster={scopedCluster}
        clusterStatus={clusterStatus}
        resourceCatalog={resourceCatalog}
        browseScope={effectiveBrowseScope}
        onBrowseScopeChange={onBrowseScopeChange}
        browseScopeLocked={resourcesNamespaceLocked}
        savedBrowseScopeLabel={savedBrowseScopeLabel}
        investigationNs={investigationNs}
        resourcesBrowseLens={resourcesBrowseLens}
        onResourcesBrowseLensChange={onResourcesBrowseLensChange}
        investigationSession={{
          active: resourcesInvestigationActive,
          namespaceLabel: lockedResourcesScope
            ? browseScopeLabel(lockedResourcesScope, { namespaces: cluster.namespaces })
            : '',
          query: activeQuery || ((starting || scopePicker.open) ? query : ''),
          starting,
        }}
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
        <ConsoleDock
          visible={showConsoleDock}
          activeView={consoleDock.activeView}
          expanded={consoleDock.expanded}
          height={consoleDock.height}
          maximized={consoleDock.maximized}
          onSelectView={consoleDock.selectView}
          onMinimize={consoleDock.minimize}
          onMaximize={consoleDock.maximize}
          onRestore={consoleDock.restore}
          onClose={consoleDock.closeAll}
          onResizeStart={consoleDock.startResize}
          streamLive={{
            running,
            logTailEngaged,
            logTailPaused,
          }}
          streamProps={{
            evidence: view.evidence,
            snapshotPods: view.state?.snapshot?.pods,
            query: running ? activeQuery : query,
            running,
            logTailEngaged,
            logTailPaused,
            tailPods: view.state?.logTailPods,
            dropped: view.dropped,
            updatedAt: view.updatedAt,
            lastEventAt: view.state?.counters?.lastEventAt,
            mode: stream.mode,
            search: stream.search,
            selectedPods: stream.selectedPods,
            follow: stream.follow,
            streamFontSize: prefs.streamFontSize,
            streamDense: prefs.streamDense,
            streamWrapLines: prefs.streamWrapLines,
            onModeChange: stream.setMode,
            onSearchChange: stream.setSearch,
            onTogglePod: stream.togglePod,
            onSelectMatched: stream.selectMatchedPods,
            onSelectAllPods: stream.selectAllPods,
            onFollowChange: stream.setFollow,
            onToggleLogTailPause: handleToggleLogTailPause,
            onStartGather: handleStartGather,
            onStopGather: handleStopGather,
            onClearLogs: handleClearLogs,
            gatherBusy,
            gatherError,
          }}
          terminalProps={{
            cluster,
            shellPref: prefs.terminalShell,
            appearance: prefs.terminalAppearance,
            onChangeShell: handleOpenTerminalShellPicker,
            shellRestartToken: terminalShellRestartToken,
            launchRequest: terminalLaunchRequest,
            onLaunchHandled: () => setTerminalLaunchRequest(null),
          }}
        />
      </div>
      </div>
      </TerminalLaunchProvider>

      <TerminalShellModal
        open={terminalShellPickerOpen}
        initialChoice={prefs.terminalShell || 'system'}
        confirmLabel={pendingShellPickerAction === 'openTerminal' ? 'Open terminal' : 'Save'}
        onConfirm={handleTerminalShellConfirm}
        onCancel={() => {
          setTerminalShellPickerOpen(false)
          setPendingShellPickerAction(null)
        }}
      />

      <ScopePickerModal
        open={scopePicker.open}
        query={scopePicker.query}
        namespace={browseScopeLabel(displayInvestigationScope, { namespaces: cluster.namespaces })}
        contextLabel={cluster.selectedContext}
        matches={scopePicker.matches}
        mode={scopePicker.mode}
        onConfirm={onScopeConfirm}
        onCancel={onScopeCancel}
      />
    </>
  )
}
