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
    GetCluster().then(apply).catch(() => {})
    SyncCluster().then(apply).catch(() => {})
    return EventsOn('cluster', apply)
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
    try {
      apply(await SelectContext(name))
    } finally {
      setConnecting(false)
    }
  }, [apply])

  const setNamespace = useCallback(async (name) => {
    setConnecting(true)
    try {
      apply(await SelectNamespace(name))
    } finally {
      setConnecting(false)
    }
  }, [apply])

  return {
    cluster,
    syncing,
    connecting,
    syncNow,
    setContext,
    setNamespace,
  }
}
