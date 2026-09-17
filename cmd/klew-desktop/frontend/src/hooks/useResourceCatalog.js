import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { GetResourceCatalog } from '../../wailsjs/go/main/App'
import { browseScopeApiParams, normalizeBrowseScope } from '../lib/browseScope.js'
import { mergeCatalogCounts, peekMergedCatalogFromCache } from '../lib/resourceCatalog.js'
import { workloadCatalogResourceIds } from '../lib/resourcePresentation.js'
import {
  CACHE_TTL,
  catalogIndexCacheKey,
  loadCatalogCached,
  peekCatalogCache,
} from '../lib/catalogCache.js'

/**
 * Fetches discovery-driven resource catalog for the active cluster scope.
 * Shows cached index immediately; loads discovery fast, then workload counts.
 * Other kind counts fill in when the user opens that kind (lazy entity list).
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

  const [catalog, setCatalog] = useState(() => peekMergedCatalogFromCache(cacheKey))
  const [loading, setLoading] = useState(() => (
    Boolean(cacheKey && !peekCatalogCache(`${cacheKey}|fast`))
  ))
  const [enriching, setEnriching] = useState(false)
  const [error, setError] = useState('')
  const reqRef = useRef(0)

  const applyCachedCatalog = useCallback((key) => {
    const merged = peekMergedCatalogFromCache(key)
    if (merged) {
      setCatalog(merged)
      setLoading(false)
    }
    return merged
  }, [])

  const reload = useCallback(async ({ force = false } = {}) => {
    if (!enabled || !cacheKey) {
      setCatalog(null)
      setLoading(false)
      setEnriching(false)
      setError('')
      return
    }

    const id = ++reqRef.current
    const hasFast = Boolean(peekCatalogCache(`${cacheKey}|fast`))
    applyCachedCatalog(cacheKey)
    if (!hasFast) setLoading(true)
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
      let merged = fastResult.data
      setCatalog(merged)
      setLoading(false)
      setEnriching(true)

      const workloadIds = workloadCatalogResourceIds()
      const countsResult = await loadCatalogCached(
        `${cacheKey}|counts-workloads`,
        CACHE_TTL.catalog,
        () => GetResourceCatalog({
          ...baseOpts,
          includeCounts: true,
          countResourceIds: workloadIds,
        }),
        { force },
      )
      if (reqRef.current !== id) return
      merged = mergeCatalogCounts(merged, countsResult.data)
      setCatalog(merged)
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
  }, [
    cacheKey,
    ctx,
    kubeconfig,
    apiParams.namespace,
    apiParams.allNamespaces,
    apiParams.namespaces,
    enabled,
    applyCachedCatalog,
  ])

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

    const cached = applyCachedCatalog(cacheKey)
    setLoading(!peekCatalogCache(`${cacheKey}|fast`))
    reload({ force: false })

    return () => {
      reqRef.current += 1
    }
  }, [cacheKey, cluster?.syncedAt, reload, enabled, applyCachedCatalog])

  return { catalog, loading, enriching, error, refresh: () => reload({ force: true }) }
}
