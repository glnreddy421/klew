import { useCallback, useState } from 'react'
import { applyTheme, readStoredTheme } from '../lib/themes'

export function useTheme() {
  const [themeId, setThemeId] = useState(() => readStoredTheme())

  const setTheme = useCallback((id) => {
    const applied = applyTheme(id)
    setThemeId(applied)
  }, [])

  return { themeId, setTheme }
}
