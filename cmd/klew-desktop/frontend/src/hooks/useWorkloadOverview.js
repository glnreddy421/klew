import { useEffect, useMemo, useRef, useState } from 'react'
import { ListCatalogEntities } from '../../wailsjs/go/main/App'
import { catalogEntityToRow } from '../lib/resourceCatalog.js'
import { browseScopeApiParams, normalizeBrowseScope } from '../lib/browseScope.js'
import { canLoadCatalogEntities } from '../lib/catalogDisplay.js'
import { buildWorkloadKindCardsFromRows } from '../lib/workloadOverview.js'
import { normalizeCatalogAccessState } from '../lib/rbacAccess.js'

function scopeReadyForList(kindGroup, api) {
  if (kindGroup?.namespaced === false) return true
  return api.allNamespaces || Boolean(api.namespace) || api.namespaces.length > 0
}

/**
 * Loads workload catalog entities for overview donuts — independent of nav selection
 * and investigation snapshot rows.
 */
export function useWorkloadOverview({ cluster, browseScope, kindGroups, enabled = true }) {
  const [entitiesByResourceId, setEntitiesByResourceId] = useState({})
  const [accessStateByResourceId, setAccessStateByResourceId] = useState({})
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const reqRef = useRef(0)

  const loadable = useMemo(
    () => (kindGroups || []).filter((kg) => canLoadCatalogEntities(kg)),
    [kindGroups],
  )

  const loadableKey = useMemo(
    () => loadable.map((kg) => kg.resourceId).sort().join('|'),
    [loadable],
  )

  const ctx = cluster?.selectedContext || cluster?.currentContext || ''
  const kubeconfig = cluster?.kubeconfigPath || ''
  const nsKey = useMemo(() => {
    const scope = normalizeBrowseScope(browseScope ?? cluster?.browseScope)
    const api = browseScopeApiParams(scope)
    if (api.allNamespaces) return '*'
    if (api.namespaces.length) return api.namespaces.join(',')
    return api.namespace
  }, [browseScope, cluster?.browseScope])

  useEffect(() => {
    if (!enabled || !loadable.length || !ctx) {
      setEntitiesByResourceId({})
      setAccessStateByResourceId({})
      setLoading(false)
      setError('')
      return undefined
    }

    const id = ++reqRef.current
    setLoading(true)
    setError('')

    const scope = normalizeBrowseScope(browseScope ?? cluster?.browseScope)
    const api = browseScopeApiParams(scope)

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

    Promise.all(fetches)
      .then((results) => {
        if (reqRef.current !== id) return
        const byId = {}
        const accessById = {}
        const errors = []
        for (const result of results) {
          byId[result.resourceId] = result.rows
          accessById[result.resourceId] = result.accessState
          if (result.error && result.accessState !== 'forbidden') errors.push(result.error)
        }
        setEntitiesByResourceId(byId)
        setAccessStateByResourceId(accessById)
        setError(errors[0] || '')
      })
      .finally(() => {
        if (reqRef.current === id) setLoading(false)
      })

    return () => {
      reqRef.current += 1
    }
  }, [enabled, loadableKey, ctx, nsKey, kubeconfig, loadable, browseScope, cluster?.browseScope])

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

  return { cards, podEntities, loading, error, deniedCount }
}
