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

  const toggleRail = useCallback(() => {
    patch({ railCollapsed: !layout.railCollapsed })
  }, [layout.railCollapsed, patch])

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
    toggleRail,
    setExplorerWidth,
    setInspectorWidth,
    setInspectorPlacement,
    patch,
  }
}
