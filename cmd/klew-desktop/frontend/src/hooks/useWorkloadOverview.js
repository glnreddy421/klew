import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { ListCatalogEntities } from '../../wailsjs/go/main/App'
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

async function fetchOverviewPayload({ loadable, scope, api, kubeconfig, ctx }) {
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
          rows: (result?.entities || []).map(catalogEntityToRow),
          accessState: normalizeCatalogAccessState(result?.accessState, errMsg),
          error: errMsg,
        }
      })
      .catch((e) => {
        const errMsg = String(e)
        return {
          resourceId: kindGroup.resourceId,
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
 * Loads workload catalog entities for overview donuts — independent of nav selection
 * and investigation snapshot rows. Cached 5 minutes with stale-while-revalidate.
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

  const loadableKey = useMemo(() => (
    (kindGroups || [])
      .filter((kg) => canLoadCatalogEntities(kg))
      .map((kg) => kg.resourceId)
      .sort()
      .join('|')
  ), [kindGroups])

  const cacheKey = useMemo(() => {
    if (!ctx || !loadableKey) return ''
    return overviewCacheKey({ ctx, kubeconfig, nsKey, loadableKey })
  }, [ctx, kubeconfig, nsKey, loadableKey])

  const kindGroupsRef = useRef(kindGroups)
  kindGroupsRef.current = kindGroups

  const initial = peekCatalogCache(cacheKey) || emptyOverviewPayload()
  const [entitiesByResourceId, setEntitiesByResourceId] = useState(initial.entitiesByResourceId)
  const [accessStateByResourceId, setAccessStateByResourceId] = useState(initial.accessStateByResourceId)
  const [error, setError] = useState(initial.error || '')
  const overviewTtl = useMemo(() => overviewTtlForScope(apiParams), [apiParams])
  const overviewPollMs = useMemo(() => overviewPollMsForScope(apiParams), [apiParams])

  const [loading, setLoading] = useState(Boolean(enabled && cacheKey && !peekCatalogCache(cacheKey)))
  const [refreshing, setRefreshing] = useState(false)
  const [updatedAt, setUpdatedAt] = useState(() => peekCatalogCache(cacheKey)?.updatedAt || 0)
  const reqRef = useRef(0)

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
    const loadable = (kindGroupsRef.current || []).filter((kg) => canLoadCatalogEntities(kg))
    if (!enabled || !loadable.length || !cacheKey) {
      applyPayload(emptyOverviewPayload())
      setLoading(false)
      setRefreshing(false)
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
          loadable,
          scope,
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
    apiParams.namespace,
    apiParams.allNamespaces,
    apiParams.namespaces,
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
      return undefined
    }

    const cached = peekCatalogCache(cacheKey)
    if (cached) applyPayload(cached)
    else applyPayload(emptyOverviewPayload())
    setLoading(!cached)
    reload({ force: false })

    return () => {
      reqRef.current += 1
    }
  }, [enabled, cacheKey, applyPayload, reload])

  const cards = useMemo(
    () => buildWorkloadKindCardsFromRows(kindGroups, entitiesByResourceId, accessStateByResourceId),
    [kindGroups, entitiesByResourceId, accessStateByResourceId],
  )

  const podEntities = useMemo(() => {
    const podGroup = (kindGroups || []).find((kg) => kg.kind === 'Pod')
    if (!podGroup?.resourceId) return []
    return entitiesByResourceId[podGroup.resourceId] || []
  }, [kindGroups, entitiesByResourceId])

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
    error,
    deniedCount,
    hasData,
    updatedAt,
    refresh: () => reload({ force: true }),
  }
}
