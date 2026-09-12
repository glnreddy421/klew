import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { deriveConnectionState, isClusterDisconnected } from '../lib/clusterConnection.js'

const MAX_AUTO_RETRIES = 5
const RETRY_DELAY_SEC = 10

export { MAX_AUTO_RETRIES, RETRY_DELAY_SEC }

export function useClusterConnection({
  cluster,
  syncing,
  connecting,
  connectingTarget,
  clusterStatus,
  statusLoading,
  syncNow,
  refreshClusterStatus,
}) {
  const [retryAttempt, setRetryAttempt] = useState(0)
  const [retryInSec, setRetryInSec] = useState(0)
  const [autoRetryExhausted, setAutoRetryExhausted] = useState(false)
  const [reconnectBusy, setReconnectBusy] = useState(false)
  const [dismissed, setDismissed] = useState(false)

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

  const dismiss = useCallback(() => {
    clearRetryTimers()
    setDismissed(true)
    setRetryInSec(0)
  }, [clearRetryTimers])

  const handleReconnect = useCallback(async () => {
    setDismissed(false)
    setRetryAttempt(0)
    setAutoRetryExhausted(false)
    await reconnect()
  }, [reconnect])

  useEffect(() => {
    setRetryAttempt(0)
    setRetryInSec(0)
    setAutoRetryExhausted(false)
    setDismissed(false)
    clearRetryTimers()
  }, [contextKey, clearRetryTimers])

  useEffect(() => {
    if (!disconnected) {
      setRetryAttempt(0)
      setRetryInSec(0)
      setAutoRetryExhausted(false)
      setDismissed(false)
      clearRetryTimers()
    }
  }, [disconnected, clearRetryTimers])

  useEffect(() => {
    clearRetryTimers()

    if (
      dismissed
      || !disconnected
      || reconnectBusy
      || syncing
      || connecting
      || statusLoading
    ) {
      return undefined
    }

    if (retryAttempt >= MAX_AUTO_RETRIES) {
      setAutoRetryExhausted(true)
      setRetryInSec(0)
      return undefined
    }

    const delay = RETRY_DELAY_SEC
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
    dismissed,
  ])

  useEffect(() => () => clearRetryTimers(), [clearRetryTimers])

  const connection = deriveConnectionState({
    cluster,
    syncing: syncing || reconnectBusy,
    connecting,
    connectingTarget,
    clusterStatus,
    statusLoading,
    retryAttempt: disconnected ? retryAttempt : 0,
    retryInSec: disconnected ? retryInSec : 0,
    autoRetryExhausted,
    maxRetries: MAX_AUTO_RETRIES,
    dismissed,
  })

  return {
    connection,
    reconnect: handleReconnect,
    dismiss,
    reconnectBusy: reconnectBusy || syncing,
  }
}
