import { kindDisplayLabel } from './resourceCatalog.js'

/**
 * Categories with optional empty-kind filtering.
 * Built-in presentation rows always remain visible.
 */
export function visibleCategories(tree, showEmpty) {
  const categories = tree?.categories || []
  return categories
    .map((cat) => ({
      ...cat,
      kinds: cat.kinds.filter((k) => {
        if (k.discoveredOnly && !k.discovered) return false
        if (k.builtin && !k.discoveredOnly) return true
        if (!k.discovered) return false
        if (showEmpty) return true
        if (k.accessState === 'forbidden') return true
        if (k.matchCount > 0) return true
        if (k.countState?.state === 'loaded') return k.countState.value > 0
        if (k.countState?.state === 'unknown') return false
        return k.accessState !== 'unavailable'
      }),
    }))
    .filter((cat) => cat.kinds.length > 0)
}

/**
 * Entity rows for a selected group + resource kind.
 */
export function entitiesForKind(tree, groupId, kind, resourceId) {
  if (!tree || !groupId || !kind) return []
  const cat = tree.categories.find((c) => c.id === groupId)
  if (!cat) return []
  const kindGroup = cat.kinds.find((k) =>
    resourceId ? k.resourceId === resourceId : k.kind === kind,
  ) || cat.kinds.find((k) => k.kind === kind)
  if (!kindGroup) return []
  return (kindGroup.items || []).map((node) => node.row).filter(Boolean)
}

/**
 * Filter entity rows by name search (entity list only).
 */
export function filterEntitiesBySearch(entities, query) {
  const q = String(query || '').trim().toLowerCase()
  if (!q) return entities
  return entities.filter((row) => {
    const hay = `${row.name || ''} ${row.key || ''}`.toLowerCase()
    return hay.includes(q)
  })
}

/**
 * Pick default group/kind — first kind with entities or accessible built-in.
 */
export function pickDefaultKindSelection(tree, showEmpty) {
  for (const cat of visibleCategories(tree, showEmpty)) {
    for (const kindGroup of cat.kinds) {
      if (kindGroup.matchCount > 0) {
        return {
          groupId: cat.id,
          kind: kindGroup.kind,
          resourceId: kindGroup.resourceId,
        }
      }
    }
  }
  const first = tree?.categories?.[0]
  const firstKind = first?.kinds?.[0]
  if (first && firstKind) {
    return {
      groupId: first.id,
      kind: firstKind.kind,
      resourceId: firstKind.resourceId,
    }
  }
  return { groupId: null, kind: null, resourceId: null }
}

/**
 * Resolve selection after tree refresh; keep kind/resourceId when still valid.
 */
export function resolveKindSelection(tree, prev, showEmpty) {
  const { groupId, kind, resourceId } = prev || {}
  if (groupId && kind) {
    const cat = tree?.categories?.find((c) => c.id === groupId)
    const kindGroup = cat?.kinds?.find((k) =>
      resourceId ? k.resourceId === resourceId : k.kind === kind,
    )
    if (kindGroup) {
      return { groupId, kind, resourceId: kindGroup.resourceId || resourceId }
    }
  }
  return pickDefaultKindSelection(tree, showEmpty)
}

/**
 * Entity list search placeholder.
 */
export function entitySearchPlaceholder(kind, label) {
  if (label) return `Search ${label.toLowerCase()}…`
  if (!kind) return 'Search resources…'
  return `Search ${kindDisplayLabel(kind).toLowerCase()}…`
}

/**
 * Flatten all entities from a categorized tree (focus chain mode).
 */
export function flattenTreeEntities(tree) {
  const out = []
  const seen = new Set()
  for (const cat of tree?.categories || []) {
    for (const kindGroup of cat.kinds || []) {
      for (const node of kindGroup.items || []) {
        const key = node.row?.key
        if (!key || seen.has(key)) continue
        seen.add(key)
        out.push(node.row)
      }
    }
  }
  return out
}
