import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'
import { ScopePanel } from '../components/incident/ScopePanel'
import { InvestigationSignalsPanel } from '../components/incident/InvestigationSignalsPanel'
import { CollectingMatchesSplash } from '../components/incident/CollectingMatchesSplash'
import {
  deriveMatchRows,
  getMatchedObjects,
  inspectRowForKey,
  isInspectableKey,
  pickDefaultFocus,
} from '../lib/matches'
import { buildChainRows, buildFocusScope } from '../lib/focusScope'
import { buildComponentInspect } from '../lib/componentInspect'
import { mergeInspect, normalizeObjectDetail } from '../lib/objectDetails'
import { GetObjectDetails } from '../../wailsjs/go/main/App'
import { useResourceCatalog } from '../hooks/useResourceCatalog.js'
import {
  inspectPanelMode,
  inspectShowsFocusCta,
  layoutConfig,
  listChromeForMode,
  loadLayoutMode,
} from '../lib/incidentLayout'

const ResourcesWorkbenchContext = createContext(null)

export function ResourcesWorkbenchRoot({
  view,
  cluster,
  focusKey,
  focusPinned,
  onFocusChange,
  onClearFocus,
  collecting,
  layoutMode: layoutModeProp,
  inspectKey,
  onInspectKeyChange,
  shellMode = false,
  children,
}) {
  const value = useResourcesWorkbenchState({
    view,
    cluster,
    focusKey,
    focusPinned,
    onFocusChange,
    onClearFocus,
    collecting,
    layoutModeProp,
    inspectKey,
    onInspectKeyChange,
    shellMode,
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
    collecting,
    layoutMode,
    layout,
    focusPinned,
    inspectRow,
    displayRows,
    catalog,
    catalogLoading,
    catalogError,
    cluster,
    view,
    focusKey,
    inspectKey,
    listChrome,
    handleInspect,
    handleFocus,
    handleNavKindChange,
    onClearFocus,
  } = ctx

  if (allMatches.length === 0 && !allRows.length) {
    return (
      <div className="workbench-surface resources-workbench">
        {collecting ? (
          <CollectingMatchesSplash />
        ) : (
          <div className="workbench-empty">
            <h3>No resources in scope</h3>
            <p className="muted">Start an investigation or widen your query to browse Kubernetes objects.</p>
          </div>
        )}
      </div>
    )
  }

  return (
    <div className={`workbench-surface resources-workbench layout-${layoutMode} ${shellMode ? 'resources-workbench-shell' : ''}`}>
      {shellMode && focusPinned && inspectRow && (
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
          catalogLoading={catalogLoading}
          catalogError={catalogError}
          focusKey={focusKey}
          inspectKey={inspectKey}
          mode={focusPinned ? 'chain' : 'match'}
          onInspect={handleInspect}
          onFocus={handleFocus}
          onKindChange={handleNavKindChange}
          showFocusButton={listChrome.showFocusButton}
          entityView={listChrome.entityView}
          tableDensity={listChrome.tableDensity}
          showEmptyToggle={listChrome.showEmptyToggle}
          navInExplorer={shellMode}
        />
      </section>
    </div>
  )
}

export function ResourcesWorkbenchInspector() {
  const ctx = useContext(ResourcesWorkbenchContext)
  if (!ctx?.inspectRow && !ctx?.inspectKey) {
    return <p className="muted inspector-empty">Select an entity to inspect signals and details.</p>
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
    handleInspect,
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
      onInspect={handleInspect}
      loading={detailLoading}
      error={detailError}
      emptyHint={
        inspectKey && !inspectRow
          ? `Could not open ${inspectKey} for inspection.`
          : focusPinned
          ? 'Select a component in the focus chain to inspect it.'
          : 'Select an entity to see signals and details.'
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
  focusKey,
  focusPinned,
  onFocusChange,
  collecting,
  layoutModeProp,
  inspectKey,
  onInspectKeyChange,
  shellMode = false,
}) {
  const allMatches = getMatchedObjects(view)
  const allRows = useMemo(() => deriveMatchRows(view, allMatches), [view, allMatches])
  const { catalog, loading: catalogLoading, error: catalogError } = useResourceCatalog(cluster)

  const layoutMode = layoutModeProp || loadLayoutMode()
  const layout = layoutConfig(layoutMode)

  const listChrome = useMemo(() => {
    if (shellMode) {
      const tableChrome = listChromeForMode('clean-professional')
      return {
        ...tableChrome,
        entityView: 'table',
        tableDensity: 'standard',
        showFocusButton: true,
      }
    }
    return listChromeForMode(layoutMode)
  }, [shellMode, layoutMode])

  const panelMode = shellMode ? 'detail-tabs' : inspectPanelMode(layoutMode)
  const showFocusCta = shellMode ? true : inspectShowsFocusCta(layoutMode)

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
    if (focusPinned || catalogBrowseActive) return
    if (!allRows.length) {
      onInspectKeyChange?.(null)
      return
    }
    if (inspectKey && inspectKeyAllowed(inspectKey, allRows)) return
    const preferred = pickDefaultFocus(allRows)
    if (preferred) onInspectKeyChange?.(preferred)
  }, [allRows, focusPinned, inspectKey, onInspectKeyChange, inspectKeyAllowed, catalogBrowseActive])

  useEffect(() => {
    if (focusPinned && focusKey) onInspectKeyChange?.(focusKey)
  }, [focusPinned, focusKey, onInspectKeyChange])

  const focusRow = allRows.find((r) => r.key === focusKey) || allRows[0] || null
  const drillDown = useMemo(
    () => (focusPinned && focusRow ? buildFocusScope(view, focusRow) : null),
    [focusPinned, focusRow, view],
  )

  const rows = useMemo(
    () => (drillDown?.active ? buildChainRows(view, drillDown, allRows) : allRows),
    [drillDown, view, allRows],
  )

  useEffect(() => {
    if (!rows.length || catalogBrowseActive) return
    if (inspectKey && inspectKeyAllowed(inspectKey, rows)) return
    onInspectKeyChange?.(focusPinned ? (focusKey || rows[0].key) : (pickDefaultFocus(rows) || rows[0].key))
  }, [rows, inspectKey, focusPinned, focusKey, inspectKeyAllowed, onInspectKeyChange, catalogBrowseActive])

  const inspectRow = useMemo(() => {
    if (inspectKey) {
      const fromRows = rows.find((r) => r.key === inspectKey)
      if (fromRows) return fromRows
      const fromSnap = inspectRowForKey(inspectKey, view, allRows)
      if (fromSnap) return fromSnap
      return null
    }
    if (catalogBrowseActive) return null
    return rows.find((r) => r.key === focusKey) || rows[0] || null
  }, [rows, allRows, inspectKey, focusKey, view, catalogBrowseActive])

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
    collecting,
    layoutMode,
    layout,
    focusPinned,
    inspectRow,
    displayRows,
    catalog,
    catalogLoading,
    catalogError,
    cluster,
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
  }
}
