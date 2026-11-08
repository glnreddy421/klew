import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'
import { ScopePanel } from '../components/incident/ScopePanel'
import { InvestigationSignalsPanel } from '../components/incident/InvestigationSignalsPanel'
import { InvestigationLoadingBanner } from '../components/incident/InvestigationLoadingBanner'
import { InvestigationSessionBanner } from '../components/incident/InvestigationSessionBanner'
import {
  deriveMatchRows,
  findRowByKey,
  getMatchedObjects,
  inspectRowForKey,
  isInspectableKey,
  pickDefaultFocus,
  canonicalInspectKey,
} from '../lib/matches'
import { useFocusChainCatalog } from '../hooks/useFocusChainCatalog.js'
import { buildWorkloadTrace, isWorkloadTraceKind } from '../lib/workloadTrace.js'
import { buildComponentInspect } from '../lib/componentInspect'
import { mergeInspect, normalizeObjectDetail } from '../lib/objectDetails'
import { GetObjectDetails } from '../../wailsjs/go/main/App'
import { useScopeBrowse } from '../context/ScopeBrowseContext.jsx'
import { ResourcesPaneAutoFit } from '../components/shell/ResourcesPaneAutoFit.jsx'
import { hasClusterContexts } from '../lib/clusterIdentity.js'
import { useShellInspector } from '../context/ShellInspectorContext.jsx'
import {
  inspectPanelMode,
  layoutConfig,
  listChromeForMode,
  loadLayoutMode,
} from '../lib/incidentLayout'
import { withInspectNamespace } from '../lib/investigationContext'

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
  onReconnect,
  onDisconnect,
  onOpenProxySettings,
  reconnectBusy = false,
  clusterMonitoringEnabled = true,
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
    onReconnect,
    onDisconnect,
    onOpenProxySettings,
    reconnectBusy,
    clusterMonitoringEnabled,
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
    displayRows,
    catalog,
    catalogLoading,
    catalogEnriching,
    catalogError,
    cluster,
    clusterStatus,
    view,
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
    onReconnect,
    onDisconnect,
    onOpenProxySettings,
    reconnectBusy,
    clusterMonitoringEnabled,
    handleInspect,
    handleNavKindChange,
  } = ctx

  const shellInspector = useShellInspector()

  const openInspect = useCallback((key) => {
    handleInspect(key)
    shellInspector?.expandInspector?.()
  }, [handleInspect, shellInspector])

  const openNavKindChange = useCallback((payload) => {
    handleNavKindChange(payload)
  }, [handleNavKindChange])

  if (!hasClusterContexts(cluster)) {
    return (
      <div className="workbench-surface resources-workbench">
        <div className="workbench-empty">
          <h3>No kubeconfig contexts</h3>
          <p className="muted">
            Add a cluster context to your kubeconfig, or set the kubeconfig path in Settings.
            You can reconnect once the API is reachable.
          </p>
        </div>
      </div>
    )
  }

  return (
    <div className={`workbench-surface resources-workbench layout-${layoutMode} ${shellMode ? 'resources-workbench-shell' : ''}`}>
      {shellMode && <ResourcesPaneAutoFit />}
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
          inspectKey={inspectKey}
          onInspect={shellMode ? openInspect : handleInspect}
          onKindChange={openNavKindChange}
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
          onReconnect={onReconnect}
          onDisconnect={onDisconnect}
          onOpenProxySettings={onOpenProxySettings}
          reconnectBusy={reconnectBusy}
          clusterMonitoringEnabled={clusterMonitoringEnabled}
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
  }, [handleInspect])

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
    panelMode,
    detailLoading,
    detailError,
    inspectKey,
    workloadTrace,
    workloadTraceLoading,
  } = ctx

  return (
    <InvestigationSignalsPanel
      view={view}
      inspect={inspect}
      inspectRow={inspectRow}
      layoutMode={panelMode}
      expanded={layout.id === 'investigation-flow'}
      onInspect={openInspect}
      loading={detailLoading}
      error={detailError}
      browseMode
      workloadTrace={workloadTrace}
      workloadTraceLoading={workloadTraceLoading}
      emptyHint={
        inspectKey && !inspectRow
          ? `Could not open ${inspectKey}. Pick another row or check your cluster access.`
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
  onReconnect,
  onDisconnect,
  onOpenProxySettings,
  reconnectBusy = false,
  clusterMonitoringEnabled = true,
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
    return base
  }, [shellMode, layoutMode])

  const panelMode = shellMode ? 'detail-tabs' : inspectPanelMode(layoutMode)

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
    if (catalogBrowseActive || investigationLoading) return
    if (!allRows.length) {
      onInspectKeyChange?.(null)
      return
    }
    if (inspectKey && inspectKeyAllowed(inspectKey, allRows)) return
    const preferred = pickDefaultFocus(allRows)
    if (preferred) onInspectKeyChange?.(preferred)
  }, [allRows, inspectKey, onInspectKeyChange, inspectKeyAllowed, catalogBrowseActive, investigationLoading])

  // The inspected workload's connection trace (ingress → service → pods).
  const traceRow = useMemo(() => {
    if (!inspectKey) return null
    return findRowByKey(catalogEntities, inspectKey) || findRowByKey(allRows, inspectKey)
  }, [inspectKey, catalogEntities, allRows])

  const traceCatalogEnabled = Boolean(traceRow) && isWorkloadTraceKind(traceRow.kind)

  const traceCatalog = useFocusChainCatalog({
    cluster,
    catalog,
    browseScope,
    focusRow: traceRow,
    enabled: traceCatalogEnabled,
  })

  const rows = allRows

  const inspectRow = useMemo(() => {
    if (inspectKey) {
      const fromRows = findRowByKey(rows, inspectKey)
      if (fromRows) return withInspectNamespace(fromRows, view)
      const fromCatalog = findRowByKey(catalogEntities, inspectKey)
      if (fromCatalog) return withInspectNamespace(fromCatalog, view)
      const fromSnap = inspectRowForKey(inspectKey, view, allRows)
      if (fromSnap) return withInspectNamespace(fromSnap, view)
      return null
    }
    if (catalogBrowseActive) return null
    return rows[0] || null
  }, [rows, allRows, inspectKey, view, catalogBrowseActive, catalogEntities])

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

  const workloadTrace = useMemo(() => {
    if (!traceRow || !isWorkloadTraceKind(traceRow.kind)) return null
    return buildWorkloadTrace({
      focusRow: traceRow,
      catalogRows: traceCatalog.rows || [],
      view,
    })
  }, [traceRow, traceCatalog.rows, view])

  const displayRows = useMemo(() => {
    if (!layout.sortBySignal) return rows
    const rank = { critical: 0, degraded: 1, warning: 1, healthy: 2, unknown: 3 }
    return [...rows].sort((a, b) => (rank[a.status] ?? 3) - (rank[b.status] ?? 3))
  }, [rows, layout.sortBySignal])

  const resolveInspectNs = useCallback(() => (
    investigationNs
    || allRows[0]?.namespace
    || catalogEntities[0]?.namespace
    || ''
  ), [investigationNs, allRows, catalogEntities])

  const handleInspect = (key) => {
    onInspectKeyChange?.(canonicalInspectKey(key, resolveInspectNs()) || key)
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

  return {
    allMatches,
    allRows,
    investigationLoading,
    onNavigate,
    layoutMode,
    layout,
    inspectRow,
    displayRows,
    catalog,
    catalogLoading,
    catalogEnriching,
    catalogError,
    cluster,
    clusterStatus,
    view,
    inspectKey,
    listChrome,
    handleInspect,
    handleNavKindChange,
    inspect,
    panelMode,
    detailLoading,
    detailError,
    workloadTrace,
    workloadTraceLoading: traceCatalogEnabled && traceCatalog.loading,
    browseScope,
    onBrowseScopeChange,
    browseScopeLocked,
    savedBrowseScopeLabel,
    investigationNs,
    investigationSession,
    resourcesBrowseLens,
    onResourcesBrowseLensChange,
    onReconnect,
    onDisconnect,
    onOpenProxySettings,
    reconnectBusy,
    clusterMonitoringEnabled,
  }
}
