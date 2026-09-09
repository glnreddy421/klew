import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { deriveConnectionState, isClusterDisconnected } from '../lib/clusterConnection.js'

const MAX_AUTO_RETRIES = 5
const RETRY_DELAYS_SEC = [3, 5, 8, 12, 15]

export { MAX_AUTO_RETRIES }

export function useClusterConnection({
  cluster,
  syncing,
  connecting,
  clusterStatus,
  statusLoading,
  syncNow,
  refreshClusterStatus,
}) {
  const [retryAttempt, setRetryAttempt] = useState(0)
  const [retryInSec, setRetryInSec] = useState(0)
  const [autoRetryExhausted, setAutoRetryExhausted] = useState(false)
  const [reconnectBusy, setReconnectBusy] = useState(false)

  const retryTimerRef = useRef(null)
  const countdownRef = useRef(null)
  const reconnectingRef = useRef(false)

  const contextKey = cluster?.selectedContext || cluster?.currentContext || ''

  const disconnected = useMemo(() => {
    if (syncing || connecting) return false
    if (String(cluster?.syncError || '').trim()) return true
    if (statusLoading) return false
    return isClusterDisconnected(cluster, clusterStatus)
  }, [cluster, clusterStatus, syncing, connecting, statusLoading])

  const clearRetryTimers = useCallback(() => {
    if (retryTimerRef.current != null) {
      clearTimeout(retryTimerRef.current)
      retryTimerRef.current = null
    }
    if (countdownRef.current != null) {
      clearInterval(countdownRef.current)
      countdownRef.current = null
    }
  }, [])

  const reconnect = useCallback(async () => {
    if (reconnectingRef.current) return
    reconnectingRef.current = true
    clearRetryTimers()
    setReconnectBusy(true)
    setRetryInSec(0)
    try {
      await syncNow()
      await refreshClusterStatus?.()
    } finally {
      reconnectingRef.current = false
      setReconnectBusy(false)
    }
  }, [clearRetryTimers, syncNow, refreshClusterStatus])

  const handleReconnect = useCallback(async () => {
    setRetryAttempt(0)
    setAutoRetryExhausted(false)
    await reconnect()
  }, [reconnect])

  useEffect(() => {
    setRetryAttempt(0)
    setRetryInSec(0)
    setAutoRetryExhausted(false)
    clearRetryTimers()
  }, [contextKey, clearRetryTimers])

  useEffect(() => {
    if (!disconnected) {
      setRetryAttempt(0)
      setRetryInSec(0)
      setAutoRetryExhausted(false)
      clearRetryTimers()
    }
  }, [disconnected, clearRetryTimers])

  useEffect(() => {
    clearRetryTimers()

    if (!disconnected || reconnectBusy || syncing || connecting || statusLoading) {
      return undefined
    }

    if (retryAttempt >= MAX_AUTO_RETRIES) {
      setAutoRetryExhausted(true)
      setRetryInSec(0)
      return undefined
    }

    const delay = RETRY_DELAYS_SEC[retryAttempt] ?? RETRY_DELAYS_SEC[RETRY_DELAYS_SEC.length - 1]
    setRetryInSec(delay)

    countdownRef.current = window.setInterval(() => {
      setRetryInSec((sec) => (sec > 0 ? sec - 1 : 0))
    }, 1000)

    retryTimerRef.current = window.setTimeout(async () => {
      setRetryAttempt((n) => n + 1)
      await reconnect()
    }, delay * 1000)

    return clearRetryTimers
  }, [
    disconnected,
    retryAttempt,
    reconnectBusy,
    syncing,
    connecting,
    statusLoading,
    reconnect,
    clearRetryTimers,
  ])

  useEffect(() => () => clearRetryTimers(), [clearRetryTimers])

  const connection = deriveConnectionState({
    cluster,
    syncing: syncing || reconnectBusy,
    connecting,
    clusterStatus,
    statusLoading,
    retryAttempt: disconnected ? retryAttempt : 0,
    retryInSec: disconnected ? retryInSec : 0,
    autoRetryExhausted,
    maxRetries: MAX_AUTO_RETRIES,
  })

  return {
    connection,
    reconnect: handleReconnect,
    reconnectBusy: reconnectBusy || syncing,
  }
}
