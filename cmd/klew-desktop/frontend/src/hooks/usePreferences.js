import { useCallback, useState } from 'react'
import { loadPreferences, savePreferences } from '../lib/preferences'

export function usePreferences() {
  const [prefs, setPrefsState] = useState(() => loadPreferences())

  const setPreferences = useCallback((patch) => {
    setPrefsState((prev) => {
      const next = typeof patch === 'function' ? patch(prev) : { ...prev, ...patch }
      return savePreferences(next)
    })
  }, [])

  const resetPreferences = useCallback(() => {
    const next = savePreferences({})
    setPrefsState(next)
    return next
  }, [])

  return { prefs, setPreferences, resetPreferences }
}
