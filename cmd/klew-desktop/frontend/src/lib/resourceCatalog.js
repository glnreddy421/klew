/**
 * Merges discovery catalog (Kubernetes facts) with presentation schema (UI layout).
 */

import {
  BUILTIN_PRESENTATION,
  presentationKey,
  isExtensionGroup,
  defaultNamespaced,
  isDiscoveredOnlyEntry,
} from './resourcePresentation.js'

export function kindDisplayLabel(kind, fallback) {
  if (fallback) return fallback
  if (!kind) return ''
  return `${kind}s`
}

function flattenCatalogResources(catalog) {
  if (!catalog) return []
  if (Array.isArray(catalog.resources) && catalog.resources.length) {
    return catalog.resources
  }
  return [
    ...(catalog.namespaced || []),
    ...(catalog.extensions || []),
    ...(catalog.clusterScoped || []),
  ]
}

/** Index discovered descriptors by presentation key (group/resource). */
function descriptorPresentationKey(desc) {
  if (desc?.resource) {
    return presentationKey(desc.group || '', desc.resource)
  }
  if (desc?.id) {
    const parts = String(desc.id).split('/')
    if (parts.length === 2) return presentationKey('', parts[1])
    if (parts.length === 3) return presentationKey(parts[0], parts[2])
  }
  return null
}

function indexDiscoveredByPresentationKey(catalog) {
  const byKey = new Map()
  for (const desc of flattenCatalogResources(catalog)) {
    const key = descriptorPresentationKey(desc)
    if (!key || byKey.has(key)) continue
    byKey.set(key, desc)
  }
  return byKey
}

function deriveCountState(desc, matchCount) {
  // Investigation scope matches are authoritative only when non-zero.
  // Zero in-scope matches must not mask cluster-wide catalog counts.
  if (matchCount != null && matchCount > 0) {
    return { state: 'loaded', value: matchCount }
  }
  if (!desc) {
    return { state: 'unknown' }
  }
  if (desc.count?.state === 'loaded') {
    return { state: 'loaded', value: desc.count.count ?? 0 }
  }
  if (desc.count?.state === 'forbidden') {
    return { state: 'forbidden' }
  }
  if (desc.count?.state === 'unavailable') {
    return { state: 'unavailable' }
  }
  if (desc.count?.state === 'error') {
    return { state: 'error', error: desc.count.error }
  }
  if (desc.count?.state === 'loading') {
    return { state: 'loading' }
  }
  return { state: 'unknown' }
}

function mergeBuiltinEntry(entry, discovered, matchEntities) {
  const key = presentationKey(entry.group, entry.resource)
  const desc = discovered.get(key)
  if (isDiscoveredOnlyEntry(entry) && !desc) {
    return null
  }
  const matchCount = matchEntities?.length ?? null
  const countState = deriveCountState(desc, matchCount)
  let accessState = desc?.accessState || 'unknown'
  if (!desc) {
    accessState = 'unavailable'
  } else if (countState.state === 'forbidden') {
    accessState = 'forbidden'
  } else if (matchCount != null && matchCount > 0) {
    // In-scope investigation entities prove list access for this resource.
    accessState = 'allowed'
  } else if (accessState === 'error') {
    // SSRR/list hints are not final until a LIST is attempted.
    accessState = 'unknown'
  }

  return {
    presentationKey: key,
    resourceId: desc?.id || null,
    kind: desc?.kind || entry.kind,
    label: entry.displayName,
    sortOrder: entry.sortOrder,
    apiVersion: desc?.apiVersion,
    group: entry.group,
    version: desc?.version,
    resource: entry.resource,
    namespaced: desc?.namespaced ?? defaultNamespaced(entry),
    builtin: true,
    discoveredOnly: isDiscoveredOnlyEntry(entry),
    discovered: !!desc,
    accessState,
    countState,
    matchCount: matchCount ?? 0,
    count: matchCount > 0 ? matchCount : (countState.state === 'loaded' ? countState.value : 0),
    items: (matchEntities || []).map((row) => ({ row, children: [] })),
  }
}

