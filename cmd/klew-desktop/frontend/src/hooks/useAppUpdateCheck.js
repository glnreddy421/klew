import { useEffect, useState } from 'react'
import { CheckForUpdate } from '../../wailsjs/go/main/App'
import { shouldPromptUpdate } from '../lib/appUpdate.js'

const STARTUP_DELAY_MS = 12_000

/**
 * Quietly checks GitHub after startup. Returns a result only when an update
 * should be prompted (available and not dismissed for that version).
 */
export function useAppUpdateCheck(dismissedVersion = '') {
  const [result, setResult] = useState(null)

  useEffect(() => {
    if (!CheckForUpdate) return undefined
    let cancelled = false
    const timer = setTimeout(() => {
      CheckForUpdate()
        .then((res) => {
          if (cancelled) return
          setResult(shouldPromptUpdate(res, dismissedVersion) ? res : null)
        })
        .catch(() => {
          if (!cancelled) setResult(null)
        })
    }, STARTUP_DELAY_MS)
    return () => {
      cancelled = true
      clearTimeout(timer)
    }
  }, [dismissedVersion])

  return result
}
