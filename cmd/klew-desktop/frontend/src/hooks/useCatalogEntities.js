import { useEffect, useRef, useState } from 'react'
import { ListCatalogEntities } from '../../wailsjs/go/main/App'
import { catalogEntityToRow } from '../lib/resourceCatalog.js'

/**
 * Lazily lists catalog entities for the selected resource GVR.
 */
export function useCatalogEntities({ cluster, kindGroup, enabled = true }) {
  const [entities, setEntities] = useState([])
  const [loading, setLoading] = useState(false)
  const [accessState, setAccessState] = useState('unknown')
  const [error, setError] = useState('')
  const reqRef = useRef(0)

  const resourceId = kindGroup?.resourceId
  const ctx = cluster?.selectedContext || cluster?.currentContext || ''
  const ns = cluster?.selectedNamespace || ''
  const kubeconfig = cluster?.kubeconfigPath || ''

  useEffect(() => {
    if (!enabled || !resourceId || !ctx) {
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
      namespace: ns,
      clusterScoped: kindGroup?.namespaced === false,
      kubeconfig,
      context: ctx,
    })
      .then((result) => {
        if (reqRef.current !== id) return
        setAccessState(result?.accessState || 'unknown')
        const rows = (result?.entities || []).map(catalogEntityToRow)
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
  }, [enabled, resourceId, ctx, ns, kubeconfig, kindGroup?.namespaced])

  return { entities, loading, accessState, error }
}
