import { useMemo, useEffect } from 'react'
import { buildCatalogScopeTree } from '../../lib/resourceCatalog.js'
import { flattenTreeEntities, entitySearchPlaceholder, visibleCategories } from '../../lib/resourceNavigation.js'
import { useResourceNavigation } from '../../hooks/useResourceNavigation.js'
import { useCatalogEntities } from '../../hooks/useCatalogEntities.js'
import { clusterScopeKey, useLazyResourceCounts } from '../../hooks/useLazyResourceCounts.js'
import { useScopeBrowse } from '../../context/ScopeBrowseContext.jsx'
import { ResourceNav } from './ResourceNav.jsx'
import { EntityList } from './EntityList.jsx'
import { EntityTable } from './EntityTable.jsx'

/**
 * Scope UX — resource kinds + entity collection.
 * When navInExplorer is true, ResourceNav lives in the contextual explorer.
 */
export function ScopePanel({
  rows = [],
  view,
  catalog,
  cluster,
  catalogLoading = false,
  catalogError = '',
  focusKey,
  inspectKey,
  mode = 'match',
  onInspect,
  onFocus,
  onKindChange,
  showFocusButton = true,
  entityView = 'list',
  tableDensity = 'standard',
  showEmptyToggle = true,
  navInExplorer = false,
}) {
  const browseCtx = useScopeBrowse()
  const pods = view?.state?.snapshot?.pods || []
  const chain = mode === 'chain'
  const namespace = cluster?.selectedNamespace || cluster?.currentContext || ''

  const internal = useScopePanelState({
    view,
    catalog,
    cluster,
    rows,
    chain,
    enabled: !browseCtx && !chain,
  })

  const nav = browseCtx?.nav || internal.nav
  const displayEntities = browseCtx?.displayEntities ?? internal.displayEntities
  const effectiveKindGroup = browseCtx?.effectiveKindGroup ?? internal.effectiveKindGroup
  const lazy = browseCtx?.lazy ?? internal.lazy
  const canLazyLoad = browseCtx?.canLazyLoad ?? internal.canLazyLoad
  const tree = browseCtx?.tree ?? internal.tree

  const filteredEntities = useMemo(() => {
    const q = String(nav.entitySearchQuery || '').trim().toLowerCase()
    if (!q) return displayEntities
    return displayEntities.filter((row) =>
      `${row.name || ''} ${row.key || ''}`.toLowerCase().includes(q),
    )
  }, [displayEntities, nav.entitySearchQuery])

  const entityChangeSig = useMemo(
    () => `${nav.selectedResourceId || nav.selectedKind || ''}|${displayEntities.map((e) => e.key).join(',')}`,
    [nav.selectedResourceId, nav.selectedKind, displayEntities],
  )

  useEffect(() => {
    if (chain || !onKindChange) return
    onKindChange({
      groupId: nav.selectedGroupId,
      kind: nav.selectedKind,
      resourceId: nav.selectedResourceId,
      entities: displayEntities,
    })
  }, [chain, nav.selectedGroupId, nav.selectedKind, nav.selectedResourceId, entityChangeSig, onKindChange])

  const chainEntities = useMemo(() => (chain ? flattenTreeEntities(tree) : []), [chain, tree])
  const searchPlaceholder = entitySearchPlaceholder(nav.selectedKind, effectiveKindGroup?.label)

  if (chain) {
    return (
      <div className="scope-panel scope-panel-browse">
        <EntityList
          title="Focus chain"
          kind={null}
          entities={chainEntities}
          filteredEntities={chainEntities}
          inspectKey={inspectKey}
          focusKey={focusKey}
          showFocusButton={false}
          onSelect={onInspect}
          onFocus={onFocus}
          chainMode
        />
      </div>
    )
  }

  return (
    <div className="scope-panel scope-panel-browse">
      <div className="scope-toolbar">
        <input
          type="search"
          className="scope-toolbar-search"
          value={nav.entitySearchQuery}
          onChange={(e) => nav.setEntitySearchQuery(e.target.value)}
          placeholder={searchPlaceholder}
          aria-label="Filter entities"
        />
        <div className="scope-toolbar-meta">
          {catalogLoading && <span className="scope-toolbar-hint">Syncing catalog…</span>}
          {catalogError && !catalogLoading && (
            <span className="scope-toolbar-hint scope-toolbar-warn" title={catalogError}>
              Limited discovery
            </span>
          )}
          {namespace && <span className="scope-toolbar-ns mono">{namespace}</span>}
          {showEmptyToggle && (
            <button
              type="button"
              className={`scope-toolbar-btn ${nav.showEmptyKinds ? 'is-active' : ''}`}
              onClick={nav.toggleShowEmpty}
              aria-pressed={nav.showEmptyKinds}
            >
              Empty
            </button>
          )}
        </div>
      </div>

      <div className={`scope-browse-split ${entityView === 'table' ? 'scope-browse-table' : 'scope-browse-list'} ${navInExplorer ? 'scope-browse-no-nav' : ''}`}>
        {!navInExplorer && (
          <ResourceNav
            categories={nav.categories}
            expandedGroups={nav.expandedGroups}
            selectedGroupId={nav.selectedGroupId}
            selectedKind={nav.selectedKind}
            selectedResourceId={nav.selectedResourceId}
            onToggleGroup={nav.toggleGroup}
            onSelectKind={nav.selectKind}
          />
        )}
        {entityView === 'table' ? (
          <EntityTable
            kind={nav.selectedKind}
            kindGroup={effectiveKindGroup}
            entities={displayEntities}
            filteredEntities={filteredEntities}
            entitiesLoading={lazy.loading && canLazyLoad}
            pods={pods}
            density={tableDensity}
            hasSearchQuery={nav.entitySearchQuery.trim()}
            inspectKey={inspectKey}
            focusKey={focusKey}
            onSelect={onInspect}
          />
        ) : (
          <EntityList
            kind={nav.selectedKind}
            kindGroup={effectiveKindGroup}
            entities={displayEntities}
            filteredEntities={filteredEntities}
            entitiesLoading={lazy.loading && canLazyLoad}
            hasSearchQuery={nav.entitySearchQuery.trim()}
            inspectKey={inspectKey}
            focusKey={focusKey}
            showFocusButton={showFocusButton}
            onSelect={onInspect}
            onFocus={onFocus}
          />
        )}
      </div>
    </div>
  )
}

