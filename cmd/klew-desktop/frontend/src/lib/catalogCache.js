/** Stale-while-revalidate cache for browse catalog API responses. */

export const CACHE_TTL = {
  /** Workload overview donuts + multi-kind fan-out (all-namespaces default) */
  overview: 5 * 60 * 1000,
  /** Cluster status / node counts */
  clusterStatus: 5 * 60 * 1000,
  /** Resource catalog index */
  catalog: 2 * 60 * 1000,
  /** Single-kind entity lists (all-namespaces default) */
  entities: 90 * 1000,
}

/** Tiered entity cache TTL — tighter for narrow scope where live watch is used. */
export function entitiesTtlForScope(apiParams = {}) {
  if (apiParams.allNamespaces) return 3 * 60 * 1000
  if (apiParams.namespaces?.length > 1) return 2 * 60 * 1000
  return 30 * 1000
}

/** Overview refresh cadence while the panel is visible. */
export function overviewPollMsForScope(apiParams = {}) {
  if (apiParams.allNamespaces) return 90 * 1000
  if (apiParams.namespaces?.length > 1) return 60 * 1000
  return 45 * 1000
}

export function overviewTtlForScope(apiParams = {}) {
  if (apiParams.allNamespaces) return 5 * 60 * 1000
  if (apiParams.namespaces?.length > 1) return 3 * 60 * 1000
  return 2 * 60 * 1000
}

const MAX_ENTRIES = 96

/** @type {Map<string, { data: unknown, at: number }>} */
const store = new Map()
/** @type {Map<string, Promise<unknown>>} */
const inflight = new Map()

export function buildCatalogCacheKey(...parts) {
  return parts.filter((p) => p != null && p !== '').join('|')
}

export function peekCatalogCache(key) {
  if (!key) return null
  return store.get(key)?.data ?? null
}

export function peekCatalogCacheEntry(key) {
  if (!key) return null
  return store.get(key) ?? null
}

export function isCatalogCacheFresh(key, ttlMs) {
  const entry = store.get(key)
  if (!entry) return false
  return Date.now() - entry.at < ttlMs
}

export function writeCatalogCache(key, data) {
  if (!key) return
  if (store.has(key)) store.delete(key)
  store.set(key, { data, at: Date.now() })
  while (store.size > MAX_ENTRIES) {
    const oldest = store.keys().next().value
    store.delete(oldest)
  }
}

export function invalidateCatalogCache(prefix = '') {
  if (!prefix) {
    store.clear()
    inflight.clear()
    return
  }
  for (const key of [...store.keys()]) {
    if (key.startsWith(prefix)) store.delete(key)
  }
  for (const key of [...inflight.keys()]) {
    if (key.startsWith(prefix)) inflight.delete(key)
  }
}

/**
 * Return cached data when fresh; otherwise fetch and update cache.
 * When stale data exists, returns it immediately and revalidates in the background via fetchFn.
 */
export async function loadCatalogCached(key, ttlMs, fetchFn, { force = false } = {}) {
  if (!key) {
    const data = await fetchFn()
    return { data, fromCache: false, revalidated: true }
  }

  const entry = store.get(key)
  const fresh = !force && entry && Date.now() - entry.at < ttlMs
  if (fresh) {
    return { data: entry.data, fromCache: true, revalidated: false }
  }

  const staleData = entry?.data ?? null
  const pending = inflight.get(key)
  if (pending && !force) {
    const data = await pending
    return { data, fromCache: Boolean(staleData), revalidated: true }
  }

  const promise = fetchFn()
    .then((data) => {
      writeCatalogCache(key, data)
      inflight.delete(key)
      return data
    })
    .catch((err) => {
      inflight.delete(key)
      if (staleData != null) return staleData
      throw err
    })

  inflight.set(key, promise)

  if (staleData != null && !force) {
    promise.catch(() => {})
    return { data: staleData, fromCache: true, revalidated: true }
  }

  const data = await promise
  return { data, fromCache: false, revalidated: true }
}

export function entitiesListCacheKey({
  resourceId,
  ctx,
  kubeconfig,
  nsKey,
  clusterScoped = false,
}) {
  return buildCatalogCacheKey(
    'entities',
    resourceId,
    ctx,
    kubeconfig,
    nsKey,
    clusterScoped ? 'cluster' : 'ns',
  )
}

export function overviewCacheKey({ ctx, kubeconfig, nsKey, loadableKey }) {
  return buildCatalogCacheKey('overview', ctx, kubeconfig, nsKey, loadableKey)
}

export function catalogIndexCacheKey({ ctx, kubeconfig, nsKey }) {
  return buildCatalogCacheKey('catalog', ctx, kubeconfig, nsKey)
}

export function clusterStatusCacheKey(ctx) {
  return buildCatalogCacheKey('cluster-status', ctx)
}
