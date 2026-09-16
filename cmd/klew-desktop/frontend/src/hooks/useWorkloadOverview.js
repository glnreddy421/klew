import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  ListCatalogEntities,
  StartCatalogEntityWatch,
  StopCatalogEntityWatch,
} from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { catalogEntityToRow } from '../lib/resourceCatalog.js'
import { browseScopeApiParams, normalizeBrowseScope } from '../lib/browseScope.js'
import { canLoadCatalogEntities } from '../lib/catalogDisplay.js'
import { buildWorkloadKindCardsFromRows } from '../lib/workloadOverview.js'
import { normalizeCatalogAccessState } from '../lib/rbacAccess.js'
import {
  loadCatalogCached,
  overviewCacheKey,
  overviewPollMsForScope,
  overviewTtlForScope,
  peekCatalogCache,
  writeCatalogCache,
} from '../lib/catalogCache.js'
import { useVisiblePanelRefresh } from './useVisiblePanelRefresh.js'

function scopeReadyForList(kindGroup, api) {
  if (kindGroup?.namespaced === false) return true
  return api.allNamespaces || Boolean(api.namespace) || api.namespaces.length > 0
}

function emptyOverviewPayload() {
  return {
    entitiesByResourceId: {},
    accessStateByResourceId: {},
    error: '',
  }
}

async function fetchOverviewPayload({ loadable, api, kubeconfig, ctx }) {
  const fetches = loadable.map((kindGroup) => {
    if (!scopeReadyForList(kindGroup, api)) {
      return Promise.resolve({
        resourceId: kindGroup.resourceId,
        rows: [],
        accessState: 'unknown',
      })
    }
    return ListCatalogEntities({
      resourceId: kindGroup.resourceId,
      namespace: api.namespace,
      allNamespaces: api.allNamespaces,
      namespaces: api.namespaces,
      clusterScoped: kindGroup.namespaced === false,
      kubeconfig,
      context: ctx,
    })
      .then((result) => {
        const errMsg = result?.error || ''
        return {
          resourceId: kindGroup.resourceId,
          kind: kindGroup.kind,
          rows: (result?.entities || []).map((entity) => catalogEntityToRow(entity, kindGroup.kind)),
          accessState: normalizeCatalogAccessState(result?.accessState, errMsg),
          error: errMsg,
        }
      })
      .catch((e) => {
        const errMsg = String(e)
        return {
          resourceId: kindGroup.resourceId,
          kind: kindGroup.kind,
          rows: [],
          accessState: normalizeCatalogAccessState('error', errMsg),
          error: errMsg,
        }
      })
  })

  const results = await Promise.all(fetches)
  const entitiesByResourceId = {}
  const accessStateByResourceId = {}
  const errors = []
  for (const result of results) {
    entitiesByResourceId[result.resourceId] = result.rows
    accessStateByResourceId[result.resourceId] = result.accessState
    if (result.error && result.accessState !== 'forbidden') errors.push(result.error)
  }
  return {
    entitiesByResourceId,
    accessStateByResourceId,
    error: errors[0] || '',
  }
}

/**
 * Workloads overview — near real-time in single-namespace scope (live pod watch + 8s poll).
 * Cluster-wide node counts stay on the slower cluster-status hook.
 */
