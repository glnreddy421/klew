import { useCallback, useEffect, useState } from 'react'
import {
  GetCluster,
  SelectContext,
  SelectNamespace,
  SyncCluster,
} from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'

const emptyCluster = () => ({
  kubeconfigPath: '',
  currentContext: '',
  selectedContext: '',
  selectedNamespace: '',
  cluster: '',
  user: '',
  contexts: [],
  namespaces: [],
  syncedAt: null,
  syncError: '',
  syncWarning: '',
})

export function useCluster() {
  const [cluster, setCluster] = useState(emptyCluster)
  const [syncing, setSyncing] = useState(false)
  const [connecting, setConnecting] = useState(false)
  const [connectingTarget, setConnectingTarget] = useState(null)
  const [namespaceBusy, setNamespaceBusy] = useState(false)

  const apply = useCallback((st) => {
    if (!st) return
    setCluster({
      ...emptyCluster(),
      ...st,
      contexts: Array.isArray(st.contexts) ? st.contexts : [],
      namespaces: Array.isArray(st.namespaces) ? st.namespaces : [],
    })
  }, [])

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        apply(await GetCluster())
        if (cancelled) return
        apply(await SyncCluster())
      } catch {
        // cluster events will deliver state when the backend recovers
      }
    })()
    const off = EventsOn('cluster', apply)
    return () => {
      cancelled = true
      if (typeof off === 'function') off()
    }
  }, [apply])

  const syncNow = useCallback(async () => {
    setSyncing(true)
    try {
      apply(await SyncCluster())
    } finally {
      setSyncing(false)
    }
  }, [apply])

  const setContext = useCallback(async (name) => {
    setConnecting(true)
    setConnectingTarget({ kind: 'context', name })
    try {
      apply(await SelectContext(name))
    } finally {
      setConnecting(false)
      setConnectingTarget(null)
    }
  }, [apply])

  const setNamespace = useCallback(async (name) => {
    setNamespaceBusy(true)
    try {
      apply(await SelectNamespace(name))
    } finally {
      setNamespaceBusy(false)
    }
  }, [apply])

  return {
    cluster,
    syncing,
    connecting,
    connectingTarget,
    namespaceBusy,
    syncNow,
    setContext,
    setNamespace,
  }
}
