import { useEffect, useRef } from 'react'

/**
 * Calls callback on an interval while the document tab is visible.
 * Pauses when the window/tab is hidden to avoid hammering large clusters.
 */
export function useVisiblePanelRefresh(callback, intervalMs, enabled = true) {
  const cbRef = useRef(callback)
  cbRef.current = callback

  useEffect(() => {
    if (!enabled || !intervalMs || intervalMs <= 0) return undefined

    let id = null

    function start() {
      if (id != null || document.visibilityState !== 'visible') return
      id = window.setInterval(() => {
        cbRef.current?.()
      }, intervalMs)
    }

    function stop() {
      if (id == null) return
      window.clearInterval(id)
      id = null
    }

    function onVisibility() {
      if (document.visibilityState === 'visible') {
        cbRef.current?.()
        start()
      } else {
        stop()
      }
    }

    onVisibility()
    document.addEventListener('visibilitychange', onVisibility)
    return () => {
      stop()
      document.removeEventListener('visibilitychange', onVisibility)
    }
  }, [enabled, intervalMs])
}
