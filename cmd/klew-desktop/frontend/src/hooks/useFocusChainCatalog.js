import { useEffect, useMemo, useRef, useState } from 'react'
import { ListCatalogEntities } from '../../wailsjs/go/main/App'
import { catalogEntityToRow } from '../lib/resourceCatalog.js'
import { browseScopeApiParams, normalizeBrowseScope } from '../lib/browseScope.js'
import { focusChainKindGroups } from '../lib/focusScope.js'
import { normalizeCatalogAccessState } from '../lib/rbacAccess.js'

function scopeReadyForList(kindGroup, api) {
  if (kindGroup?.namespaced === false) return true
  return api.allNamespaces || Boolean(api.namespace) || api.namespaces.length > 0
}

/**
 * Loads catalog entities for the kinds needed to build a focus chain when no
 * investigation snapshot is available (Resources browse mode).
 */
export function useFocusChainCatalog({
  cluster,
  catalog,
  browseScope,
  focusRow,
  enabled = false,
}) {
  const [rows, setRows] = useState([])
  const [loading, setLoading] = useState(false)
  const reqRef = useRef(0)

  const kindGroups = useMemo(
    () => focusChainKindGroups(catalog),
    [catalog],
  )

  const loadableKey = useMemo(
    () => kindGroups.map((kg) => kg.resourceId).sort().join('|'),
    [kindGroups],
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

  const focusNs = focusRow?.namespace || focusRow?.ref?.namespace || ''

  useEffect(() => {
    if (!enabled || !focusRow || !ctx || !kindGroups.length) {
      setRows([])
      setLoading(false)
      return undefined
    }

    const id = ++reqRef.current
    setLoading(true)

    const scope = normalizeBrowseScope(browseScope ?? cluster?.browseScope)
    const api = browseScopeApiParams(scope)

    const fetches = kindGroups.map((kindGroup) => {
      if (!scopeReadyForList(kindGroup, api)) {
        return Promise.resolve({ rows: [] })
      }
      return ListCatalogEntities({
        resourceId: kindGroup.resourceId,
        namespace: focusNs || api.namespace,
        allNamespaces: focusNs ? false : api.allNamespaces,
        namespaces: focusNs ? [] : api.namespaces,
        clusterScoped: kindGroup.namespaced === false,
        kubeconfig,
        context: ctx,
      })
        .then((result) => ({
          rows: (result?.entities || []).map((entity) => catalogEntityToRow(entity, kindGroup.kind)),
          accessState: normalizeCatalogAccessState(result?.accessState, result?.error || ''),
        }))
        .catch(() => ({ rows: [] }))
    })

    Promise.all(fetches)
      .then((results) => {
        if (reqRef.current !== id) return
        const byKey = new Map()
        for (const result of results) {
          for (const row of result.rows || []) {
            if (row?.key) byKey.set(row.key, row)
          }
        }
        setRows([...byKey.values()])
      })
      .finally(() => {
        if (reqRef.current === id) setLoading(false)
      })

    return () => {
      reqRef.current += 1
    }
  }, [enabled, focusRow?.key, focusNs, ctx, kubeconfig, loadableKey, kindGroups, browseScope, cluster?.browseScope])

  return { rows, loading }
}
