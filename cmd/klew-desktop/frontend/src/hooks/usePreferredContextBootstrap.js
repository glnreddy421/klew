import { useEffect, useMemo, useRef } from 'react'
import {
  nextPreferredContext,
  preferredContextNames,
} from '../lib/clusterContexts.js'

/**
 * On launch / Home, prefer default → first favorite → first context.
 * If the active context fails to connect, fall through to the next candidate
 * once each. Stops when connected, exhausted, or the user picks a context.
 */
export function usePreferredContextBootstrap({
  enabled = true,
  contexts = [],
  activeContext = '',
  defaultContext = '',
  favoriteContexts = [],
  disconnected = false,
  busy = false,
  userPicked = false,
  onSelectContext,
}) {
  const triedRef = useRef(new Set())
  const bootstrappedRef = useRef(false)
  const selectingRef = useRef(false)
  const contextKey = useMemo(
    () => (Array.isArray(contexts) ? contexts.map((c) => c?.name).filter(Boolean).join('\0') : ''),
    [contexts],
  )
  const favoriteKey = useMemo(
    () => (Array.isArray(favoriteContexts) ? favoriteContexts.join('\0') : ''),
    [favoriteContexts],
  )

  useEffect(() => {
    triedRef.current = new Set()
    bootstrappedRef.current = false
    selectingRef.current = false
  }, [defaultContext, favoriteKey, contextKey])

  useEffect(() => {
    if (!enabled || userPicked || busy || selectingRef.current) return
    if (typeof onSelectContext !== 'function') return
    if (!contextKey) return

    const order = preferredContextNames(contexts, { defaultContext, favoriteContexts })
    if (order.length === 0) return

    const active = String(activeContext || '').trim()
    const preferred = order[0]

    // First pass: land on the preferred context if kubeconfig current differs.
    if (!bootstrappedRef.current) {
      bootstrappedRef.current = true
      if (preferred && preferred !== active) {
        triedRef.current.add(preferred)
        selectingRef.current = true
        Promise.resolve(onSelectContext(preferred)).finally(() => {
          selectingRef.current = false
        })
      } else if (preferred) {
        triedRef.current.add(preferred)
      }
      return
    }

    // Later: if the active context is disconnected, try the next candidate.
    if (!disconnected || !active) return
    triedRef.current.add(active)
    const next = nextPreferredContext(contexts, {
      defaultContext,
      favoriteContexts,
      current: active,
      alreadyTried: [...triedRef.current],
    })
    if (!next) return
    triedRef.current.add(next)
    selectingRef.current = true
    Promise.resolve(onSelectContext(next)).finally(() => {
      selectingRef.current = false
    })
  }, [
    enabled,
    userPicked,
    busy,
    contexts,
    contextKey,
    activeContext,
    defaultContext,
    favoriteContexts,
    disconnected,
    onSelectContext,
  ])
}
