import { useCallback, useEffect, useRef, useState } from 'react'
import { loadColumnWidths, saveColumnWidths } from '../lib/entityTableColumnWidths.js'

/**
 * Per-kind manual column widths (empty object = automatic layout).
 *
 * `previewWidths` updates state only (use on every drag frame); `commitWidths`
 * persists the current widths once (drag end). `setWidths` does both.
 */
export function useEntityTableColumnWidths(kind) {
  const [state, setState] = useState(() => ({ kind, widths: loadColumnWidths(kind) }))

  useEffect(() => {
    if (state.kind !== kind) setState({ kind, widths: loadColumnWidths(kind) })
  }, [kind, state.kind])

  const widths = state.kind === kind ? state.widths : {}
  const widthsRef = useRef(widths)
  widthsRef.current = widths

  /** `fn(prevWidths) → nextWidths`, state only. */
  const previewWidths = useCallback((fn) => {
    const next = fn(widthsRef.current)
    widthsRef.current = next
    setState({ kind, widths: next })
  }, [kind])

  const commitWidths = useCallback(() => {
    saveColumnWidths(kind, widthsRef.current)
  }, [kind])

  const setWidths = useCallback((fn) => {
    previewWidths(fn)
    commitWidths()
  }, [commitWidths, previewWidths])

  const resetWidths = useCallback(() => setWidths(() => ({})), [setWidths])

  return { widths, previewWidths, commitWidths, setWidths, resetWidths }
}
