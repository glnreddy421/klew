import { useCallback, useEffect, useState } from 'react'
import {
  explorerDefaultCollapsed,
  loadShellLayout,
  saveShellLayout,
} from '../lib/shellLayout.js'

export function useShellLayout(activeTab) {
  const [layout, setLayout] = useState(() => loadShellLayout())

  useEffect(() => {
    setLayout((prev) => ({
      ...prev,
      explorerCollapsed: explorerDefaultCollapsed(activeTab),
    }))
  }, [activeTab])

  const patch = useCallback((next) => {
    setLayout((prev) => {
      const merged = { ...prev, ...next }
      saveShellLayout(merged)
      return merged
    })
  }, [])

  const toggleExplorer = useCallback(() => {
    patch({ explorerCollapsed: !layout.explorerCollapsed })
  }, [layout.explorerCollapsed, patch])

  const toggleInspector = useCallback(() => {
    patch({ inspectorCollapsed: !layout.inspectorCollapsed })
  }, [layout.inspectorCollapsed, patch])

  const setRailMode = useCallback((railMode) => {
    patch({ railMode })
  }, [patch])

  const setExplorerWidth = useCallback((w) => {
    patch({ explorerWidth: Math.round(w) })
  }, [patch])

  const setInspectorWidth = useCallback((w) => {
    patch({ inspectorWidth: Math.round(w) })
  }, [patch])

  const setInspectorPlacement = useCallback((placement) => {
    const bottom = placement === 'bottom'
    setLayout((prev) => {
      const next = {
        ...prev,
        inspectorPlacement: bottom ? 'bottom' : 'right',
        inspectorCollapsed: false,
        // A width set for one placement means nothing for the other.
        inspectorWidthPinned: false,
      }
      if (bottom && prev.inspectorWidth > 320) {
        next.inspectorWidth = 280
      }
      saveShellLayout(next)
      return next
    })
  }, [])

  return {
    layout,
    toggleExplorer,
    toggleInspector,
    setRailMode,
    setExplorerWidth,
    setInspectorWidth,
    setInspectorPlacement,
    patch,
  }
}
