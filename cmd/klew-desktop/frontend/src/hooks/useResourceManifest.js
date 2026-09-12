import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  loadResourceManifest,
  manifestCacheKey,
  peekManifestCache,
} from '../lib/manifestCache.js'
import { manifestTargetKey } from '../lib/manifestTarget.js'

/**
 * Fetches read-only kubectl get -o yaml for the selected resource.
 * Uses a small in-memory stale-while-revalidate cache (show cached YAML, refresh async).
 */
export function useResourceManifest(target, cluster) {
  const reqRef = useRef(0)

  const ctx = cluster?.selectedContext || cluster?.currentContext || ''
  const targetKey = manifestTargetKey(target)
  const cacheKey = useMemo(
    () => manifestCacheKey(target, cluster),
    [target, cluster, targetKey, ctx],
  )

  const [manifest, setManifest] = useState(() => peekManifestCache(cacheKey))
  const [loading, setLoading] = useState(() => Boolean(cacheKey && !peekManifestCache(cacheKey)))
  const [refreshing, setRefreshing] = useState(false)
  const [error, setError] = useState('')

  const runFetch = useCallback((background = false) => {
    if (!target?.name || !ctx || !cacheKey) return Promise.resolve()

    const id = ++reqRef.current
    if (background) {
      setRefreshing(true)
    } else {
      setLoading(true)
    }
    setError('')

    return loadResourceManifest(target, cluster, cacheKey)
      .then((result) => {
        if (reqRef.current !== id) return
        setManifest(result)
        if (result?.error) {
          setError(result.error)
        } else {
          setError('')
        }
      })
      .catch((err) => {
        if (reqRef.current !== id) return
        if (!background) setManifest(null)
        setError(String(err?.message || err || 'Failed to fetch manifest'))
      })
      .finally(() => {
        if (reqRef.current !== id) return
        setLoading(false)
        setRefreshing(false)
      })
  }, [target, cluster, cacheKey, ctx])

  useEffect(() => {
    if (!cacheKey) {
      setManifest(null)
      setError('')
      setLoading(false)
      setRefreshing(false)
      return undefined
    }

    const cached = peekManifestCache(cacheKey)
    setManifest(cached)
    setError('')
    setRefreshing(false)
    setLoading(!cached)

    runFetch(Boolean(cached))

    return () => {
      reqRef.current += 1
    }
  }, [cacheKey, runFetch])

  const refresh = useCallback(() => runFetch(Boolean(manifest)), [runFetch, manifest])

  return { manifest, loading, refreshing, error, refresh }
}
