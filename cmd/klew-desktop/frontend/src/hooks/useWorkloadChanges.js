import { useEffect, useState } from 'react'
import {
  changesCacheKey,
  entryFreshness,
  fetchPodRevision,
  fetchWorkloadChanges,
  peekChanges,
} from '../lib/changesCache'

function useCachedFetch(key, enabled, generation, load) {
  const [state, setState] = useState(() => ({ data: peekChanges(key)?.data || null, loading: false, error: '' }))

  useEffect(() => {
    if (!enabled || !key) {
      setState({ data: null, loading: false, error: '' })
      return undefined
    }
    let cancelled = false
    const entry = peekChanges(key)
    const freshness = entryFreshness(entry, { generation })
    // Stale-while-revalidate: show what we have; an invalid entry (spec changed) is not shown.
    setState({
      data: freshness === 'fresh' || freshness === 'stale' ? entry.data : null,
      loading: freshness !== 'fresh',
      error: '',
    })
    if (freshness === 'fresh') return undefined
    load()
      .then((data) => { if (!cancelled) setState({ data, loading: false, error: '' }) })
      .catch((err) => {
        if (!cancelled) setState((s) => ({ ...s, loading: false, error: String(err?.message || err || 'Failed to load revision history') }))
      })
    return () => { cancelled = true }
    // load is recreated each render; key + generation identify the request.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, enabled, generation])

  return state
}

/** Revision history of a workload (lazy; cached per cluster + object + generation). */
export function useWorkloadChanges({ scope, kind, namespace, name, generation = 0, enabled = true }) {
  const key = changesCacheKey(scope, kind, namespace, name)
  return useCachedFetch(key, enabled, generation, () => fetchWorkloadChanges(key, kind, namespace, name))
}

/** Which workload revision a Pod runs. */
export function usePodRevision({ scope, namespace, name, enabled = true }) {
  const key = changesCacheKey(scope, 'Pod', namespace, name)
  return useCachedFetch(key ? `pod|${key}` : '', enabled, 0, () => fetchPodRevision(key, namespace, name))
}