function useScopePanelState({ view, catalog, cluster, rows, chain, enabled }) {
  const pods = view?.state?.snapshot?.pods || []

  const rowSig = useMemo(
    () => rows.map((r) => `${r.key}:${r.status}:${r.ready}:${r.total}:${r.restarts}`).join('|'),
    [rows],
  )
  const podSig = useMemo(
    () => pods.map((p) => `${p.name}:${p.ready ? 1 : 0}:${p.restartCount || 0}`).join('|'),
    [pods],
  )
  const catalogSig = useMemo(
    () => (catalog ? `${catalog.generatedAt}:${catalog.resources?.length || 0}` : ''),
    [catalog],
  )

  const baseTree = useMemo(
    () => buildCatalogScopeTree(catalog, rows, pods),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [rowSig, podSig, catalogSig],
  )

  const navBase = useResourceNavigation(baseTree, { enabled })

  const investigationEntities = navBase.entities
  const kindGroup = navBase.selectedKindGroup
  const canLazyLoad = Boolean(
    kindGroup?.resourceId
    && kindGroup.accessState !== 'forbidden'
    && kindGroup.countState?.state !== 'forbidden'
    && kindGroup.discovered
    && investigationEntities.length === 0,
  )

  const lazy = useCatalogEntities({ cluster, kindGroup, enabled: enabled && canLazyLoad })
  const tree = useLazyResourceCounts(baseTree, {
    clusterKey: clusterScopeKey(cluster),
    kindGroup,
    lazy,
  })

  const nav = useMemo(() => ({
    ...navBase,
    categories: visibleCategories(tree, navBase.showEmptyKinds),
  }), [navBase, tree])

  const displayEntities = investigationEntities.length ? investigationEntities : lazy.entities

  const effectiveKindGroup = useMemo(() => {
    if (!kindGroup) return null
    if (investigationEntities.length > 0) {
      return {
        ...kindGroup,
        accessState: 'allowed',
        countState: { state: 'loaded', value: investigationEntities.length },
      }
    }
    if (lazy.accessState === 'allowed') {
      return {
        ...kindGroup,
        accessState: 'allowed',
        countState: { state: 'loaded', value: lazy.entities.length },
      }
    }
    if (lazy.accessState === 'forbidden') {
      return { ...kindGroup, accessState: 'forbidden', countState: { state: 'forbidden' } }
    }
    if (lazy.accessState === 'unavailable') {
      return { ...kindGroup, accessState: 'unavailable', countState: { state: 'unavailable' } }
    }
    return kindGroup
  }, [kindGroup, lazy.accessState, lazy.entities.length, investigationEntities.length])

  return { tree, nav, displayEntities, effectiveKindGroup, lazy, canLazyLoad }
}
