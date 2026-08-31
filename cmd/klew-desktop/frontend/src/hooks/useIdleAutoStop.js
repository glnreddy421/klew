import { useCallback, useEffect, useRef } from 'react'

const CHECK_MS = 60_000
const BUMP_THROTTLE_MS = 30_000

/**
 * Stops an active investigation after wall-clock idle time with no user input.
 * Incoming log lines / cluster events do not reset the timer.
 */
export function useIdleAutoStop({ enabled, idleMinutes, active, onIdle }) {
  const lastActivityRef = useRef(Date.now())
  const onIdleRef = useRef(onIdle)
  onIdleRef.current = onIdle

  const bumpActivity = useCallback(() => {
    lastActivityRef.current = Date.now()
  }, [])

  useEffect(() => {
    if (!active) {
      lastActivityRef.current = Date.now()
    }
  }, [active])

  useEffect(() => {
    if (!enabled || !active || idleMinutes <= 0) {
      return undefined
    }

    lastActivityRef.current = Date.now()
    let lastBump = 0
    const mark = () => {
      const now = Date.now()
      if (now - lastBump < BUMP_THROTTLE_MS) return
      lastBump = now
      lastActivityRef.current = now
    }

    const events = ['mousedown', 'keydown', 'click', 'scroll', 'touchstart', 'wheel']
    for (const name of events) {
      window.addEventListener(name, mark, { passive: true })
    }

    const interval = window.setInterval(() => {
      const idleMs = idleMinutes * 60 * 1000
      if (Date.now() - lastActivityRef.current < idleMs) return
      lastActivityRef.current = Date.now()
      onIdleRef.current?.()
    }, CHECK_MS)

    return () => {
      for (const name of events) {
        window.removeEventListener(name, mark)
      }
      clearInterval(interval)
    }
  }, [enabled, active, idleMinutes])

  return { bumpActivity }
}

/** Human label for settings UI, e.g. 120 → "2 hours". */
export function formatIdleDuration(minutes) {
  const m = Math.max(1, Math.round(Number(minutes) || 0))
  if (m % 60 === 0 && m >= 60) {
    const h = m / 60
    return `${h} hour${h === 1 ? '' : 's'}`
  }
  return `${m} minute${m === 1 ? '' : 's'}`
}
