import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'
import { ScopePanel } from '../components/incident/ScopePanel'
import { InvestigationSignalsPanel } from '../components/incident/InvestigationSignalsPanel'
import { InvestigationLoadingBanner } from '../components/incident/InvestigationLoadingBanner'
import { InvestigationSessionBanner } from '../components/incident/InvestigationSessionBanner'
import {
  deriveMatchRows,
  getMatchedObjects,
  inspectRowForKey,
  isInspectableKey,
  pickDefaultFocus,
  synthesizeFocusRow,
} from '../lib/matches'
import { buildChainRows, buildFocusScope } from '../lib/focusScope'
import { useFocusChainCatalog } from '../hooks/useFocusChainCatalog.js'
import { buildComponentInspect } from '../lib/componentInspect'
import { mergeInspect, normalizeObjectDetail } from '../lib/objectDetails'
import { GetObjectDetails } from '../../wailsjs/go/main/App'
import { useScopeBrowse } from '../context/ScopeBrowseContext.jsx'
import { useShellInspector } from '../context/ShellInspectorContext.jsx'
import {
  inspectPanelMode,
  layoutConfig,
  listChromeForMode,
  loadLayoutMode,
} from '../lib/incidentLayout'

const ResourcesWorkbenchContext = createContext(null)

export function useResourcesWorkbench() {
  return useContext(ResourcesWorkbenchContext)
}

export function ResourcesWorkbenchRoot({
  view,
  cluster,
  clusterStatus = null,
  catalog: catalogProp,
  catalogLoading: catalogLoadingProp,
  catalogEnriching = false,
  catalogError: catalogErrorProp,
  focusKey,
  focusPinned,
  onFocusChange,
  onClearFocus,
  investigationLoading = false,
  onNavigate,
  layoutMode: layoutModeProp,
  inspectKey,
  onInspectKeyChange,
  shellMode = false,
  browseScope,
  onBrowseScopeChange,
  browseScopeLocked = false,
  savedBrowseScopeLabel = '',
  investigationNs = '',
  investigationSession = null,
  resourcesBrowseLens = 'matches',
  onResourcesBrowseLensChange,
  children,
}) {
  const value = useResourcesWorkbenchState({
    view,
    cluster,
    clusterStatus,
    catalog: catalogProp,
    catalogLoading: catalogLoadingProp,
    catalogEnriching,
    catalogError: catalogErrorProp,
    focusKey,
    focusPinned,
    onFocusChange,
    onClearFocus,
    investigationLoading,
    onNavigate,
    layoutModeProp,
    inspectKey,
    onInspectKeyChange,
    shellMode,
    browseScope,
    onBrowseScopeChange,
    browseScopeLocked,
    savedBrowseScopeLabel,
    investigationNs,
    investigationSession,
    resourcesBrowseLens,
    onResourcesBrowseLensChange,
  })
  return (
    <ResourcesWorkbenchContext.Provider value={value}>
      {children}
    </ResourcesWorkbenchContext.Provider>
  )
}

