import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  StartInvestigation,
  StopInvestigation,
  DiscoverMatches,
  GetView,
  OpenNewWindow,
} from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime/runtime'
import { emptyView } from './lib/constants'
import { useCluster } from './hooks/useCluster'
import { useStreamPanel, PANEL_NORMAL } from './hooks/useStreamPanel'
import { useTheme } from './hooks/useTheme'
import { useSidebar } from './hooks/useSidebar'
import { usePreferences } from './hooks/usePreferences'
import { startOptionsFromPreferences } from './lib/preferences'
import { Sidebar } from './components/Sidebar'
import { TopBar } from './components/TopBar'
import { LiveStreamPanel } from './components/LiveStreamPanel'
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
  const [query, setQuery] = useState('')
  const [running, setRunning] = useState(false)
  const [starting, setStarting] = useState(false)
  const [activeQuery, setActiveQuery] = useState('')
  const [error, setError] = useState('')
  const [scopePicker, setScopePicker] = useState({
    open: false,
    matches: [],
    query: '',
    mode: 'initial',
  })

  const [focusKey, setFocusKey] = useState(null)
  const [focusPinned, setFocusPinned] = useState(false)

  const { cluster, syncing, syncNow, setContext, setNamespace } = useCluster()
  const stream = useStreamPanel()
  const { themeId, setTheme } = useTheme()
  const sidebar = useSidebar()
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
    setFocusKey(pickDefaultFocus(matchRows))
  }, [matchRows])

  const runInvestigation = useCallback(async (q) => {
    const trimmed = normalizeInvestigationQuery(q)
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
    setActiveQuery(trimmed)
    setFocusKey(null)
    setFocusPinned(false)
    stream.resetFilters()
    if (prefs.openStreamOnInvestigate) {
      stream.openPanel(PANEL_NORMAL)
    }
    stream.setFollow(Boolean(prefs.followLogsByDefault))
    const next = await GetView()
    applyView(next)
    // Live tail defaults to all pods (empty selection); user can narrow via the filter menu.
  }, [cluster, stream, prefs, applyView])

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
    setTab('incident')
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
    setTab('incident')
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
    await StopInvestigation()
    setRunning(false)
    setStarting(false)
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

  // Patterns is a full-page view; keep live tail on Overview and runtime tabs.
  const showStream = tab !== 'evidence' && tab !== 'graph' && tab !== 'patterns' && tab !== 'settings'
  const maximized = stream.panelState === 'maximized'

  const filterLogsFromPatterns = useCallback((term) => {
    const q = String(term || '').trim()
    stream.setSearch(q)
    stream.setMode('logs')
    if (q) {
      stream.setFollow(false)
      stream.openPanel(PANEL_NORMAL)
      setTab('incident')
    }
  }, [stream])

  return (
    <div className="shell">
      <Sidebar
        active={tab}
        onSelect={setTab}
        onSettings={() => setTab('settings')}
        collapsed={sidebar.collapsed}
        onToggle={sidebar.toggle}
      />

      <div className="workspace">
        <TopBar
          cluster={cluster}
          syncing={syncing}
          onSync={syncNow}
          onContextChange={onContextChange}
          onNamespaceChange={onNamespaceChange}
          onNewWindow={onNewWindow}
          query={query}
          onQueryChange={setQuery}
          onQueryClear={() => setQuery('')}
          running={running}
          starting={starting}
          activeQuery={activeQuery}
          onStart={onStart}
          onStop={onStop}
        />

        {error && <div className="banner-error">{error}</div>}

        <div className={`workspace-main ${maximized ? 'stream-maximized' : ''}`}>
          <div className="workspace-body">
            <MainContent
              tab={tab}
              view={view}
              running={running}
              starting={starting}
              scopePickerOpen={scopePicker.open}
              activeQuery={activeQuery}
              cluster={cluster}
              themeId={themeId}
              onThemeChange={setTheme}
              onOpenSettings={() => setTab('settings')}
              onOpenEvidence={() => setTab('evidence')}
              prefs={prefs}
              onPrefsChange={setPreferences}
              onClusterRefresh={syncNow}
              focusKey={focusKey}
              focusPinned={focusPinned}
              drillDown={drillDown}
              onFocusChange={handleFocusChange}
              onClearFocus={handleClearFocus}
              onFilterLogsFromPatterns={filterLogsFromPatterns}
            />
          </div>

          {showStream && (
            <LiveStreamPanel
              evidence={view.evidence}
              snapshotPods={view.state?.snapshot?.pods}
              query={running ? activeQuery : query}
              running={running}
              dropped={view.dropped}
              updatedAt={view.updatedAt}
              lastEventAt={view.state?.counters?.lastEventAt}
              panelState={stream.panelState}
              height={stream.height}
              mode={stream.mode}
              search={stream.search}
              selectedPods={stream.selectedPods}
              follow={stream.follow}
              paused={stream.paused}
              streamFontSize={prefs.streamFontSize}
              streamDense={prefs.streamDense}
              streamWrapLines={prefs.streamWrapLines}
              onModeChange={stream.setMode}
              onSearchChange={stream.setSearch}
              onTogglePod={stream.togglePod}
              onSelectMatched={stream.selectMatchedPods}
              onSelectAllPods={stream.selectAllPods}
              onFollowChange={stream.setFollow}
              onPausedChange={stream.setPaused}
              onTogglePaused={stream.togglePaused}
              onMinimize={stream.minimize}
              onMaximize={stream.maximize}
              onRestore={stream.restore}
              onClose={stream.close}
              onOpen={stream.openPanel}
              onResizeStart={stream.startResize}
            />
          )}
        </div>
      </div>

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
    </div>
  )
}
