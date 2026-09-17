import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { ListCatalogEntities, StartCatalogEntityWatch, StopCatalogEntityWatch } from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { catalogEntityToRow } from '../lib/resourceCatalog.js'
import { browseScopeApiParams, normalizeBrowseScope } from '../lib/browseScope.js'
import { normalizeCatalogAccessState } from '../lib/rbacAccess.js'
import {
  entitiesTtlForScope,
  entitiesListCacheKey,
  loadCatalogCached,
  peekCatalogCache,
  peekCatalogCacheEntry,
  writeCatalogCache,
} from '../lib/catalogCache.js'

function emptyEntityPayload() {
  return { entities: [], accessState: 'unknown', error: '' }
}

function mapListResult(listResult, kind) {
  const errMsg = listResult?.error || ''
  return {
    entities: (listResult?.entities || []).map((entity) => catalogEntityToRow(entity, kind)),
    accessState: normalizeCatalogAccessState(listResult?.accessState, errMsg),
    error: errMsg,
  }
}

function scopeReady(apiParams, clusterScoped) {
  if (clusterScoped) return true
  return apiParams.allNamespaces || Boolean(apiParams.namespace) || apiParams.namespaces.length > 0
}

/**
 * Lazily lists catalog entities for the selected resource GVR.
 * Single-namespace scopes stream via Kubernetes watch; wide scopes poll while visible.
 */
