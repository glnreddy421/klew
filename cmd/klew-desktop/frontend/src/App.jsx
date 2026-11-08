import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  StartInvestigation,
  StopInvestigation,
  InvestigationTargets,
  InvestigableTypes,
  GetView,
  OpenNewWindow,
  SetAutoRefresh,
  SetPollEverySec,
  SetKubectlOptions,
  SetNetworkProxy,
  RefreshInvestigation,
} from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime/runtime'
import { emptyView } from './lib/constants'
import { TerminalLaunchProvider } from './context/TerminalLaunchContext.jsx'
import { InvestigableProvider } from './context/InvestigableContext.jsx'
import { investigableIndex } from './lib/investigable.js'
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
  singleBrowseScope,
} from './lib/browseScope.js'
import { HOME_NAV, normalizeNavEntry, useNavigationHistory } from './hooks/useNavigationHistory'
import { useStreamPanel } from './hooks/useStreamPanel'
import { useLogStreams } from './hooks/useLogStream'
import { logTabTitle, podTargets, streamTargetsFor } from './lib/logStream'
import { LogStreamProvider } from './context/LogStreamContext.jsx'
import { SettingsDialog } from './components/settings/SettingsDialog.jsx'
import { McpSetupDialog } from './components/McpSetupDialog.jsx'
import { useTerminalPanel } from './hooks/useTerminalPanel'
import { useTerminalTabs } from './hooks/useTerminalTabs'
import { useConsoleDock } from './hooks/useConsoleDock'
import { useDockTabs } from './hooks/useDockTabs'
import { useTheme } from './hooks/useTheme'
import { usePreferences } from './hooks/usePreferences'
import { useEventHistory } from './hooks/useEventHistory'
import { EventHistoryProvider } from './context/EventHistoryContext.jsx'
import { startOptionsFromPreferences } from './lib/preferences'
import { applyUiFont, scheduleFontCacheRefresh } from './lib/fonts'
import { applyTopbarScale } from './lib/topbarScale'
import { invalidateCatalogCache } from './lib/catalogCache.js'
import { hasActiveClusterContext } from './lib/clusterIdentity.js'
import { toggleFavoriteContext } from './lib/clusterContexts.js'
import { registerSettingsRefreshHandler, scheduleSettingsCacheRefresh } from './lib/settingsCache'
import { applyTheme } from './lib/themes'
import { resolveTerminalShellPref } from './lib/terminalShell'
import { useIdleAutoStop, formatIdleDuration } from './hooks/useIdleAutoStop'
import { AppShell } from './components/shell/AppShell.jsx'
import { ResourcesWorkbenchRoot } from './views/ResourcesWorkbenchView'
import { ScopeBrowseProvider } from './context/ScopeBrowseContext.jsx'
import { HeaderSlotProvider } from './context/HeaderSlotContext.jsx'
import { defaultRelations } from './components/shell/explorers/ExplorerPanels.jsx'
import { ConsoleDock } from './components/ConsoleDock'
import { TerminalShellModal } from './components/TerminalShellModal'
import { InvestigateBar, InvestigationTabs, MODE_INVESTIGATE, MODE_WATCH } from './components/shell/InvestigateBar.jsx'
import { MainContent } from './views/MainContent'
import { ClusterConnectionBanner } from './components/shell/ClusterConnectionBanner'
import {
  deriveMatchRows,
  getMatchedObjects,
  findRowByKey,
  buildInspectKey,
} from './lib/matches'
import { isEditableTarget } from './lib/keyboard'
import { normalizeInvestigationQuery } from './lib/investigationQuery'
import {
  queryForTargets,
  reconcileTargets,
} from './lib/investigateTarget.js'
import { RESOURCES_SURFACE_TABS } from './lib/constants'