function mergeCustomEntry(desc, matchEntities) {
  const matchCount = matchEntities?.length ?? null
  const countState = deriveCountState(desc, matchCount)
  let accessState = desc.accessState || 'unknown'
  if (countState.state === 'forbidden') {
    accessState = 'forbidden'
  } else if (matchCount != null && matchCount > 0) {
    accessState = 'allowed'
  } else if (accessState === 'error') {
    accessState = 'unknown'
  }
  return {
    presentationKey: presentationKey(desc.group, desc.resource),
    resourceId: desc.id,
    kind: desc.kind,
    label: kindDisplayLabel(desc.kind),
    sortOrder: 999,
    apiVersion: desc.apiVersion,
    group: desc.group,
    version: desc.version,
    resource: desc.resource,
    namespaced: desc.namespaced,
    builtin: false,
    discovered: true,
    accessState,
    countState,
    matchCount: matchCount ?? 0,
    count: matchCount > 0 ? matchCount : (countState.state === 'loaded' ? countState.value : 0),
    items: (matchEntities || []).map((row) => ({ row, children: [] })),
  }
}

function indexRowsByKind(rows) {
  const byKind = {}
  for (const row of rows || []) {
    const kind = row.kind || row.ref?.kind || 'Unknown'
    if (!byKind[kind]) byKind[kind] = []
    byKind[kind].push(row)
  }
  return byKind
}

function indexRowsByResourceId(rows) {
  const byId = new Map()
  for (const row of rows || []) {
    const id = row.resourceId || row.gvr
    if (!id) continue
    if (!byId.has(id)) byId.set(id, [])
    byId.get(id).push(row)
  }
  return byId
}

function syntheticPodRow(p) {
  return {
    key: `Pod/${p.name}`,
    kind: 'Pod',
    name: p.name,
    resourceId: 'v1/pods',
    ref: { kind: 'Pod', name: p.name, namespace: p.namespace },
    ready: p.ready ? 1 : 0,
    total: 1,
    restarts: p.restartCount || 0,
    status: p.ready ? 'healthy' : 'degraded',
  }
}

function buildPodItems(allRows, podList, podMatchRows) {
  const byKey = new Map()
  for (const p of podList || []) {
    if (!p?.name) continue
    const key = `Pod/${p.name}`
    const existing = (allRows || []).find((r) => r.key === key)
    byKey.set(key, existing || syntheticPodRow(p))
  }
  for (const row of podMatchRows || []) {
    const key = row.key || `Pod/${row.name}`
    if (!byKey.has(key)) byKey.set(key, row)
  }
  return [...byKey.values()].sort((a, b) => (a.name || '').localeCompare(b.name || ''))
}

function matchEntitiesFor(entry, desc, byKind, byResourceId, allRows, pods) {
  if (desc?.id && byResourceId.has(desc.id)) {
    return byResourceId.get(desc.id)
  }
  const kind = desc?.kind || entry?.kind
  if (kind === 'Pod') {
    return buildPodItems(allRows, pods, byKind.Pod || [])
  }
  if (!kind) return []
  const kindRows = byKind[kind] || []
  if (!desc?.id) return kindRows
  return kindRows.filter((row) => {
    const rowGvr = row.resourceId || row.gvr
    if (!rowGvr) return true
    return rowGvr === desc.id
  })
}

/**
 * Build semantic navigation tree from discovery catalog + investigation scope.
 */
