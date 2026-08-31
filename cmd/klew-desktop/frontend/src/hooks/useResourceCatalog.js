import { useEffect, useRef, useState } from 'react'
import { GetResourceCatalog } from '../../wailsjs/go/main/App'

/**
 * Fetches discovery-driven resource catalog for the active cluster scope.
 * Cancels stale responses on context/namespace change.
 */
export function useResourceCatalog(cluster) {
  const [catalog, setCatalog] = useState(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const reqRef = useRef(0)

  const ctx = cluster?.selectedContext || cluster?.currentContext || ''
  const ns = cluster?.selectedNamespace || ''
  const kubeconfig = cluster?.kubeconfigPath || ''

  useEffect(() => {
    if (!ctx) {
      setCatalog(null)
      setLoading(false)
      setError('')
      return undefined
    }

    const id = ++reqRef.current
    setLoading(true)
    setError('')

    GetResourceCatalog({
      context: ctx,
      namespace: ns,
      kubeconfig,
      includeCounts: true,
      refresh: false,
    })
      .then((c) => {
        if (reqRef.current !== id) return
        setCatalog(c)
      })
      .catch((e) => {
        if (reqRef.current !== id) return
        setError(String(e))
        setCatalog(null)
      })
      .finally(() => {
        if (reqRef.current === id) setLoading(false)
      })

    return () => {
      reqRef.current += 1
    }
  }, [ctx, ns, kubeconfig])

  return { catalog, loading, error }
}