export default function App() {
  const [view, setView] = useState(emptyView())
  const [tab, setTab] = useState(HOME_NAV.tab)
  // The terminal and logs dock is for working in a cluster; Home has none.
  const dockAllowed = tab !== HOME_NAV.tab
  const dockAllowedRef = useRef(dockAllowed)
  dockAllowedRef.current = dockAllowed
  const [nodesFocus, setNodesFocus] = useState('cluster')
  const [settingsSection, setSettingsSection] = useState('general')
  // Settings is a dialog over the current page, not a navigation entry.
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [mcpSetupOpen, setMcpSetupOpen] = useState(false)
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
    if (entry.tab === 'settings') {
      setSettingsSection(entry.settingsSection || 'general')
      setSettingsOpen(true)
      return
    }
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
  const [refreshingInvestigation, setRefreshingInvestigation] = useState(false)
  const [starting, setStarting] = useState(false)
  const [activeQuery, setActiveQuery] = useState('')
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  // Resources surface: Watch browses, Investigate runs investigations from the bar.
  const [investigateMode, setInvestigateMode] = useState(MODE_WATCH)
  // Investigation targets as "Kind/name" keys; empty is the whole namespace.
  const [invTargets, setInvTargets] = useState([])
  const [invIncludeLogs, setInvIncludeLogs] = useState(true)
  const [activeNoLogs, setActiveNoLogs] = useState(false)
  const [invMatches, setInvMatches] = useState({ namespace: '', items: [], unavailable: [], loading: false, error: '' })

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
  const { themeId, resolvedTheme, setTheme } = useTheme()
  const { prefs, setPreferences, reloadPreferences } = usePreferences()
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
      invalidateCatalogCache()
      const ns = cluster.selectedNamespace || ''
      setInvestigationScope(singleBrowseScope(ns))
      setBrowseScope(allBrowseScope())
    }
    prevContextRef.current = ctx
  }, [cluster.selectedContext, cluster.currentContext])

  useEffect(() => {
    const ns = cluster.selectedNamespace || ''
    if (!ns) return
    setInvestigationScope((prev) => {
      const current = normalizeBrowseScope(prev)
      if (current.namespace) return prev
      return singleBrowseScope(ns)
    })
  }, [cluster.selectedContext, cluster.selectedNamespace])

  const normalizedInvestigationScope = useMemo(
    () => normalizeInvestigationScope(investigationScope, { fallbackNamespace: cluster.selectedNamespace }),
    [investigationScope, cluster.selectedNamespace],
  )
  const normalizedBrowseScope = useMemo(() => normalizeBrowseScope(browseScope), [browseScope])

  const investigating = investigateMode === MODE_INVESTIGATE
  const invNamespace = normalizedInvestigationScope.namespace || ''
  const resourcesInvestigationActive = Boolean(lockedResourcesScope)
  const resourcesNamespaceLocked = resourcesInvestigationActive
    && resourcesBrowseLens === RESOURCES_BROWSE_LENS.MATCHES


  const effectiveBrowseScope = useMemo(() => {
    if (resourcesNamespaceLocked && lockedResourcesScope) {
      return normalizeBrowseScope(lockedResourcesScope)
    }
    // Investigate mode shows the namespace about to be investigated.
    if (investigating && !resourcesInvestigationActive && invNamespace) {
      return singleBrowseScope(invNamespace)
    }
    return normalizedBrowseScope
  }, [resourcesNamespaceLocked, lockedResourcesScope, investigating, resourcesInvestigationActive, invNamespace, normalizedBrowseScope])

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
  const [monitoringPaused, setMonitoringPaused] = useState(false)
  const clusterMonitoringEnabled = !monitoringPaused
  const hasActiveContext = hasActiveClusterContext(cluster)
  // One context per window — catalog/workloads load only after the user opens Resources.
  const resourcesBrowseEnabled = clusterMonitoringEnabled && hasActiveContext && tab === 'resources'
  const resourceCatalog = useResourceCatalog(cluster, effectiveBrowseScope, {
    enabled: resourcesBrowseEnabled,
  })
  const { clusterStatus, statusLoading, refreshClusterStatus } = useClusterStatus(cluster, {
    enabled: clusterMonitoringEnabled && hasActiveContext && tab !== 'home',
  })
  const {
    connection,
    reconnect,
    disconnect,
    dismiss,
    reconnectBusy,
  } = useClusterConnection({
    cluster,
    syncing,
    connecting,
    connectingTarget,
    clusterStatus,
    statusLoading,
    syncNow,
    refreshClusterStatus,
    autoMonitor: prefs.autoMonitorCluster !== false,
    monitoringPaused,
    onMonitoringPausedChange: setMonitoringPaused,
  })
  const kubeContextNames = useMemo(
    () => (cluster?.contexts || []).map((c) => c?.name).filter(Boolean),
    [cluster?.contexts],
  )
  const eventHistory = useEventHistory({
    favoriteContexts: prefs.favoriteContexts,
    monitoringPaused,
    contexts: kubeContextNames,
  })
  const stream = useStreamPanel()
  const terminal = useTerminalPanel()
  const consoleDock = useConsoleDock({ stream, terminal })
  const [terminalShellPickerOpen, setTerminalShellPickerOpen] = useState(false)
  const [terminalShellRestartToken, setTerminalShellRestartToken] = useState(0)
  const [terminalLaunchRequest, setTerminalLaunchRequest] = useState(null)
  const [logsFindFromPatterns, setLogsFindFromPatterns] = useState(null)
  const terminalTabs = useTerminalTabs(cluster, {
    open: terminal.open,
    persist: false,
    skipInitialSeed: Boolean(terminalLaunchRequest?.id),
  })
  const restoreLogStreamsRef = useRef(null)
  const logStreams = useLogStreams({
    maxLogRequests: prefs.maxLogRequests,
    onRestored: (statuses) => restoreLogStreamsRef.current?.(statuses),
  })
  const dockTabs = useDockTabs({
    terminalTabs,
    logStreams,
    onOpenTerminalPanel: terminal.openPanel,
    onExpandDock: consoleDock.openStream,
  })
  restoreLogStreamsRef.current = (statuses) => {
    dockTabs.restoreLogStreams(statuses)
    if (statuses?.length) {
      consoleDock.openStream()
    }
  }
  const clearLogTabsOnContextChangeRef = useRef(null)
  clearLogTabsOnContextChangeRef.current = () => {
    logStreams.stopAll()
    dockTabs.clearLogTabs()
  }
  useEffect(() => {
    const ctx = cluster.selectedContext || cluster.currentContext || ''
    if (prevContextRef.current && prevContextRef.current !== ctx) {
      clearLogTabsOnContextChangeRef.current?.()
    }
  }, [cluster.selectedContext, cluster.currentContext])
  const activeQueryRef = useRef('')

  useEffect(() => {
    applyUiFont(prefs.uiFont).catch(() => {})
  }, [prefs.uiFont])

  useEffect(() => {
    applyTopbarScale(prefs.topbarScale)
  }, [prefs.topbarScale])

  useEffect(() => {
    return scheduleFontCacheRefresh(() => prefs.uiFont)
  }, [prefs.uiFont])

  useEffect(() => {
    return registerSettingsRefreshHandler((snapshot) => {
      reloadPreferences()
      if (snapshot?.themeId) {
        applyTheme(snapshot.themeId)
      }
      if (snapshot?.prefs?.uiFont) {
        applyUiFont(snapshot.prefs.uiFont).catch(() => {})
      }
      if (snapshot?.prefs?.topbarScale != null) {
        applyTopbarScale(snapshot.prefs.topbarScale)
      }
    })
  }, [reloadPreferences])

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
    if (typeof SetNetworkProxy !== 'function') return
    SetNetworkProxy(
      prefs.httpProxy || '',
      prefs.httpsProxy || '',
      prefs.noProxy || '',
    ).catch(() => {})
  }, [prefs.httpProxy, prefs.httpsProxy, prefs.noProxy])

  const openProxySettings = useCallback(() => {
    navigateTo({ tab: 'settings', settingsSection: 'kubernetes' })
  }, [navigateTo])

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

  const refreshNow = useCallback(async () => {
    if (running) {
      if (refreshingInvestigation) return
      setRefreshingInvestigation(true)
      try {
        await RefreshInvestigation()
      } catch (err) {
        setError(String(err))
      } finally {
        setRefreshingInvestigation(false)
      }
      return
    }
    if (!syncing) await syncNow()
  }, [running, refreshingInvestigation, syncing, syncNow])

  useEffect(() => {
    function onKeyDown(e) {
      if (!(e.metaKey || e.ctrlKey) || e.key.toLowerCase() !== 'r') return
      if (isEditableTarget(e.target)) return
      if (syncing || refreshingInvestigation || starting) return
      e.preventDefault()
      refreshNow()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [refreshNow, syncing, refreshingInvestigation, starting])

  const enterInvestigateRef = useRef(() => {})
  useEffect(() => {
    function onKeyDown(e) {
      if (!(e.metaKey || e.ctrlKey) || e.key.toLowerCase() !== 'k') return
      e.preventDefault()
      enterInvestigateRef.current()
    }
    window.addEventListener('keydown', onKeyDown)
    const offSearch = EventsOn('menu:focus-search', () => enterInvestigateRef.current())
    return () => {
      window.removeEventListener('keydown', onKeyDown)
      offSearch?.()
    }
  }, [])

  useEffect(() => {
    const offSettings = EventsOn('menu:settings', () => {
      navigateTo({ tab: 'settings', settingsSection: 'general' })
    })
    const offHelp = EventsOn('menu:help', () => {
      navigateTo({ tab: 'settings', settingsSection: 'help' })
    })
    const offMcp = EventsOn('menu:mcp-setup', () => setMcpSetupOpen(true))
    return () => {
      offSettings?.()
      offHelp?.()
      offMcp?.()
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
  }, [setPreferences, terminal])

  const openShellPicker = useCallback(() => {
    setTerminalShellPickerOpen(true)
  }, [])

  // Like an IDE: open the user's default shell straight away; Shell… in the dock changes it.
  const openNewTerminalTab = useCallback(() => {
    if (!terminal.open) {
      terminal.openPanel()
      consoleDock.openTerminal()
      return
    }
    dockTabs.addTerminalTab()
  }, [terminal, consoleDock, dockTabs])

  const toggleTerminal = useCallback(() => {
    if (!dockAllowedRef.current) return
    if (!terminal.open) {
      terminal.openPanel()
      consoleDock.openTerminal()
      return
    }
    dockTabs.focusOrOpenTerminal()
  }, [terminal, consoleDock, dockTabs])

  const handleOpenTerminalShellPicker = useCallback(() => {
    openShellPicker()
  }, [openShellPicker])

  const handleTerminalShellConfirm = useCallback((choice) => {
    applyTerminalShell(choice)
  }, [applyTerminalShell])

  const handleTerminalShellChange = useCallback(() => {
    if (terminal.open) {
      setTerminalShellRestartToken(Date.now())
    }
  }, [terminal.open])

  const launchTerminalCommand = useCallback((opts) => {
    if (!opts?.initialInput) return
    terminal.openPanel()
    setTerminalLaunchRequest({
      id: Date.now(),
      title: opts.title || 'Terminal',
      namespace: opts.namespace || cluster.selectedNamespace || '',
      initialInput: opts.initialInput,
    })
    consoleDock.openTerminal()
  }, [terminal, consoleDock, cluster.selectedNamespace])

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
      if (!dockAllowedRef.current) return
      const sel = streamSelectionRef.current
      const picked = sel.table.length ? sel.table : sel.inspector
      if (picked.length) streamResourcesRef.current?.(picked)
      else consoleDock.openStream()
      return
    }
    navigateTo(entry)
  }, [tab, nodesFocus, settingsSection, navigateTo, toggleTerminal, consoleDock])

  useEffect(() => {
    const offTerminal = EventsOn('menu:terminal', () => {
      toggleTerminal()
    })
    // ⌘⇧L streams the current selection into a tab (or focuses its existing tab).
    const offLiveLogs = EventsOn('menu:live-logs', () => {
      if (!dockAllowedRef.current) return
      const sel = streamSelectionRef.current
      const picked = sel.table.length ? sel.table : sel.inspector
      if (picked.length) streamResourcesRef.current?.(picked)
      else consoleDock.openStream()
    })
    return () => {
      offTerminal?.()
      offLiveLogs?.()
    }
  }, [toggleTerminal, consoleDock])


  const finishStopRef = useRef(async () => {})

  finishStopRef.current = async ({ idleMessage } = {}) => {
    await StopInvestigation()
    unlockResourcesFromInvestigation()
    setRunning(false)
    setStarting(false)
    setActiveQuery('')
    setView(emptyView())
    stream.resetFilters()
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

  const runInvestigation = useCallback(async (q, matches = [], { noLogs = false, namespace = '' } = {}) => {
    const trimmed = normalizeInvestigationQuery(q)
    const ns = namespace || investigationNamespace(normalizedInvestigationScope, matches)
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
    opts.noLogs = Boolean(noLogs)
    // StartInvestigation stops any prior session — no separate Stop click needed.
    lockResourcesForInvestigation(normalizedInvestigationScope, ns)
    setInvestigationNs(ns)
    if (ns !== cluster.selectedNamespace) {
      await setNamespace(ns)
    }
    await StartInvestigation(opts)
    setActiveNoLogs(Boolean(noLogs))
    setRunning(true)
    setActiveQuery(trimmed)
    stream.resetFilters()
    const next = await GetView()
    applyView(next)
    bumpActivity()
  }, [cluster, normalizedInvestigationScope, lockResourcesForInvestigation, setNamespace, stream, prefs, applyView, bumpActivity])

  function beginInvestigationTransition(nextQuery) {
    setView(emptyView())
    setActiveQuery(normalizeInvestigationQuery(nextQuery))
    stream.resetFilters()
  }

  // What the engine can investigate in the chosen namespace, grouped by kind
  // (built-in and custom collectors), for the Kind and Object pickers.
  useEffect(() => {
    if (!investigating || !invNamespace || !cluster.selectedContext) return undefined
    let cancelled = false
    setInvMatches((prev) => ({ ...prev, namespace: invNamespace, loading: true, error: '' }))
    InvestigationTargets({
      query: '',
      ...discoverOptionsFromScope(singleBrowseScope(invNamespace), {
        context: cluster.selectedContext,
        kubeconfig: cluster.kubeconfigPath,
      }),
    })
      .then((list) => {
        if (!cancelled) {
          setInvMatches({
            namespace: invNamespace, items: list?.groups || [], unavailable: list?.unavailable || [], loading: false, error: '',
          })
        }
      })
      .catch((err) => {
        if (!cancelled) setInvMatches({ namespace: invNamespace, items: [], unavailable: [], loading: false, error: String(err) })
      })
    return () => { cancelled = true }
  }, [investigating, invNamespace, cluster.selectedContext, cluster.kubeconfigPath])

  const invItems = invMatches.namespace === invNamespace ? invMatches.items : []
  const invPendingQuery = queryForTargets(invTargets, invItems)

  useEffect(() => {
    // Only once the loaded targets belong to the chosen namespace.
    if (invMatches.loading || invMatches.namespace !== invNamespace || invMatches.error) return
    setInvTargets((t) => {
      const kept = reconcileTargets(t, invItems)
      return kept.length === t.length ? t : kept
    })
  }, [invItems, invMatches.loading, invMatches.namespace, invMatches.error, invNamespace])

  const invChanged = running && (
    invPendingQuery !== activeQuery
    || invNamespace !== investigationNs
    || activeNoLogs !== !invIncludeLogs
  )
  const canRunInvestigation = Boolean(invNamespace) && !starting && !invMatches.loading && (!running || invChanged)

  async function onRunFromBar() {
    if (!canRunInvestigation) return
    await startInvestigationOf(invNamespace, invPendingQuery)
  }

  // Runs an investigation of namespace, narrowed by query ('' = whole namespace).
  async function startInvestigationOf(namespace, q) {
    setError('')
    setQuery(q)
    beginInvestigationTransition(q)
    navigateTo('incident')
    lockResourcesForInvestigation(singleBrowseScope(namespace), namespace)
    setStarting(true)
    try {
      await runInvestigation(q, [], { noLogs: !invIncludeLogs, namespace })
    } catch (err) {
      setError(String(err))
      setRunning(false)
      setActiveQuery('')
      setView(emptyView())
      unlockResourcesFromInvestigation()
    } finally {
      setStarting(false)
    }
  }

  const onInvestigateModeChange = useCallback((mode) => {
    setInvestigateMode(mode)
    if (mode === MODE_WATCH) {
      navigateTo('resources')
      return
    }
    // Start from the namespace being browsed, when it is a single one.
    if (!running && !starting && normalizedBrowseScope.mode === 'single' && normalizedBrowseScope.namespace) {
      setInvestigationScope(singleBrowseScope(normalizedBrowseScope.namespace))
    }
    navigateTo(running || starting ? 'incident' : 'resources')
  }, [navigateTo, running, starting, normalizedBrowseScope])
  enterInvestigateRef.current = () => onInvestigateModeChange(MODE_INVESTIGATE)

  // Inspector "Investigate": target the selected resource exactly.
  // What Investigate can target, from the engine's collectors (built-in and
  // user extensions). Resources surfaces mark only these.
  const [investigableTypes, setInvestigableTypes] = useState(null)
  const onInvestigateRowRef = useRef(null)
  useEffect(() => {
    let cancelled = false
    Promise.resolve(InvestigableTypes?.())
      .then((types) => { if (!cancelled) setInvestigableTypes(Array.isArray(types) ? types : []) })
      .catch(() => { if (!cancelled) setInvestigableTypes([]) })
    return () => { cancelled = true }
  }, [])
  // What Resources has selected for ⌘⇧L, by surface.
  const streamSelectionRef = useRef({ table: [], inspector: [] })
  const [streamSelection, setStreamSelection] = useState({ table: [], inspector: [] })
  const streamResourcesRef = useRef(null)
  streamResourcesRef.current = async (resources) => {
    const { targets, skipped } = streamTargetsFor(resources)
    consoleDock.openStream()
    if (!targets.length) return
    const res = await dockTabs.openLogStream({ targets, tailLines: prefs.tailLines })
    if (res?.streamId) {
      stream.setFollow(Boolean(prefs.followLogsByDefault))
      if (skipped) setNotice(`Streaming ${targets.length} source${targets.length === 1 ? '' : 's'}; ${skipped} selected item${skipped === 1 ? '' : 's'} cannot stream logs.`)
    }
  }
  const streamPodLogsRef = useRef(null)
  streamPodLogsRef.current = async ({ kind, name, namespace, container } = {}) => {
    if (kind !== 'Pod' || !name || !namespace) return
    consoleDock.openStream()
    const res = await dockTabs.openLogStream({
      targets: podTargets(namespace, [name]),
      container: container || '',
      tailLines: prefs.tailLines,
    })
    if (res?.streamId) stream.setFollow(Boolean(prefs.followLogsByDefault))
  }
  const logStreamActions = useMemo(() => ({
    streamResources: (resources) => streamResourcesRef.current?.(resources),
    streamPodLogs: (spec) => streamPodLogsRef.current?.(spec),
    setSelection: (source, resources) => {
      const list = resources || []
      streamSelectionRef.current = { ...streamSelectionRef.current, [source]: list }
      setStreamSelection((prev) => {
        if (prev[source] === list || (prev[source]?.length === 0 && list.length === 0)) return prev
        return { ...prev, [source]: list }
      })
    },
  }), [])

  const pickedResourcesForStream = streamSelection.table.length ? streamSelection.table : streamSelection.inspector
  const pickedStreamTargets = useMemo(() => streamTargetsFor(pickedResourcesForStream).targets, [pickedResourcesForStream])
  const canStreamSelection = pickedStreamTargets.length > 0
  const selectionStreamLabel = useMemo(
    () => (pickedStreamTargets.length ? logTabTitle({ targets: pickedStreamTargets }) : ''),
    [pickedStreamTargets],
  )
  const handleStreamCurrentSelection = useCallback(() => {
    const sel = streamSelectionRef.current
    const picked = sel.table.length ? sel.table : sel.inspector
    if (picked.length) streamResourcesRef.current?.(picked)
  }, [])

  const investigable = useMemo(() => ({
    index: investigableIndex(investigableTypes),
    loaded: investigableTypes !== null,
    investigate: (row) => onInvestigateRowRef.current?.(row),
  }), [investigableTypes])

  async function onInvestigateRow(row) {
    const ns = String(row?.namespace || '').trim()
    if (!row?.kind || !row?.name || !ns || starting) return
    const key = `${row.kind}/${row.name}`
    // The breadcrumb and inspector follow the object being investigated, not
    // whatever was selected before.
    setInspectKey(buildInspectKey(row.kind, row.name, ns))
    setInvestigateMode(MODE_INVESTIGATE)
    setInvestigationScope(singleBrowseScope(ns))
    setInvTargets([key])
    await startInvestigationOf(ns, key)
  }

  onInvestigateRowRef.current = onInvestigateRow

  // Opening an investigation tab from anywhere (banners, links) shows Investigate mode.
  useEffect(() => {
    if (RESOURCES_SURFACE_TABS.has(tab) && tab !== 'resources') setInvestigateMode(MODE_INVESTIGATE)
  }, [tab])

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

  async function onConnectContext(name) {
    await onContextChange(name)
    navigateTo('resources')
  }

  const onSetDefaultContext = useCallback((name) => {
    if (!name) return
    const current = String(prefs.defaultContext || '').trim()
    setPreferences({ defaultContext: current === name ? '' : name })
  }, [prefs.defaultContext, setPreferences])

  const onToggleFavoriteContext = useCallback((name) => {
    if (!name) return
    setPreferences({
      favoriteContexts: toggleFavoriteContext(prefs.favoriteContexts, name),
    })
  }, [prefs.favoriteContexts, setPreferences])

  const onClearDefaultContext = useCallback(() => {
    setPreferences({ defaultContext: '' })
  }, [setPreferences])

  function onInvestigationScopeChange(nextScope) {
    // While running, a new namespace is staged for Re-run.
    if (starting) return
    const next = normalizeInvestigationScope(nextScope, { fallbackNamespace: cluster.selectedNamespace })
    if (!next.namespace) return
    if (next.namespace !== invNamespace) {
      setInvTargets([])
    }
    setInvestigationScope(next)
  }

  function onBrowseScopeChange(nextScope) {
    if (resourcesNamespaceLocked) return
    const next = normalizeBrowseScope(nextScope)
    setBrowseScope(next)
    const ns = next.mode === 'single' ? String(next.namespace || '').trim() : ''
    if (ns && ns !== (cluster.selectedNamespace || '')) {
      setNamespace(ns).catch(() => {})
    }
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
    return findRowByKey(rows, inspectKey)
  }, [view, inspectKey])

  // Top-bar terminal button: hides the dock (sessions keep running) or brings it back.
  const handleTogglePanel = useCallback(() => {
    if (!dockAllowedRef.current) return
    if (consoleDock.expanded) {
      consoleDock.collapse()
      return
    }
    if (dockTabs.tabs.length > 0) {
      consoleDock.openStream()
      return
    }
    toggleTerminal()
  }, [consoleDock, dockTabs.tabs.length, toggleTerminal])

  function wrapShell(payload) {
    const shell = (
      <AppShell
        tab={tab}
        onTabChange={handleNavigate}
        onOpenSettings={() => navigateTo({ tab: 'settings', settingsSection: 'general' })}
        onOpenHelp={() => navigateTo({ tab: 'settings', settingsSection: 'help' })}
        topBarProps={{
          cluster,
          syncing: syncing || refreshingInvestigation,
          onSync: refreshNow,
          onContextChange,
          running,
          starting,
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
          onDisconnect: disconnect,
          reconnectBusy,
          monitoringPaused,
          connecting,
          connectingTarget,
          defaultContext: prefs.defaultContext,
          favoriteContexts: prefs.favoriteContexts,
          onSetDefaultContext,
          onToggleFavoriteContext,
          panelOpen: consoleDock.expanded,
          panelRunning: terminalTabs.tabs.length > 0 || logStreams.hasActiveStream,
          onTogglePanel: dockAllowed ? handleTogglePanel : undefined,
        }}
        showExplorer={running || starting || RESOURCES_SURFACE_TABS.has(tab)}
        investigationActive={running || starting}
        investigationStarting={starting}
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
        onInvestigateRow={onInvestigateRow}
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

  const investigateChrome = (
    <div className="investigate-chrome">
      <InvestigateBar
        mode={investigateMode}
        onModeChange={onInvestigateModeChange}
        cluster={cluster}
        scope={normalizedInvestigationScope}
        onScopeChange={onInvestigationScopeChange}
        investigationNs={investigationNs}
        groups={invItems}
        unavailable={invMatches.namespace === invNamespace ? invMatches.unavailable : []}
        matchesLoading={invMatches.loading}
        matchesError={invMatches.error}
        targets={invTargets}
        onTargetsChange={setInvTargets}
        includeLogs={invIncludeLogs}
        onIncludeLogsChange={setInvIncludeLogs}
        running={running}
        starting={starting}
        changed={invChanged}
        canRun={canRunInvestigation}
        onRun={onRunFromBar}
        onStop={onStop}
      />
      {investigating && (running || starting) && (
        <InvestigationTabs
          tab={tab}
          onSelect={(id) => navigateTo(id)}
          counts={{ incident: view.state?.problems?.length || 0 }}
        />
      )}
    </div>
  )

  const maximized = consoleDock.maximized

  const filterLogsFromPatterns = useCallback(async (term) => {
    const q = String(term || '').trim()
    if (q) {
      stream.setFollow(false)
      setLogsFindFromPatterns({ id: Date.now(), query: q })
      consoleDock.openStream()
      navigateTo('incident')
      if (dockTabs.logTabs.length > 0) {
        if (dockTabs.activeItem?.kind !== 'log') {
          const lastLog = dockTabs.logTabs[dockTabs.logTabs.length - 1]
          if (lastLog) dockTabs.selectTab(`log:${lastLog.streamId}`)
        }
        return
      }
      if (running) {
        const podNames = (view.state?.snapshot?.pods || [])
          .map((p) => p?.name)
          .filter(Boolean)
        if (!podNames.length) return
        await dockTabs.openLogStream({
          targets: podTargets(investigationNs, podNames),
          tailLines: prefs.tailLines,
        })
      }
    }
  }, [stream, consoleDock, running, dockTabs, investigationNs, view.state?.snapshot?.pods, prefs.tailLines, navigateTo])

  const handleCloseDock = useCallback(() => {
    consoleDock.closeAll()
    logStreams.stopAll()
    dockTabs.clearAllTabs()
  }, [consoleDock, logStreams, dockTabs])

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

      <HeaderSlotProvider>
      <EventHistoryProvider value={eventHistory}>
      <TerminalLaunchProvider launch={launchTerminalCommand}>
      <InvestigableProvider value={investigable}>
      <LogStreamProvider value={logStreamActions}>
      <div className={`app-root ${maximized ? 'stream-maximized' : ''}`}>
      <ClusterConnectionBanner
        connection={connection}
        onReconnect={reconnect}
        onDismiss={dismiss}
        onOpenSettings={() => navigateTo({ tab: 'settings', settingsSection: 'kubernetes' })}
        onOpenProxySettings={openProxySettings}
        reconnectBusy={reconnectBusy}
      />
      <MainContent
        tab={tab}
        view={view}
        running={running}
        starting={starting}
        activeQuery={activeQuery}
        cluster={scopedCluster}
        clusterStatus={clusterStatus}
        resourceCatalog={resourceCatalog}
        browseScope={effectiveBrowseScope}
        onBrowseScopeChange={onBrowseScopeChange}
        browseScopeLocked={resourcesNamespaceLocked || (investigating && Boolean(invNamespace))}
        savedBrowseScopeLabel={savedBrowseScopeLabel}
        investigationNs={investigationNs || (investigating ? invNamespace : '')}
        investigateChrome={investigateChrome}
        resourcesBrowseLens={resourcesBrowseLens}
        onResourcesBrowseLensChange={onResourcesBrowseLensChange}
        investigationSession={{
          active: resourcesInvestigationActive,
          namespaceLabel: lockedResourcesScope
            ? browseScopeLabel(lockedResourcesScope, { namespaces: cluster.namespaces })
            : '',
          query: activeQuery || (starting ? query : ''),
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
        statusLoading={statusLoading}
        onReconnect={reconnect}
        onDisconnect={disconnect}
        reconnectBusy={reconnectBusy}
        monitoringPaused={monitoringPaused}
        connection={connection}
        connecting={connecting}
        onContextChange={onConnectContext}
        onSetDefaultContext={onSetDefaultContext}
        onToggleFavoriteContext={onToggleFavoriteContext}
        onClearDefaultContext={onClearDefaultContext}
        onOpenProxySettings={openProxySettings}
        onOpenMcpSetup={() => setMcpSetupOpen(true)}
        onTerminalShellChange={handleTerminalShellChange}
        onOpenTerminalShellPicker={handleOpenTerminalShellPicker}
        terminalShellRestartToken={terminalShellRestartToken}
        settingsSection={settingsSection}
        onSettingsSectionChange={setSettingsSection}
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
          visible={consoleDock.expanded && dockAllowed}
          tabs={dockTabs.tabs}
          logTabs={dockTabs.logTabs}
          activeKey={dockTabs.activeKey}
          activeItem={dockTabs.activeItem}
          onSelectTab={dockTabs.selectTab}
          onCloseTab={dockTabs.closeTab}
          onCloseTabs={dockTabs.closeTabs}
          onNewTerminal={openNewTerminalTab}
          onStreamSelection={handleStreamCurrentSelection}
          canStreamSelection={canStreamSelection}
          selectionLabel={selectionStreamLabel}
          height={consoleDock.height}
          maximized={consoleDock.maximized}
          onMinimize={consoleDock.minimize}
          onMaximize={consoleDock.maximize}
          onRestore={consoleDock.restore}
          onClose={handleCloseDock}
          onResizeStart={consoleDock.startResize}
          logStreams={logStreams}
          streamProps={{
            running,
            investigationPods: view.state?.snapshot?.pods,
            investigationNamespace: investigationNs,
            tailLines: prefs.tailLines,
            defaultFollow: Boolean(prefs.followLogsByDefault),
            streamFontSize: prefs.streamFontSize,
            streamDense: prefs.streamDense,
            streamWrapLines: prefs.streamWrapLines,
            logsFindFromPatterns,
            onLogsFindFromPatternsHandled: () => setLogsFindFromPatterns(null),
          }}
          terminalTabs={terminalTabs}
          terminalProps={{
            cluster,
            shellPref: prefs.terminalShell,
            appTheme: resolvedTheme,
            onChangeShell: handleOpenTerminalShellPicker,
            shellRestartToken: terminalShellRestartToken,
            launchRequest: terminalLaunchRequest,
            onLaunchHandled: () => setTerminalLaunchRequest(null),
          }}
        />
      </div>
      </div>
      </LogStreamProvider>
      </InvestigableProvider>
      </TerminalLaunchProvider>
      </EventHistoryProvider>
      </HeaderSlotProvider>

      <McpSetupDialog
        open={mcpSetupOpen}
        onClose={() => setMcpSetupOpen(false)}
        contexts={cluster?.contexts || []}
      />

      <SettingsDialog
        open={settingsOpen}
        onClose={() => setSettingsOpen(false)}
        cluster={cluster}
        themeId={themeId}
        onThemeChange={setTheme}
        prefs={prefs}
        onPrefsChange={setPreferences}
        onClusterRefresh={syncNow}
        onReconnect={reconnect}
        onDisconnect={disconnect}
        reconnectBusy={reconnectBusy}
        monitoringPaused={monitoringPaused}
        connection={connection}
        onClearDefaultContext={onClearDefaultContext}
        onTerminalShellChange={handleTerminalShellChange}
        section={settingsSection}
        onSectionChange={setSettingsSection}
      />

      <TerminalShellModal
        open={terminalShellPickerOpen}
        initialChoice={prefs.terminalShell || 'system'}
        confirmLabel="Save"
        onConfirm={handleTerminalShellConfirm}
        onCancel={() => {
          setTerminalShellPickerOpen(false)
        }}
      />
    </>
  )
}