export function useWorkloadOverview({ cluster, browseScope, kindGroups, enabled = true }) {
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

  const loadable = useMemo(
    () => (kindGroups || []).filter((kg) => canLoadCatalogEntities(kg)),
    [kindGroups],
  )

  const loadableKey = useMemo(() => (
    loadable.map((kg) => kg.resourceId).sort().join('|')
  ), [loadable])

  const loadableResourceIds = useMemo(
    () => new Set(loadable.map((kg) => kg.resourceId)),
    [loadable],
  )

  const cacheKey = useMemo(() => {
    if (!ctx || !loadableKey) return ''
    return overviewCacheKey({ ctx, kubeconfig, nsKey, loadableKey })
  }, [ctx, kubeconfig, nsKey, loadableKey])

  const podGroup = useMemo(
    () => loadable.find((kg) => kg.kind === 'Pod'),
    [loadable],
  )

  const watchMode = useMemo(
    () => !apiParams.allNamespaces && apiParams.namespaces.length <= 1 && Boolean(podGroup),
    [apiParams.allNamespaces, apiParams.namespaces.length, podGroup],
  )

  const podWatchKey = useMemo(
    () => (cacheKey && podGroup ? `${cacheKey}|pods-live` : ''),
    [cacheKey, podGroup],
  )

  const kindGroupsRef = useRef(kindGroups)
  kindGroupsRef.current = kindGroups

  const initial = peekCatalogCache(cacheKey) || emptyOverviewPayload()
  const [entitiesByResourceId, setEntitiesByResourceId] = useState(initial.entitiesByResourceId)
  const [accessStateByResourceId, setAccessStateByResourceId] = useState(initial.accessStateByResourceId)
  const [error, setError] = useState(initial.error || '')
  const [live, setLive] = useState(false)
  const overviewTtl = useMemo(() => overviewTtlForScope(apiParams), [apiParams])
  const overviewPollMs = useMemo(() => overviewPollMsForScope(apiParams), [apiParams])

  const [loading, setLoading] = useState(Boolean(enabled && cacheKey && !peekCatalogCache(cacheKey)))
  const [refreshing, setRefreshing] = useState(false)
  const [updatedAt, setUpdatedAt] = useState(() => peekCatalogCache(cacheKey)?.updatedAt || 0)
  const reqRef = useRef(0)

  const mergeResourceRows = useCallback((resourceId, rows, accessState, at = Date.now()) => {
    setEntitiesByResourceId((prev) => {
      const next = { ...prev, [resourceId]: rows }
      setAccessStateByResourceId((accessPrev) => {
        const accessNext = { ...accessPrev, [resourceId]: accessState }
        if (cacheKey) {
          writeCatalogCache(cacheKey, {
            entitiesByResourceId: next,
            accessStateByResourceId: accessNext,
            error: '',
            updatedAt: at,
          })
        }
        return accessNext
      })
      return next
    })
    setUpdatedAt(at)
  }, [cacheKey])

  const applyPayload = useCallback((payload, at = Date.now()) => {
    setEntitiesByResourceId(payload.entitiesByResourceId || {})
    setAccessStateByResourceId(payload.accessStateByResourceId || {})
    setError(payload.error || '')
    setUpdatedAt(at)
    if (cacheKey) {
      writeCatalogCache(cacheKey, { ...payload, updatedAt: at })
    }
  }, [cacheKey])

  const reload = useCallback(async ({ force = false } = {}) => {
    const loadableNow = (kindGroupsRef.current || []).filter((kg) => canLoadCatalogEntities(kg))
    if (!enabled || !loadableNow.length || !cacheKey) {
      applyPayload(emptyOverviewPayload())
      setLoading(false)
      setRefreshing(false)
      setLive(false)
      return
    }

    const hasCached = Boolean(peekCatalogCache(cacheKey))
    const id = ++reqRef.current

    if (!hasCached) setLoading(true)
    else setRefreshing(true)

    try {
      const result = await loadCatalogCached(
        cacheKey,
        overviewTtl,
        () => fetchOverviewPayload({
          loadable: loadableNow,
          api: apiParams,
          kubeconfig,
          ctx,
        }),
        { force },
      )
      if (reqRef.current !== id) return
      applyPayload(result.data, Date.now())
    } catch (e) {
      if (reqRef.current !== id) return
      setError(String(e))
    } finally {
      if (reqRef.current === id) {
        setLoading(false)
        setRefreshing(false)
      }
    }
  }, [
    enabled,
    cacheKey,
    ctx,
    kubeconfig,
    apiParams,
    overviewTtl,
    applyPayload,
  ])

  useVisiblePanelRefresh(
    () => reload({ force: true }),
    overviewPollMs,
    enabled && Boolean(cacheKey),
  )

  useEffect(() => {
    if (!enabled || !cacheKey) {
      applyPayload(emptyOverviewPayload())
      setLoading(false)
      setRefreshing(false)
      setLive(false)
      return undefined
    }

    applyPayload(peekCatalogCache(cacheKey) || emptyOverviewPayload())
    setLoading(!peekCatalogCache(cacheKey))
    reload({ force: true })

    return () => {
      reqRef.current += 1
    }
  }, [enabled, cacheKey, applyPayload, reload])

  useEffect(() => {
    if (!enabled || !watchMode || !podGroup || !podWatchKey) {
      return undefined
    }

    let active = true

    const listOpts = {
      resourceId: podGroup.resourceId,
      namespace: apiParams.namespace,
      allNamespaces: apiParams.allNamespaces,
      namespaces: apiParams.namespaces,
      clusterScoped: podGroup.namespaced === false,
      kubeconfig,
      context: ctx,
      watchKey: podWatchKey,
    }

    function applyWatchPayload(payload) {
      if (!active || payload?.watchKey !== podWatchKey) return
      if (!loadableResourceIds.has(payload.resourceId)) return
      const rows = (payload.entities || []).map((entity) => catalogEntityToRow(entity, 'Pod'))
      mergeResourceRows(
        payload.resourceId,
        rows,
        normalizeCatalogAccessState(payload.accessState, payload.error || ''),
        payload.updatedAt || Date.now(),
      )
      setLive(Boolean(payload.live))
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
    watchMode,
    podGroup,
    podWatchKey,
    apiParams,
    kubeconfig,
    ctx,
    loadableResourceIds,
    mergeResourceRows,
  ])

  const cards = useMemo(
    () => buildWorkloadKindCardsFromRows(kindGroups, entitiesByResourceId, accessStateByResourceId),
    [kindGroups, entitiesByResourceId, accessStateByResourceId],
  )

  const podEntities = useMemo(() => {
    if (!podGroup?.resourceId) return []
    return entitiesByResourceId[podGroup.resourceId] || []
  }, [podGroup, entitiesByResourceId])

  const deniedCount = useMemo(
    () => Object.values(accessStateByResourceId).filter((s) => s === 'forbidden').length,
    [accessStateByResourceId],
  )

  const hasData = useMemo(
    () => Object.keys(entitiesByResourceId).length > 0,
    [entitiesByResourceId],
  )

  return {
    cards,
    podEntities,
    loading,
    refreshing,
    live,
    error,
    deniedCount,
    hasData,
    updatedAt,
    refresh: () => reload({ force: true }),
  }
}