export function buildCatalogScopeTree(catalog, rows, pods) {
  const list = Array.isArray(rows) ? rows : []
  if (!catalog) {
    return buildRowsOnlyScopeTree(list, pods)
  }

  const byKind = indexRowsByKind(list)
  const byResourceId = indexRowsByResourceId(list)
  const discovered = indexDiscoveredByPresentationKey(catalog)
  const assignedKeys = new Set()

  const categories = []

  for (const cat of BUILTIN_PRESENTATION) {
    const kinds = cat.resources
      .map((entry) => {
        const key = presentationKey(entry.group, entry.resource)
        assignedKeys.add(key)
        const desc = discovered.get(key)
        const entities = matchEntitiesFor(entry, desc, byKind, byResourceId, list, pods)
        return mergeBuiltinEntry(entry, discovered, entities)
      })
      .filter(Boolean)
    categories.push({
      id: cat.id,
      label: cat.label,
      count: kinds.reduce((n, k) => n + (k.matchCount || 0), 0),
      kinds,
    })
  }

  const customKinds = []
  const otherKinds = []

  for (const desc of flattenCatalogResources(catalog)) {
    const key = presentationKey(desc.group, desc.resource)
    if (assignedKeys.has(key)) continue
    assignedKeys.add(key)
    const entities = matchEntitiesFor(null, desc, byKind, byResourceId, list, pods)
    const node = mergeCustomEntry(desc, entities)
    if (desc.source === 'extension' || isExtensionGroup(desc.group)) {
      customKinds.push(node)
    } else {
      otherKinds.push(node)
    }
  }

  if (customKinds.length) {
    customKinds.sort((a, b) => a.label.localeCompare(b.label))
    categories.push({
      id: 'custom',
      label: 'Custom Resources',
      count: customKinds.reduce((n, k) => n + (k.matchCount || 0), 0),
      kinds: customKinds,
    })
  }

  if (otherKinds.length) {
    otherKinds.sort((a, b) => a.label.localeCompare(b.label))
    categories.push({
      id: 'other',
      label: 'Other',
      count: otherKinds.reduce((n, k) => n + (k.matchCount || 0), 0),
      kinds: otherKinds,
    })
  }

  return { count: list.length, categories }
}

/**
 * Merge lazily listed entity counts into the navigation tree.
 * Keys are catalog resource IDs (e.g. v1/nodes).
 */
export function applyLazyCountsToTree(tree, lazyCountsByResourceId) {
  if (!tree?.categories?.length || !lazyCountsByResourceId) return tree
  const entries = Object.entries(lazyCountsByResourceId)
  if (!entries.length) return tree

  const categories = tree.categories.map((cat) => {
    const kinds = cat.kinds.map((kindGroup) => {
      const patch = kindGroup.resourceId && lazyCountsByResourceId[kindGroup.resourceId]
      if (!patch || kindGroup.matchCount > 0) return kindGroup
      if (patch.accessState === 'forbidden') {
        return {
          ...kindGroup,
          accessState: 'forbidden',
          countState: { state: 'forbidden' },
          count: 0,
        }
      }
      if (patch.accessState === 'unavailable') {
        return {
          ...kindGroup,
          accessState: 'unavailable',
          countState: { state: 'unavailable' },
          count: 0,
        }
      }
      if (patch.count == null) return kindGroup
      return {
        ...kindGroup,
        accessState: 'allowed',
        countState: { state: 'loaded', value: patch.count },
        count: patch.count,
      }
    })
    return {
      ...cat,
      kinds,
      count: kinds.reduce((n, k) => {
        const d = getKindCountDisplay(k)
        if (d.className === 'count-active' && d.label) return n + Number(d.label)
        return n
      }, 0),
    }
  })

  return { ...tree, categories }
}

