import { useCallback, useRef, useState } from 'react'

export const HOME_NAV = {
  tab: 'incident',
  nodesFocus: 'cluster',
  settingsSection: 'general',
}

export function navEntryKey(entry) {
  if (!entry) return ''
  return [
    entry.tab || '',
    entry.nodesFocus || '',
    entry.settingsSection || '',
  ].join('|')
}

export function normalizeNavEntry(partial, current = HOME_NAV) {
  return {
    tab: partial?.tab ?? current.tab ?? HOME_NAV.tab,
    nodesFocus: partial?.nodesMode ?? partial?.nodesFocus ?? current.nodesFocus ?? HOME_NAV.nodesFocus,
    settingsSection: partial?.settingsSection ?? current.settingsSection ?? HOME_NAV.settingsSection,
  }
}

/**
 * In-app back / forward / home navigation for investigation surfaces.
 */
export function useNavigationHistory(initialEntry = HOME_NAV) {
  const stackRef = useRef([normalizeNavEntry(initialEntry)])
  const indexRef = useRef(0)
  const skipPushRef = useRef(false)
  const [revision, setRevision] = useState(0)

  const bump = useCallback(() => setRevision((n) => n + 1), [])

  const current = stackRef.current[indexRef.current]
  const canGoBack = indexRef.current > 0
  const canGoForward = indexRef.current < stackRef.current.length - 1

  const push = useCallback((entry) => {
    if (skipPushRef.current) {
      skipPushRef.current = false
      return
    }
    const normalized = normalizeNavEntry(entry, stackRef.current[indexRef.current])
    const cur = stackRef.current[indexRef.current]
    if (navEntryKey(cur) === navEntryKey(normalized)) return

    const trimmed = stackRef.current.slice(0, indexRef.current + 1)
    trimmed.push(normalized)
    stackRef.current = trimmed
    indexRef.current = trimmed.length - 1
    bump()
  }, [bump])

  const go = useCallback((delta) => {
    const next = indexRef.current + delta
    if (next < 0 || next >= stackRef.current.length) return null
    indexRef.current = next
    skipPushRef.current = true
    bump()
    return stackRef.current[next]
  }, [bump])

  const back = useCallback(() => go(-1), [go])
  const forward = useCallback(() => go(1), [go])

  return {
    current,
    push,
    back,
    forward,
    canGoBack,
    canGoForward,
    revision,
  }
}
