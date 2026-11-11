import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { ListRecentWarnings } from '../../wailsjs/go/main/App'
import { browseScopeApiParams, normalizeBrowseScope } from '../lib/browseScope.js'
import { useVisiblePanelRefresh } from './useVisiblePanelRefresh.js'

const POLL_MS = 30_000

/** Newest Warning events in the browse scope (read-only list). */
export function useRecentWarnings({ cluster, browseScope, enabled = true }) {
  const ctx = cluster?.selectedContext || cluster?.currentContext || ''
  const kubeconfig = cluster?.kubeconfigPath || ''
  const api = useMemo(
    () => browseScopeApiParams(normalizeBrowseScope(browseScope ?? cluster?.browseScope)),
    [browseScope, cluster?.browseScope],
  )
  const scopeKey = `${ctx}|${kubeconfig}|${api.allNamespaces ? '*' : ''}|${api.namespace}|${api.namespaces.join(',')}`

  const [state, setState] = useState({ events: [], total: 0, accessState: 'unknown', error: '' })
  const [loading, setLoading] = useState(false)
  const reqRef = useRef(0)

  const reload = useCallback(async () => {
    if (!enabled || !ctx) return
    const id = ++reqRef.current
    setLoading(true)
    try {
      const result = await ListRecentWarnings({
        namespace: api.namespace,
        allNamespaces: api.allNamespaces,
        namespaces: api.namespaces,
        kubeconfig,
        context: ctx,
      })
      if (reqRef.current !== id) return
      setState({
        events: result?.events || [],
        total: result?.total || 0,
        accessState: result?.accessState || 'unknown',
        error: result?.error || '',
      })
    } catch (e) {
      if (reqRef.current !== id) return
      setState((prev) => ({ ...prev, accessState: 'error', error: String(e) }))
    } finally {
      if (reqRef.current === id) setLoading(false)
    }
    // scopeKey covers api, ctx and kubeconfig.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled, scopeKey])

  useEffect(() => {
    setState({ events: [], total: 0, accessState: 'unknown', error: '' })
    reload()
    return () => { reqRef.current += 1 }
  }, [reload])

  useVisiblePanelRefresh(reload, POLL_MS, enabled && Boolean(ctx))

  return { ...state, loading, refresh: reload }
}
