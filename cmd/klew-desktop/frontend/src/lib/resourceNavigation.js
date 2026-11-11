import { kindDisplayLabel } from './resourceCatalog.js'

/** Virtual kind id for category overview pages (e.g. Workloads → Overview). */
export const CATEGORY_OVERVIEW_KIND = '__overview__'

export const CATEGORY_OVERVIEW_GROUPS = new Set(['workloads'])

export function isCategoryOverview(kind) {
  return kind === CATEGORY_OVERVIEW_KIND
}

export function categorySupportsOverview(categoryId) {
  return CATEGORY_OVERVIEW_GROUPS.has(categoryId)
}

/**
 * Toggle one resource category. Categories open and close independently —
 * opening one never collapses the others.
 */
export function toggleExpandedGroup(expanded, groupId) {
  const next = new Set(expanded)
  if (next.has(groupId)) next.delete(groupId)
  else next.add(groupId)
  return next
}

/** Ensure a category is open, keeping every other open category as is. */
export function expandGroup(expanded, groupId) {
  if (!groupId || expanded.has(groupId)) return expanded
  const next = new Set(expanded)
  next.add(groupId)
  return next
}

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
        if (k.virtual) return true
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
  if (!tree || !groupId || !kind || isCategoryOverview(kind)) return []
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
 * Pick default group/kind — Workloads opens on Overview; else first kind with entities.
 */
export function pickDefaultKindSelection(tree, showEmpty) {
  for (const cat of visibleCategories(tree, showEmpty)) {
    if (categorySupportsOverview(cat.id)) {
      return {
        groupId: cat.id,
        kind: CATEGORY_OVERVIEW_KIND,
        resourceId: null,
      }
    }
  }
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
  if (groupId && isCategoryOverview(kind)) {
    const cat = tree?.categories?.find((c) => c.id === groupId)
    if (cat && categorySupportsOverview(groupId)) {
      return { groupId, kind: CATEGORY_OVERVIEW_KIND, resourceId: null }
    }
  }
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
  if (isCategoryOverview(kind)) return 'Overview'
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

/** kubectl short names, so `po`, `deploy` or `svc` find their kind. */
const KIND_SHORT_NAMES = {
  Pod: ['po'],
  Deployment: ['deploy'],
  ReplicaSet: ['rs'],
  ReplicationController: ['rc'],
  StatefulSet: ['sts'],
  DaemonSet: ['ds'],
  CronJob: ['cj'],
  Service: ['svc'],
  Endpoints: ['ep'],
  Ingress: ['ing'],
  NetworkPolicy: ['netpol'],
  PersistentVolume: ['pv'],
  PersistentVolumeClaim: ['pvc'],
  StorageClass: ['sc'],
  ConfigMap: ['cm'],
  ServiceAccount: ['sa'],
  HorizontalPodAutoscaler: ['hpa'],
  PodDisruptionBudget: ['pdb'],
  Namespace: ['ns'],
  Node: ['no'],
  Event: ['ev'],
  CustomResourceDefinition: ['crd', 'crds'],
}

function kindMatchRank(kindGroup, q) {
  const short = KIND_SHORT_NAMES[kindGroup.kind] || []
  const names = [kindGroup.label, kindGroup.kind, kindGroup.resource]
    .filter(Boolean)
    .map((s) => String(s).toLowerCase())
  if (short.includes(q) || names.includes(q)) return 0
  if (names.some((n) => n.startsWith(q))) return 1
  if (names.some((n) => n.includes(q))) return 2
  if (kindGroup.group && String(kindGroup.group).toLowerCase().includes(q)) return 3
  return -1
}

/**
 * Narrow categories to kinds matching query (label, kind, plural, API group or
 * kubectl short name). Categories with no match are dropped. `best` is the
 * closest match, for Enter-to-open.
 */
export function filterCategoriesByQuery(categories, query) {
  const q = String(query || '').trim().toLowerCase()
  if (!q) return { categories, best: null }
  let best = null
  const out = []
  for (const cat of categories || []) {
    const kinds = []
    for (const kindGroup of cat.kinds || []) {
      const rank = kindMatchRank(kindGroup, q)
      if (rank < 0) continue
      kinds.push(kindGroup)
      if (!best || rank < best.rank) best = { rank, categoryId: cat.id, kindGroup }
    }
    if (kinds.length) out.push({ ...cat, kinds })
  }
  return { categories: out, best }
}
