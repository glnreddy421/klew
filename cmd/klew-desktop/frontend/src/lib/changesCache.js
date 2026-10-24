import { GetPodRevision, GetWorkloadChanges } from '../../wailsjs/go/main/App'

/**
 * Revision history cache — same LRU + stale-while-revalidate pattern as the
 * manifest cache. Entries are keyed per cluster and object and carry the
 * workload generation: a newer generation from the live watch invalidates
 * the entry; otherwise entries older than REVALIDATE_MS are refreshed in the
 * background while the cached copy is shown.
 */
export const MAX_ENTRIES = 64
export const REVALIDATE_MS = 60_000

/** @type {Map<string, { data: object, at: number }>} */
const cache = new Map()
/** @type {Map<string, Promise<object>>} */
const inflight = new Map()

export function changesCacheKey(scope, kind, namespace, name) {
  if (!scope || !kind || !name) return ''
  return `${scope}|${kind}/${namespace || ''}/${name}`
}

export function peekChanges(key) {
  return key ? cache.get(key) || null : null
}

export function storeChanges(key, data, at = Date.now()) {
  if (!key || !data) return
  if (cache.has(key)) cache.delete(key)
  cache.set(key, { data, at })
  while (cache.size > MAX_ENTRIES) cache.delete(cache.keys().next().value)
}

export function clearChangesCache() {
  cache.clear()
  inflight.clear()
}

/**
 * "fresh" — use as is; "stale" — show, refetch in background; "invalid" — the
 * workload's spec changed (generation moved on), refetch before trusting it.
 */
export function entryFreshness(entry, { generation = 0, now = Date.now() } = {}) {
  if (!entry) return 'missing'
  const cachedGen = entry.data?.generation || 0
  if (generation && cachedGen && generation !== cachedGen) return 'invalid'
  return now - entry.at > REVALIDATE_MS ? 'stale' : 'fresh'
}

function dedupe(key, fn) {
  if (inflight.has(key)) return inflight.get(key)
  const p = fn().finally(() => inflight.delete(key))
  inflight.set(key, p)
  return p
}

export function fetchWorkloadChanges(key, kind, namespace, name) {
  return dedupe(key, () => GetWorkloadChanges(kind, namespace || '', name).then((data) => {
    storeChanges(key, data)
    return data
  }))
}

export function fetchPodRevision(key, namespace, name) {
  return dedupe(`pod|${key}`, () => GetPodRevision(namespace || '', name).then((data) => {
    storeChanges(`pod|${key}`, data)
    return data
  }))
}