/** Fallback when catalog has not loaded. */
export function buildRowsOnlyScopeTree(rows, pods) {
  const list = Array.isArray(rows) ? rows : []
  const byKind = indexRowsByKind(list)
  const kinds = Object.keys(byKind)
    .sort((a, b) => a.localeCompare(b))
    .map((kind) => {
      const entities = kind === 'Pod'
        ? buildPodItems(list, pods, byKind.Pod || [])
        : byKind[kind]
      return {
        resourceId: kind,
        kind,
        label: kindDisplayLabel(kind),
        builtin: false,
        discovered: false,
        accessState: 'unknown',
        countState: { state: 'loaded', value: entities.length },
        count: entities.length,
        matchCount: entities.length,
        items: entities.map((row) => ({ row, children: [] })),
      }
    })

  return {
    count: list.length,
    categories: kinds.length
      ? [{ id: 'in-scope', label: 'In scope', count: list.length, kinds }]
      : [],
  }
}

export function resourceMetadataTitle(kindGroup) {
  if (!kindGroup) return ''
  const lines = [kindGroup.label || kindGroup.kind]
  if (kindGroup.apiVersion) lines.push(kindGroup.apiVersion)
  if (kindGroup.namespaced === false) lines.push('cluster scoped')
  else if (kindGroup.namespaced) lines.push('namespaced')
  if (kindGroup.resource) lines.push(`resource: ${kindGroup.resource}`)
  if (!kindGroup.discovered && kindGroup.builtin) lines.push('API not discovered on cluster')
  return lines.join('\n')
}

/**
 * Format sidebar count indicator.
 * Uses CSS classes instead of emoji for cleaner rendering.
 */
export function getKindCountDisplay(kindGroup) {
  if (!kindGroup) return { label: '', className: 'count-unknown' }

  const access = kindGroup.accessState
  const cs = kindGroup.countState

  if (kindGroup.matchCount != null && kindGroup.matchCount > 0) {
    return { label: String(kindGroup.matchCount), className: 'count-active' }
  }
  if (access === 'forbidden' || cs?.state === 'forbidden') {
    return { label: '', className: 'count-denied', title: 'Access denied' }
  }
  if (cs?.state === 'loaded') {
    return {
      label: String(cs.value ?? 0),
      className: cs.value === 0 ? 'count-zero' : 'count-active',
    }
  }
  if (!kindGroup.discovered && kindGroup.builtin && !kindGroup.discoveredOnly) {
    return { label: '', className: 'count-unavailable', title: 'API not available' }
  }
  if (access === 'unavailable' || cs?.state === 'unavailable') {
    return { label: '', className: 'count-unavailable', title: 'Unavailable' }
  }
  if (access === 'error' || cs?.state === 'error') {
    return { label: '', className: 'count-unavailable', title: 'Error' }
  }
  if (cs?.state === 'loading') {
    return { label: '…', className: 'count-unknown' }
  }
  return { label: '', className: 'count-unknown' }
}

/** @deprecated use getKindCountDisplay */
export function formatKindCount(kindGroup) {
  const d = getKindCountDisplay(kindGroup)
  if (d.className === 'count-denied') return '🔒'
  if (d.className === 'count-unavailable') return '!'
  return d.label || '—'
}

export function isAccessDenied(kindGroup) {
  if (!kindGroup) return false
  return kindGroup.accessState === 'forbidden'
    || kindGroup.countState?.state === 'forbidden'
}

export function isUnavailable(kindGroup) {
  if (!kindGroup) return false
  return (!kindGroup.discovered && kindGroup.builtin && !kindGroup.discoveredOnly)
    || kindGroup.accessState === 'unavailable'
    || kindGroup.countState?.state === 'unavailable'
}

export function catalogEntityToRow(entity) {
  const kind = entity.kind || 'Resource'
  const ns = entity.namespace || ''
  const key = ns ? `${kind}/${ns}/${entity.name}` : `${kind}/${entity.name}`
  return {
    key,
    kind,
    name: entity.name,
    namespace: ns,
    resourceId: entity.resourceId,
    uid: entity.uid,
    ref: {
      kind,
      name: entity.name,
      namespace: ns,
      uid: entity.uid,
    },
    status: 'unknown',
    signal: entity.statusHint || '',
  }
}
