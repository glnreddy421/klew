import { useCallback, useEffect, useRef, useState } from 'react'
import { GetClusterStatus } from '../../wailsjs/go/main/App'

const STATUS_POLL_MS = 45000

export function useClusterStatus(cluster) {
  const [clusterStatus, setClusterStatus] = useState(null)
  const [statusLoading, setStatusLoading] = useState(false)
  const reqRef = useRef(0)

  const contextName = cluster?.selectedContext || cluster?.currentContext || ''

  const refresh = useCallback(async ({ background = false } = {}) => {
    if (!contextName) {
      setClusterStatus(null)
      setStatusLoading(false)
      return
    }

    const id = ++reqRef.current
    if (!background) setStatusLoading(true)
    try {
      const status = await GetClusterStatus()
      if (reqRef.current !== id) return
      setClusterStatus(status)
    } catch (err) {
      if (reqRef.current !== id) return
      setClusterStatus({
        available: false,
        apiReachable: false,
        error: String(err?.message || err || 'Could not reach cluster API'),
      })
    } finally {
      if (reqRef.current === id && !background) setStatusLoading(false)
    }
  }, [contextName])

  useEffect(() => {
    refresh({ background: false })
  }, [refresh, cluster?.syncedAt])

  useEffect(() => {
    if (!contextName) return undefined
    const id = window.setInterval(() => refresh({ background: true }), STATUS_POLL_MS)
    return () => window.clearInterval(id)
  }, [contextName, refresh])

  return { clusterStatus, statusLoading, refreshClusterStatus: refresh }
}
