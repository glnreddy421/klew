import { useEffect, useRef, useState } from 'react'
import { ListCatalogEntities } from '../../wailsjs/go/main/App'
import { catalogEntityToRow } from '../lib/resourceCatalog.js'
import { browseScopeApiParams, normalizeBrowseScope } from '../lib/browseScope.js'

/**
 * Lazily lists catalog entities for the selected resource GVR.
 */
export function useCatalogEntities({ cluster, kindGroup, browseScope, enabled = true }) {
  const [entities, setEntities] = useState([])
  const [loading, setLoading] = useState(false)
  const [accessState, setAccessState] = useState('unknown')
  const [error, setError] = useState('')
  const reqRef = useRef(0)

  const resourceId = kindGroup?.resourceId
  const ctx = cluster?.selectedContext || cluster?.currentContext || ''
  const kubeconfig = cluster?.kubeconfigPath || ''
  const scope = normalizeBrowseScope(browseScope ?? cluster?.browseScope)
  const api = browseScopeApiParams(scope)
  const nsKey = api.allNamespaces
    ? '*'
    : (api.namespaces.length ? api.namespaces.join(',') : api.namespace)

  useEffect(() => {
    if (!enabled || !resourceId || !ctx) {
      setEntities([])
      setLoading(false)
      setAccessState('unknown')
      setError('')
      return undefined
    }
    if (!api.allNamespaces && !api.namespace && api.namespaces.length === 0 && kindGroup?.namespaced !== false) {
      setEntities([])
      setLoading(false)
      setAccessState('unknown')
      setError('')
      return undefined
    }

    const id = ++reqRef.current
    setEntities([])
    setLoading(true)
    setError('')
    setAccessState('unknown')

    ListCatalogEntities({
      resourceId,
      namespace: api.namespace,
      allNamespaces: api.allNamespaces,
      namespaces: api.namespaces,
      clusterScoped: kindGroup?.namespaced === false,
      kubeconfig,
      context: ctx,
    })
      .then((result) => {
        if (reqRef.current !== id) return
        setAccessState(result?.accessState || 'unknown')
        const rows = (result?.entities || []).map((entity) => catalogEntityToRow(entity, kindGroup?.kind))
        setEntities(rows)
        if (result?.error) setError(result.error)
      })
      .catch((e) => {
        if (reqRef.current !== id) return
        setError(String(e))
        setEntities([])
        setAccessState('error')
      })
      .finally(() => {
        if (reqRef.current === id) setLoading(false)
      })

    return () => {
      reqRef.current += 1
    }
  }, [enabled, resourceId, ctx, nsKey, kubeconfig, kindGroup?.namespaced])

  return { entities, loading, accessState, error }
}
