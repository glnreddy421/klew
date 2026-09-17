import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { GetResourceCatalog } from '../../wailsjs/go/main/App'
import { browseScopeApiParams, normalizeBrowseScope } from '../lib/browseScope.js'
import {
  CACHE_TTL,
  catalogIndexCacheKey,
  loadCatalogCached,
  peekCatalogCache,
} from '../lib/catalogCache.js'

/**
 * Fetches discovery-driven resource catalog for the active cluster scope.
 * Loads a fast index first (no per-kind counts), then enriches counts in the background.
 * Cached ~2 minutes with stale-while-revalidate.
 */
export function useResourceCatalog(cluster, browseScope, { enabled = true } = {}) {
  const ctx = cluster?.selectedContext || cluster?.currentContext || ''
  const kubeconfig = cluster?.kubeconfigPath || ''
  const scope = useMemo(() => normalizeBrowseScope(browseScope), [browseScope])
  const apiParams = useMemo(() => browseScopeApiParams(scope), [scope])
  const nsKey = useMemo(() => (
    apiParams.allNamespaces
      ? '*'
      : (apiParams.namespaces.length ? apiParams.namespaces.join(',') : apiParams.namespace)
  ), [apiParams.allNamespaces, apiParams.namespace, apiParams.namespaces])

  const cacheKey = ctx && (apiParams.allNamespaces || apiParams.namespace || apiParams.namespaces.length)
    ? catalogIndexCacheKey({ ctx, kubeconfig, nsKey })
    : ''

  const [catalog, setCatalog] = useState(() => (cacheKey ? peekCatalogCache(cacheKey) : null))
  const [loading, setLoading] = useState(Boolean(cacheKey && !peekCatalogCache(cacheKey)))
  const [enriching, setEnriching] = useState(false)
  const [error, setError] = useState('')
  const reqRef = useRef(0)

  const reload = useCallback(async ({ force = false } = {}) => {
    if (!enabled || !cacheKey) {
      setCatalog(null)
      setLoading(false)
      setEnriching(false)
      setError('')
      return
    }

    const id = ++reqRef.current
    const hasCached = Boolean(peekCatalogCache(cacheKey))
    if (!hasCached) setLoading(true)
    else setEnriching(true)
    setError('')

    const baseOpts = {
      context: ctx,
      namespace: apiParams.namespace,
      allNamespaces: apiParams.allNamespaces,
      namespaces: apiParams.namespaces,
      kubeconfig,
      refresh: force,
    }

    try {
      const fastResult = await loadCatalogCached(
        `${cacheKey}|fast`,
        CACHE_TTL.catalog,
        () => GetResourceCatalog({ ...baseOpts, includeCounts: false }),
        { force },
      )
      if (reqRef.current !== id) return
      setCatalog(fastResult.data)
      setLoading(false)
      setEnriching(true)

      const fullResult = await loadCatalogCached(
        `${cacheKey}|full`,
        CACHE_TTL.catalog,
        () => GetResourceCatalog({ ...baseOpts, includeCounts: true }),
        { force },
      )
      if (reqRef.current !== id) return
      setCatalog(fullResult.data)
    } catch (e) {
      if (reqRef.current !== id) return
      setError(String(e))
      if (!peekCatalogCache(`${cacheKey}|fast`)) setCatalog(null)
    } finally {
      if (reqRef.current === id) {
        setLoading(false)
        setEnriching(false)
      }
    }
  }, [cacheKey, ctx, kubeconfig, apiParams.namespace, apiParams.allNamespaces, apiParams.namespaces, enabled])

  useEffect(() => {
    if (!enabled || !cacheKey) {
      if (!enabled) {
        setLoading(false)
        setEnriching(false)
      } else {
        setCatalog(null)
        setLoading(false)
        setEnriching(false)
        setError('')
      }
      return undefined
    }

    const cached = peekCatalogCache(`${cacheKey}|full`) || peekCatalogCache(`${cacheKey}|fast`)
    if (cached) setCatalog(cached)
    setLoading(!cached)
    reload({ force: false })

    return () => {
      reqRef.current += 1
    }
  }, [cacheKey, reload, cluster?.syncedAt, enabled])

  return { catalog, loading, enriching, error, refresh: () => reload({ force: true }) }
}
