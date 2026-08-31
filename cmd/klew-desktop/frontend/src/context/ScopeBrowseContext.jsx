import { createContext, useContext, useMemo } from 'react'
import { buildCatalogScopeTree } from '../lib/resourceCatalog.js'
import { visibleCategories } from '../lib/resourceNavigation.js'
import { useResourceNavigation } from '../hooks/useResourceNavigation.js'
import { useCatalogEntities } from '../hooks/useCatalogEntities.js'
import { clusterScopeKey, useLazyResourceCounts } from '../hooks/useLazyResourceCounts.js'

const ScopeBrowseContext = createContext(null)

export function ScopeBrowseProvider({
  view,
  catalog,
  cluster,
  rows = [],
  chain = false,
  children,
}) {
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

  const nav = useResourceNavigation(baseTree, { enabled: !chain })

  const investigationEntities = nav.entities
  const kindGroup = nav.selectedKindGroup
  const canLazyLoad = Boolean(
    kindGroup?.resourceId
    && kindGroup.accessState !== 'forbidden'
    && kindGroup.countState?.state !== 'forbidden'
    && kindGroup.discovered
    && investigationEntities.length === 0,
  )

  const lazy = useCatalogEntities({ cluster, kindGroup, enabled: !chain && canLazyLoad })
  const tree = useLazyResourceCounts(baseTree, {
    clusterKey: clusterScopeKey(cluster),
    kindGroup,
    lazy,
  })

  const navWithCounts = useMemo(() => {
    const cat = tree?.categories?.find((c) => c.id === nav.selectedGroupId)
    const selectedKindGroup = cat?.kinds?.find((k) =>
      nav.selectedResourceId ? k.resourceId === nav.selectedResourceId : k.kind === nav.selectedKind,
    ) || nav.selectedKindGroup
    return {
      ...nav,
      categories: visibleCategories(tree, nav.showEmptyKinds),
      selectedKindGroup,
    }
  }, [nav, tree])

  const displayEntities = useMemo(
    () => (investigationEntities.length ? investigationEntities : lazy.entities),
    [investigationEntities, lazy.entities],
  )

  const effectiveKindGroup = useMemo(() => {
    const group = navWithCounts.selectedKindGroup || kindGroup
    if (!group) return null
    if (investigationEntities.length > 0) {
      return {
        ...group,
        accessState: 'allowed',
        countState: { state: 'loaded', value: investigationEntities.length },
      }
    }
    if (lazy.accessState === 'allowed') {
      return {
        ...group,
        accessState: 'allowed',
        countState: { state: 'loaded', value: lazy.entities.length },
      }
    }
    if (lazy.accessState === 'forbidden') {
      return { ...group, accessState: 'forbidden', countState: { state: 'forbidden' } }
    }
    if (lazy.accessState === 'unavailable') {
      return { ...group, accessState: 'unavailable', countState: { state: 'unavailable' } }
    }
    return group
  }, [navWithCounts.selectedKindGroup, kindGroup, lazy.accessState, lazy.entities.length, investigationEntities.length])

  const value = useMemo(() => ({
    tree,
    nav: navWithCounts,
    pods,
    displayEntities,
    effectiveKindGroup,
    lazy,
    canLazyLoad,
    chain,
  }), [tree, navWithCounts, pods, displayEntities, effectiveKindGroup, lazy, canLazyLoad, chain])

  return (
    <ScopeBrowseContext.Provider value={value}>
      {children}
    </ScopeBrowseContext.Provider>
  )
}

export function useScopeBrowse() {
  return useContext(ScopeBrowseContext)
}