export function ResourcesWorkbenchView({ shellMode = false }) {
  const ctx = useContext(ResourcesWorkbenchContext)
  if (!ctx) {
    throw new Error('ResourcesWorkbenchView requires ResourcesWorkbenchRoot')
  }


  const {
    allMatches,
    allRows,
    investigationLoading,
    onNavigate,
    layoutMode,
    layout,
    focusPinned,
    inspectRow,
    displayRows,
    catalog,
    catalogLoading,
    catalogEnriching,
    catalogError,
    cluster,
    clusterStatus,
    view,
    focusKey,
    inspectKey,
    listChrome,
    browseScope,
    onBrowseScopeChange,
    browseScopeLocked,
    savedBrowseScopeLabel,
    investigationNs,
    investigationSession,
    resourcesBrowseLens,
    onResourcesBrowseLensChange,
    handleInspect,
    handleFocus,
    handleNavKindChange,
    onClearFocus,
  } = ctx

  const shellInspector = useShellInspector()

  const openInspect = useCallback((key) => {
    handleInspect(key)
    shellInspector?.expandInspector?.()
  }, [handleInspect, shellInspector])

  const openNavKindChange = useCallback((payload) => {
    handleNavKindChange(payload)
  }, [handleNavKindChange])

  const clusterReady = Boolean(cluster?.selectedContext || cluster?.currentContext)

  if (!clusterReady) {
    return (
      <div className="workbench-surface resources-workbench">
        <div className="workbench-empty">
          <h3>Connect a cluster</h3>
          <p className="muted">Select a context and namespace to browse Kubernetes resources.</p>
        </div>
      </div>
    )
  }

  return (
    <div className={`workbench-surface resources-workbench layout-${layoutMode} ${shellMode ? 'resources-workbench-shell' : ''}`}>
      {investigationSession?.active && (
        <InvestigationSessionBanner
          namespaceLabel={investigationSession.namespaceLabel}
          query={investigationSession.query}
          starting={investigationSession.starting}
        />
      )}
      {investigationLoading && (
        <InvestigationLoadingBanner onOpenOverview={() => onNavigate?.('incident')} />
      )}
      {focusPinned && inspectRow && (
        <div className="workbench-inline-chrome workbench-inline-chrome-compact">
          <span className="muted mono">{inspectRow.kind}/{inspectRow.name}</span>
          <button type="button" className="text-link-btn" onClick={() => onClearFocus?.()}>Clear focus</button>
        </div>
      )}

      <section className="workbench-panel workbench-catalog-panel workbench-entity-panel">
        <ScopePanel
          rows={displayRows}
          view={view}
          catalog={catalog}
          cluster={cluster}
          clusterStatus={clusterStatus}
          catalogLoading={catalogLoading}
          catalogEnriching={catalogEnriching}
          catalogError={catalogError}
          focusKey={focusKey}
          inspectKey={inspectKey}
          mode={focusPinned ? 'chain' : 'match'}
          onInspect={shellMode ? openInspect : handleInspect}
          onFocus={handleFocus}
          onKindChange={openNavKindChange}
          showFocusButton={listChrome.showFocusButton}
          entityView={listChrome.entityView}
          tableDensity={listChrome.tableDensity}
          showEmptyToggle={listChrome.showEmptyToggle}
          navInExplorer={shellMode}
          browseScope={browseScope}
          onBrowseScopeChange={onBrowseScopeChange}
          browseScopeLocked={browseScopeLocked}
          savedBrowseScopeLabel={savedBrowseScopeLabel}
          investigationNs={investigationNs}
          browseLens={resourcesBrowseLens}
          onBrowseLensChange={onResourcesBrowseLensChange}
          showBrowseLens={Boolean(investigationSession?.active)}
        />
      </section>
    </div>
  )
}

export function ResourcesWorkbenchInspector() {
  const ctx = useContext(ResourcesWorkbenchContext)
  const shellInspector = useShellInspector()
  const handleInspect = ctx?.handleInspect

  const openInspect = useCallback((key) => {
    handleInspect?.(key)
    shellInspector?.expandInspector?.()
  }, [handleInspect, shellInspector])

  if (!ctx?.inspectRow && !ctx?.inspectKey) {
    return (
      <div className="inspector-empty-state">
        <p className="inspector-empty-title">No resource selected</p>
        <p className="muted inspector-empty">
          Select a row in the table to browse Summary, Spec, Relationships, Events, and other live detail tabs.
        </p>
      </div>
    )
  }
  const {
    view,
    inspect,
    inspectRow,
    layoutMode,
    layout,
    focusPinned,
    showFocusCta,
    panelMode,
    handleFocus,
    detailLoading,
    detailError,
    inspectKey,
  } = ctx

  return (
    <InvestigationSignalsPanel
      view={view}
      inspect={inspect}
      inspectRow={inspectRow}
      layoutMode={panelMode}
      expanded={layout.id === 'investigation-flow'}
      focusPinned={focusPinned}
      showFocusCta={showFocusCta}
      onFocus={handleFocus}
      onInspect={openInspect}
      loading={detailLoading}
      error={detailError}
      browseMode
      emptyHint={
        inspectKey && !inspectRow
          ? `Could not open ${inspectKey}. Pick another row or check your cluster access.`
          : focusPinned
          ? 'Select a resource in the focus chain to load its details here.'
          : 'Select a row in the table to load live object details in the tabs below.'
      }
    />
  )
}

/** Catalog + rows for ScopeBrowseProvider wrapper in MainContent. */
export function useResourcesCatalog(view, cluster) {
  const allMatches = getMatchedObjects(view)
  const allRows = useMemo(() => deriveMatchRows(view, allMatches), [view, allMatches])
  const catalogState = useResourceCatalog(cluster)
  return { allMatches, allRows, ...catalogState }
}