export function useCatalogEntities({ cluster, kindGroup, browseScope, enabled = true }) {
  const resourceId = kindGroup?.resourceId
  const kind = kindGroup?.kind
  const ctx = cluster?.selectedContext || cluster?.currentContext || ''
  const kubeconfig = cluster?.kubeconfigPath || ''
  const scope = useMemo(
    () => normalizeBrowseScope(browseScope ?? cluster?.browseScope),
    [browseScope, cluster?.browseScope],
  )
  const apiParams = useMemo(() => browseScopeApiParams(scope), [scope])
  const nsKey = useMemo(() => (
    apiParams.allNamespaces
      ? '*'
      : (apiParams.namespaces.length ? apiParams.namespaces.join(',') : apiParams.namespace)
  ), [apiParams.allNamespaces, apiParams.namespace, apiParams.namespaces])
  const entityTtl = useMemo(() => entitiesTtlForScope(apiParams), [apiParams])
  const watchMode = useMemo(
    () => !apiParams.allNamespaces && apiParams.namespaces.length <= 1,
    [apiParams.allNamespaces, apiParams.namespaces.length],
  )

  const cacheKey = useMemo(() => {
    if (!resourceId || !ctx) return ''
    return entitiesListCacheKey({
      resourceId,
      ctx,
      kubeconfig,
      nsKey,
      clusterScoped: kindGroup?.namespaced === false,
    })
  }, [resourceId, ctx, kubeconfig, nsKey, kindGroup?.namespaced])

  const listOpts = useMemo(() => ({
    resourceId,
    namespace: apiParams.namespace,
    allNamespaces: apiParams.allNamespaces,
    namespaces: apiParams.namespaces,
    clusterScoped: kindGroup?.namespaced === false,
    kubeconfig,
    context: ctx,
    watchKey: cacheKey,
  }), [
    resourceId,
    apiParams.namespace,
    apiParams.allNamespaces,
    apiParams.namespaces,
    kindGroup?.namespaced,
    kubeconfig,
    ctx,
    cacheKey,
  ])

  const initial = peekCatalogCache(cacheKey) || emptyEntityPayload()
  const [entities, setEntities] = useState(initial.entities || [])
  const [accessState, setAccessState] = useState(initial.accessState || 'unknown')
  const [error, setError] = useState(initial.error || '')
  const [loading, setLoading] = useState(Boolean(enabled && cacheKey && !peekCatalogCache(cacheKey)))
  const [refreshing, setRefreshing] = useState(false)
  const [updatedAt, setUpdatedAt] = useState(() => peekCatalogCache(cacheKey)?.updatedAt || 0)
  const [live, setLive] = useState(false)
  const reqRef = useRef(0)

  const applyPayload = useCallback((payload, { at = Date.now(), streaming = false } = {}) => {
    setEntities(payload.entities || [])
    setAccessState(payload.accessState || 'unknown')
    setError(payload.error || '')
    setUpdatedAt(at)
    setLive(streaming)
    if (cacheKey) {
      writeCatalogCache(cacheKey, { ...payload, updatedAt: at })
    }
  }, [cacheKey])

  const reload = useCallback(async ({ force = false } = {}) => {
    if (!enabled || !resourceId || !ctx || !cacheKey) {
      applyPayload(emptyEntityPayload(), { at: 0, streaming: false })
      setLoading(false)
      setRefreshing(false)
      setLive(false)
      return
    }
    if (!scopeReady(apiParams, kindGroup?.namespaced === false)) {
      applyPayload(emptyEntityPayload(), { at: 0, streaming: false })
      setLoading(false)
      setRefreshing(false)
      setLive(false)
      return
    }

    const cachedEntry = peekCatalogCacheEntry(cacheKey)
    const hasCached = Boolean(cachedEntry?.data)
    const id = ++reqRef.current
    if (hasCached) {
      applyPayload(cachedEntry.data, { at: cachedEntry.at || Date.now(), streaming: false })
      setLoading(false)
      setRefreshing(true)
    } else {
      setLoading(true)
    }

    try {
      const result = await loadCatalogCached(
        cacheKey,
        entityTtl,
        async () => {
          const listResult = await ListCatalogEntities(listOpts)
          return mapListResult(listResult, kind)
        },
        { force },
      )
      if (reqRef.current !== id) return
      applyPayload(result.data, { at: Date.now(), streaming: false })
    } catch (e) {
      if (reqRef.current !== id) return
      const errMsg = String(e)
      setError(errMsg)
      if (!hasCached) {
        setEntities([])
        setAccessState(normalizeCatalogAccessState('error', errMsg))
      }
      setLive(false)
    } finally {
      if (reqRef.current === id) {
        setLoading(false)
        setRefreshing(false)
      }
    }
  }, [
    enabled,
    resourceId,
    ctx,
    cacheKey,
    apiParams,
    kindGroup?.namespaced,
    kind,
    kubeconfig,
    entityTtl,
    listOpts,
    applyPayload,
  ])

  useEffect(() => {
    if (!enabled || !cacheKey) {
      applyPayload(emptyEntityPayload(), { at: 0, streaming: false })
      setLoading(false)
      setRefreshing(false)
      setLive(false)
      return undefined
    }

    const cachedEntry = peekCatalogCacheEntry(cacheKey)
    if (cachedEntry?.data) {
      applyPayload(cachedEntry.data, { at: cachedEntry.at || 0, streaming: false })
      setLoading(false)
    } else {
      applyPayload(emptyEntityPayload(), { at: 0, streaming: false })
      setLoading(true)
    }
    reload({ force: false })

    return () => {
      reqRef.current += 1
    }
  }, [enabled, cacheKey, applyPayload, reload, cluster?.syncedAt])

  useEffect(() => {
    if (!enabled || !cacheKey || !scopeReady(apiParams, kindGroup?.namespaced === false)) {
      return undefined
    }

    let active = true

    function applyWatchPayload(payload) {
      if (!active || payload?.watchKey !== cacheKey) return
      applyPayload(
        mapListResult(payload, kind),
        { at: payload.updatedAt || Date.now(), streaming: Boolean(payload.live) },
      )
      setLoading(false)
      setRefreshing(false)
    }

    function syncWatch() {
      if (!active || document.visibilityState !== 'visible') {
        StopCatalogEntityWatch().catch(() => {})
        setLive(false)
        return
      }
      StartCatalogEntityWatch(listOpts).catch(() => {})
    }

    const off = EventsOn('catalog:entities', applyWatchPayload)
    syncWatch()
    document.addEventListener('visibilitychange', syncWatch)

    return () => {
      active = false
      document.removeEventListener('visibilitychange', syncWatch)
      StopCatalogEntityWatch().catch(() => {})
      if (typeof off === 'function') off()
      setLive(false)
    }
  }, [
    enabled,
    cacheKey,
    kind,
    listOpts,
    apiParams,
    kindGroup?.namespaced,
    applyPayload,
  ])

  return {
    entities,
    loading,
    refreshing,
    accessState,
    error,
    updatedAt,
    live: live && watchMode,
    watchMode,
    refresh: () => reload({ force: true }),
  }
}
