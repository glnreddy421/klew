import { useCallback, useEffect, useRef, useState } from 'react'
import { GetClusterStatus } from '../../wailsjs/go/main/App'
import {
  CACHE_TTL,
  clusterStatusCacheKey,
  loadCatalogCached,
  peekCatalogCache,
} from '../lib/catalogCache.js'

const STATUS_POLL_MS = 45000

export function useClusterStatus(cluster) {
  const contextName = cluster?.selectedContext || cluster?.currentContext || ''
  const cacheKey = contextName ? clusterStatusCacheKey(contextName) : ''

  const [clusterStatus, setClusterStatus] = useState(() => (
    cacheKey ? peekCatalogCache(cacheKey) : null
  ))
  const [statusLoading, setStatusLoading] = useState(Boolean(cacheKey && !peekCatalogCache(cacheKey)))
  const [refreshing, setRefreshing] = useState(false)
  const reqRef = useRef(0)

  const refresh = useCallback(async ({ background = false, force = false } = {}) => {
    if (!contextName || !cacheKey) {
      setClusterStatus(null)
      setStatusLoading(false)
      setRefreshing(false)
      return
    }

    const hasCached = Boolean(peekCatalogCache(cacheKey))
    const id = ++reqRef.current
    const bg = background || (hasCached && !force)

    if (!bg) setStatusLoading(true)
    else setRefreshing(true)

    try {
      const result = await loadCatalogCached(
        cacheKey,
        CACHE_TTL.clusterStatus,
        async () => {
          const status = await GetClusterStatus()
          if (status?.apiReachable === false) {
            throw new Error(status.error || 'Could not reach cluster API')
          }
          return status
        },
        { force },
      )
      if (reqRef.current !== id) return
      setClusterStatus(result.data)
    } finally {
      if (reqRef.current === id) {
        setStatusLoading(false)
        setRefreshing(false)
      }
    }
  }, [contextName, cacheKey])

  useEffect(() => {
    if (!cacheKey) {
      setClusterStatus(null)
      setStatusLoading(false)
      setRefreshing(false)
      return undefined
    }

    const cached = peekCatalogCache(cacheKey)
    if (cached) setClusterStatus(cached)
    setStatusLoading(!cached)
    refresh({ background: Boolean(cached), force: false })

    return () => {
      reqRef.current += 1
    }
  }, [cacheKey, refresh, cluster?.syncedAt])

  useEffect(() => {
    if (!contextName) return undefined
    const id = window.setInterval(() => refresh({ background: true }), STATUS_POLL_MS)
    return () => window.clearInterval(id)
  }, [contextName, refresh])

  return { clusterStatus, statusLoading, refreshing, refreshClusterStatus: refresh }
}
