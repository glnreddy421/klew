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
})

export function useCluster() {
  const [cluster, setCluster] = useState(emptyCluster)
  const [syncing, setSyncing] = useState(false)

  const apply = useCallback((st) => {
    if (st) setCluster(st)
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
    apply(await SelectContext(name))
  }, [apply])

  const setNamespace = useCallback(async (name) => {
    apply(await SelectNamespace(name))
  }, [apply])

  return {
    cluster,
    syncing,
    syncNow,
    setContext,
    setNamespace,
  }
}
