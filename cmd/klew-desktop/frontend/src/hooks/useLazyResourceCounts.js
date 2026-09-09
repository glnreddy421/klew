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
    if (lazy.accessState === 'forbidden' || lazy.accessState === 'unavailable' || lazy.accessState === 'error') {
      setLazyCounts((prev) => {
        const existing = prev[kindGroup.resourceId]
        const accessState = lazy.accessState === 'error' ? 'unavailable' : lazy.accessState
        if (existing?.accessState === accessState && existing?.count === 0) return prev
        return {
          ...prev,
          [kindGroup.resourceId]: { count: 0, accessState },
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
  const kubeconfig = cluster?.kubeconfigPath || ''
  const scope = cluster?.scope ?? cluster?.browseScope
  let nsKey = cluster?.selectedNamespace || ''
  if (scope?.mode === 'all') nsKey = '*'
  else if (scope?.mode === 'multi') nsKey = (scope.namespaces || []).join(',')
  else if (scope?.namespace) nsKey = scope.namespace
  return `${kubeconfig}|${ctx}|${nsKey}`
}
