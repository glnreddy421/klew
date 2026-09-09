import { useEffect, useRef, useState } from 'react'
import { GetResourceManifest } from '../../wailsjs/go/main/App'
import { manifestTargetKey } from '../lib/manifestTarget.js'

/**
 * Fetches read-only kubectl get -o yaml for the selected resource.
 */
export function useResourceManifest(target, cluster) {
  const [manifest, setManifest] = useState(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const reqRef = useRef(0)

  const ctx = cluster?.selectedContext || cluster?.currentContext || ''
  const kubeconfig = cluster?.kubeconfigPath || ''
  const targetKey = manifestTargetKey(target)

  const fetchManifest = () => {
    if (!target?.name || !ctx) return Promise.resolve()
    const id = ++reqRef.current
    setLoading(true)
    setError('')

    return GetResourceManifest({
      resourceId: target.resourceId || '',
      kind: target.kind || '',
      name: target.name || '',
      namespace: target.namespace || '',
      clusterScoped: Boolean(target.clusterScoped),
      kubeconfig,
      context: ctx,
    })
      .then((result) => {
        if (reqRef.current !== id) return
        setManifest(result)
        if (result?.error) {
          setError(result.error)
        } else {
          setError('')
        }
      })
      .catch((err) => {
        if (reqRef.current !== id) return
        setManifest(null)
        setError(String(err?.message || err || 'Failed to fetch manifest'))
      })
      .finally(() => {
        if (reqRef.current === id) setLoading(false)
      })
  }

  useEffect(() => {
    if (!targetKey || !ctx) {
      setManifest(null)
      setError('')
      setLoading(false)
      return undefined
    }
    fetchManifest()
    return () => {
      reqRef.current += 1
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [targetKey, ctx, kubeconfig])

  return { manifest, loading, error, refresh: fetchManifest }
}
