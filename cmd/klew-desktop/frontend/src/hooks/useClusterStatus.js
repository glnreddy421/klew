import { useCallback, useEffect, useState } from 'react'
import { GetClusterStatus } from '../../wailsjs/go/main/App'

export function useClusterStatus(cluster) {
  const [clusterStatus, setClusterStatus] = useState(null)

  const contextName = cluster?.selectedContext || cluster?.currentContext || ''

  const refresh = useCallback(async () => {
    if (!contextName) {
      setClusterStatus(null)
      return
    }
    try {
      setClusterStatus(await GetClusterStatus())
    } catch {
      setClusterStatus(null)
    }
  }, [contextName])

  useEffect(() => {
    refresh()
  }, [refresh, cluster?.syncedAt])

  return { clusterStatus, refreshClusterStatus: refresh }
}