function useResourcesWorkbenchState({
  view,
  cluster,
  clusterStatus = null,
  catalog: catalogProp,
  catalogLoading: catalogLoadingProp = false,
  catalogEnriching = false,
  catalogError: catalogErrorProp = '',
  focusKey,
  focusPinned,
  onFocusChange,
  investigationLoading = false,
  onNavigate,
  layoutModeProp,
  inspectKey,
  onInspectKeyChange,
  shellMode = false,
  browseScope,
  onBrowseScopeChange,
  browseScopeLocked = false,
  savedBrowseScopeLabel = '',
  investigationNs = '',
  investigationSession = null,
  resourcesBrowseLens = 'matches',
  onResourcesBrowseLensChange,
}) {
  const allMatches = getMatchedObjects(view)
  const allRows = useMemo(() => deriveMatchRows(view, allMatches), [view, allMatches])
  const catalog = catalogProp
  const catalogLoading = catalogLoadingProp
  const catalogError = catalogErrorProp
  const browseCtx = useScopeBrowse()
  const catalogEntities = browseCtx?.displayEntities || []

  const layoutMode = layoutModeProp || loadLayoutMode()
  const layout = layoutConfig(layoutMode)

  const listChrome = useMemo(() => {
    const base = shellMode
      ? {
        ...listChromeForMode('clean-professional'),
        entityView: 'table',
        tableDensity: 'standard',
        showEmptyToggle: false,
      }
      : listChromeForMode(layoutMode)
    return { ...base, showFocusButton: true }
  }, [shellMode, layoutMode])

  const panelMode = shellMode ? 'detail-tabs' : inspectPanelMode(layoutMode)
  const showFocusCta = true

  const isAdhocInspectable = useCallback((key, rowList) => {
    return isInspectableKey(key, view, rowList || allRows)
  }, [view, allRows])

  const inspectKeyRef = useRef(inspectKey)
  inspectKeyRef.current = inspectKey

  // null = not yet browsing catalog; [] = empty kind selected; [...] = entity keys
  const [catalogEntityKeys, setCatalogEntityKeys] = useState(null)
  const catalogBrowseActive = catalogEntityKeys !== null

  const inspectKeyAllowed = useCallback((key, rowList) => {
    if (!key) return false
    if (catalogEntityKeys?.includes(key)) return true
    return isAdhocInspectable(key, rowList || allRows)
  }, [isAdhocInspectable, allRows, catalogEntityKeys])

  useEffect(() => {
    if (focusPinned || catalogBrowseActive || investigationLoading) return
    if (!allRows.length) {
      onInspectKeyChange?.(null)
      return
    }
    if (inspectKey && inspectKeyAllowed(inspectKey, allRows)) return
    const preferred = pickDefaultFocus(allRows)
    if (preferred) onInspectKeyChange?.(preferred)
  }, [allRows, focusPinned, inspectKey, onInspectKeyChange, inspectKeyAllowed, catalogBrowseActive, investigationLoading])

  useEffect(() => {
    if (focusPinned && focusKey) onInspectKeyChange?.(focusKey)
  }, [focusPinned, focusKey, onInspectKeyChange])

  const focusRow = useMemo(() => {
    if (!focusKey) return allRows[0] || null
    return (
      allRows.find((r) => r.key === focusKey)
      || catalogEntities.find((r) => r.key === focusKey)
      || synthesizeFocusRow(focusKey, investigationNs || catalogEntities[0]?.namespace || '')
      || allRows[0]
      || null
    )
  }, [focusKey, allRows, catalogEntities, investigationNs])

  const focusChainCatalog = useFocusChainCatalog({
    cluster,
    catalog,
    browseScope,
    focusRow,
    enabled: focusPinned && Boolean(focusRow),
  })

  const chainSourceRows = useMemo(() => {
    const byKey = new Map(allRows.map((r) => [r.key, r]))
    for (const row of catalogEntities) {
      if (row?.key) byKey.set(row.key, row)
    }
    for (const row of focusChainCatalog.rows || []) {
      if (row?.key) byKey.set(row.key, row)
    }
    return [...byKey.values()]
  }, [allRows, catalogEntities, focusChainCatalog.rows])

  const drillDown = useMemo(
    () => (focusPinned && focusRow
      ? buildFocusScope(view, focusRow, { catalogRows: focusChainCatalog.rows || [] })
      : null),
    [focusPinned, focusRow, view, focusChainCatalog.rows],
  )

  const rows = useMemo(
    () => (drillDown?.active ? buildChainRows(view, drillDown, chainSourceRows) : allRows),
    [drillDown, view, allRows, chainSourceRows],
  )

  useEffect(() => {
    if (!rows.length || catalogBrowseActive) return
    if (inspectKey && inspectKeyAllowed(inspectKey, rows)) return
    onInspectKeyChange?.(focusPinned ? (focusKey || rows[0]?.key) : (pickDefaultFocus(rows) || rows[0]?.key))
  }, [rows, inspectKey, focusPinned, focusKey, inspectKeyAllowed, onInspectKeyChange, catalogBrowseActive])

  const inspectRow = useMemo(() => {
    if (inspectKey) {
      const fromRows = rows.find((r) => r.key === inspectKey)
      if (fromRows) return fromRows
      const fromCatalog = catalogEntities.find((r) => r.key === inspectKey)
      if (fromCatalog) return fromCatalog
      const fromSnap = inspectRowForKey(inspectKey, view, allRows)
      if (fromSnap) return fromSnap
      return null
    }
    if (catalogBrowseActive) return null
    return rows.find((r) => r.key === focusKey) || rows[0] || null
  }, [rows, allRows, inspectKey, focusKey, view, catalogBrowseActive, catalogEntities])

  const snapshotInspect = useMemo(
    () => (inspectRow ? buildComponentInspect(view, inspectRow) : null),
    [view, inspectRow],
  )

  const [liveDetail, setLiveDetail] = useState(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detailError, setDetailError] = useState(null)

  useEffect(() => {
    if (!inspectRow) {
      setLiveDetail(null)
      setDetailError(null)
      setDetailLoading(false)
      return
    }
    const kind = inspectRow.kind || inspectRow.ref?.kind
    const name = inspectRow.name || inspectRow.ref?.name
    const ns = inspectRow.namespace || inspectRow.ref?.namespace || ''
    if (!kind || !name) return

    let cancelled = false
    setDetailLoading(true)
    setDetailError(null)
    GetObjectDetails(kind, name, ns)
      .then((detail) => {
        if (cancelled) return
        setLiveDetail(normalizeObjectDetail(detail, inspectRow))
        setDetailLoading(false)
      })
      .catch((err) => {
        if (cancelled) return
        setLiveDetail(null)
        setDetailError(String(err?.message || err || 'Failed to load details'))
        setDetailLoading(false)
      })
    return () => { cancelled = true }
  }, [inspectRow?.key, inspectRow?.kind, inspectRow?.name, inspectRow?.namespace])

  const inspect = useMemo(
    () => mergeInspect(liveDetail, snapshotInspect),
    [liveDetail, snapshotInspect],
  )

  const displayRows = useMemo(() => {
    if (!layout.sortBySignal || focusPinned) return rows
    const rank = { critical: 0, degraded: 1, warning: 1, healthy: 2, unknown: 3 }
    return [...rows].sort((a, b) => (rank[a.status] ?? 3) - (rank[b.status] ?? 3))
  }, [rows, layout.sortBySignal, focusPinned])

  const handleInspect = (key) => {
    onInspectKeyChange?.(key)
    if (!focusPinned && allRows.some((r) => r.key === key)) {
      onFocusChange?.(key, { pinned: false })
    }
  }

  const handleNavKindChange = useCallback(({ entities }) => {
    const keys = (entities || []).map((e) => e.key).filter(Boolean)
    setCatalogEntityKeys((prev) => {
      if (prev !== null && prev.length === keys.length && prev.every((k, i) => k === keys[i])) {
        return prev
      }
      return keys
    })
    if (!keys.length) {
      onInspectKeyChange?.(null)
      return
    }
    const prev = inspectKeyRef.current
    if (prev && keys.includes(prev)) return
    onInspectKeyChange?.(keys[0])
  }, [onInspectKeyChange])

  const handleFocus = (key) => {
    onFocusChange?.(key, { pinned: true })
  }

  return {
    allMatches,
    allRows,
    investigationLoading,
    onNavigate,
    layoutMode,
    layout,
    focusPinned,
    inspectRow,
    displayRows,
    catalog,
    catalogLoading,
    catalogEnriching,
    catalogError,
    cluster,
    clusterStatus,
    view,
    focusKey,
    inspectKey,
    listChrome,
    handleInspect,
    handleFocus,
    handleNavKindChange,
    inspect,
    showFocusCta,
    panelMode,
    detailLoading,
    detailError,
    browseScope,
    onBrowseScopeChange,
    browseScopeLocked,
    savedBrowseScopeLabel,
    investigationNs,
    investigationSession,
    resourcesBrowseLens,
    onResourcesBrowseLensChange,
  }
}
