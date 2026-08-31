import { useEffect, useMemo, useState } from 'react'
import { applyLazyCountsToTree } from '../lib/resourceCatalog.js'

/**
 * Caches lazy LIST results per resourceId and patches navigation tree counts.
 */
export function useLazyResourceCounts(baseTree, { clusterKey, kindGroup, lazy }) {
  const [lazyCounts, setLazyCounts] = useState({})

  useEffect(() => {
    setLazyCounts({})
  }, [clusterKey])

  useEffect(() => {
    if (!kindGroup?.resourceId || lazy.loading) return
    if (lazy.accessState === 'allowed') {
      const count = lazy.entities.length
      setLazyCounts((prev) => {
        const existing = prev[kindGroup.resourceId]
        if (existing?.count === count && existing?.accessState === 'allowed') return prev
        return { ...prev, [kindGroup.resourceId]: { count, accessState: 'allowed' } }
      })
      return
    }
    if (lazy.accessState === 'forbidden' || lazy.accessState === 'unavailable') {
      setLazyCounts((prev) => {
        const existing = prev[kindGroup.resourceId]
        if (existing?.accessState === lazy.accessState) return prev
        return {
          ...prev,
          [kindGroup.resourceId]: { count: 0, accessState: lazy.accessState },
        }
      })
    }
  }, [kindGroup?.resourceId, lazy.loading, lazy.accessState, lazy.entities.length])

  return useMemo(
    () => applyLazyCountsToTree(baseTree, lazyCounts),
    [baseTree, lazyCounts],
  )
}

export function clusterScopeKey(cluster) {
  const ctx = cluster?.selectedContext || cluster?.currentContext || ''
  const ns = cluster?.selectedNamespace || ''
  const kubeconfig = cluster?.kubeconfigPath || ''
  return `${kubeconfig}|${ctx}|${ns}`
}
