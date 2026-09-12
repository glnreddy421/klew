import { GetResourceManifest } from '../../wailsjs/go/main/App'
import { manifestTargetKey } from './manifestTarget.js'

const MAX_ENTRIES = 64
const PREFETCH_FRESH_MS = 30_000

/** @type {Map<string, { data: object, at: number }>} */
const cache = new Map()
/** @type {Map<string, Promise<object>>} */
const inflight = new Map()

export function manifestCacheKey(target, cluster) {
  const ctx = cluster?.selectedContext || cluster?.currentContext || ''
  const kubeconfig = cluster?.kubeconfigPath || ''
  const targetKey = manifestTargetKey(target)
  if (!target?.name || !ctx || !targetKey) return ''
  return `${kubeconfig}|${ctx}|${targetKey}`
}

export function peekManifestCache(key) {
  if (!key) return null
  return cache.get(key)?.data ?? null
}

export function storeManifestCache(key, data) {
  if (!key || !data) return
  if (cache.has(key)) cache.delete(key)
  cache.set(key, { data, at: Date.now() })
  while (cache.size > MAX_ENTRIES) {
    const oldest = cache.keys().next().value
    cache.delete(oldest)
  }
}

export function clearManifestCache() {
  cache.clear()
  inflight.clear()
}

function buildManifestRequest(target, cluster) {
  const ctx = cluster?.selectedContext || cluster?.currentContext || ''
  const kubeconfig = cluster?.kubeconfigPath || ''
  return {
    resourceId: target.resourceId || '',
    kind: target.kind || '',
    name: target.name || '',
    namespace: target.namespace || '',
    clusterScoped: Boolean(target.clusterScoped),
    kubeconfig,
    context: ctx,
  }
}

export function fetchResourceManifest(target, cluster) {
  if (!target?.name) {
    return Promise.reject(new Error('resource name is required'))
  }
  const ctx = cluster?.selectedContext || cluster?.currentContext || ''
  if (!ctx) {
    return Promise.reject(new Error('cluster context is required'))
  }
  return GetResourceManifest(buildManifestRequest(target, cluster))
}

/**
 * Fetch manifest once per cache key; concurrent callers share the same promise.
 */
export function loadResourceManifest(target, cluster, cacheKey) {
  const key = cacheKey || manifestCacheKey(target, cluster)
  if (!key) {
    return Promise.reject(new Error('manifest cache key is required'))
  }

  const pending = inflight.get(key)
  if (pending) return pending

  const promise = fetchResourceManifest(target, cluster)
    .then((result) => {
      storeManifestCache(key, result)
      return result
    })
    .finally(() => {
      inflight.delete(key)
    })

  inflight.set(key, promise)
  return promise
}

/** Warm cache while inspecting so Manifest tab opens instantly when possible. */
export function prefetchResourceManifest(target, cluster) {
  const key = manifestCacheKey(target, cluster)
  if (!key) return Promise.resolve(null)

  const entry = cache.get(key)
  if (entry && Date.now() - entry.at < PREFETCH_FRESH_MS) {
    return Promise.resolve(entry.data)
  }
  if (inflight.has(key)) {
    return inflight.get(key)
  }

  return loadResourceManifest(target, cluster, key).catch(() => null)
}
