import { useCallback, useEffect, useState } from 'react'
import { applyTheme, readStoredTheme, resolveThemeId, subscribeTheme } from '../lib/themes'

/**
 * `themeId` is the stored preference (light / dark / system); `resolvedTheme` is
 * the palette in effect, which follows macOS while the preference is system.
 */
export function useTheme() {
  const [themeId, setThemeId] = useState(() => readStoredTheme())
  const [resolvedTheme, setResolvedTheme] = useState(() => resolveThemeId(readStoredTheme()))

  useEffect(() => subscribeTheme((resolved, pref) => {
    setResolvedTheme(resolved)
    setThemeId(pref)
  }), [])

  const setTheme = useCallback((pref) => {
    applyTheme(pref)
  }, [])

  return { themeId, resolvedTheme, setTheme }
}
