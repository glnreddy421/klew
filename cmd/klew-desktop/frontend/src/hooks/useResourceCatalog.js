import { useEffect, useRef, useState } from 'react'
import { GetResourceCatalog } from '../../wailsjs/go/main/App'
import { browseScopeApiParams, normalizeBrowseScope } from '../lib/browseScope.js'

/**
 * Fetches discovery-driven resource catalog for the active cluster scope.
 * Loads a fast index first (no per-kind counts), then enriches counts in the background.
 */
export function useResourceCatalog(cluster, browseScope) {
  const [catalog, setCatalog] = useState(null)
  const [loading, setLoading] = useState(false)
  const [enriching, setEnriching] = useState(false)
  const [error, setError] = useState('')
  const reqRef = useRef(0)

  const ctx = cluster?.selectedContext || cluster?.currentContext || ''
  const kubeconfig = cluster?.kubeconfigPath || ''
  const scope = normalizeBrowseScope(browseScope)
  const api = browseScopeApiParams(scope)
  const nsKey = api.allNamespaces
    ? '*'
    : (api.namespaces.length ? api.namespaces.join(',') : api.namespace)

  useEffect(() => {
    if (!ctx) {
      setCatalog(null)
      setLoading(false)
      setEnriching(false)
      setError('')
      return undefined
    }
    if (!api.allNamespaces && !api.namespace && api.namespaces.length === 0) {
      setCatalog(null)
      setLoading(false)
      setEnriching(false)
      setError('')
      return undefined
    }

    const id = ++reqRef.current
    setLoading(true)
    setEnriching(false)
    setError('')

    const baseOpts = {
      context: ctx,
      namespace: api.namespace,
      allNamespaces: api.allNamespaces,
      namespaces: api.namespaces,
      kubeconfig,
      refresh: false,
    }

    let cancelled = false

    GetResourceCatalog({ ...baseOpts, includeCounts: false })
      .then((fast) => {
        if (reqRef.current !== id) return null
        setCatalog(fast)
        setLoading(false)
        setEnriching(true)
        return GetResourceCatalog({ ...baseOpts, includeCounts: true })
      })
      .then((full) => {
        if (cancelled || reqRef.current !== id) return
        if (full) setCatalog(full)
      })
      .catch((e) => {
        if (reqRef.current !== id) return
        setError(String(e))
        setCatalog(null)
      })
      .finally(() => {
        if (reqRef.current === id) {
          setLoading(false)
          setEnriching(false)
        }
      })

    return () => {
      cancelled = true
      reqRef.current += 1
    }
  }, [ctx, nsKey, kubeconfig])

  return { catalog, loading, enriching, error }
}
