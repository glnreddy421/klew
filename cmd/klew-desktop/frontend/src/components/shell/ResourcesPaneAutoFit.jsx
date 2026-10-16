import { useEffect, useLayoutEffect, useMemo } from 'react'
import { useScopeBrowse } from '../../context/ScopeBrowseContext.jsx'
import { useShellInspector } from '../../context/ShellInspectorContext.jsx'
import {
  EXPLORER_AUTO_MAX,
  explorerWidthForLabels,
  measureExplorerFitWidth,
  resourceNameColumnWidth,
} from '../../lib/paneAutoFit.js'

/**
 * Auto expand/contract shell panes for long Kubernetes names (Resources tab).
 */
export function ResourcesPaneAutoFit() {
  const shell = useShellInspector()
  const browse = useScopeBrowse()
  const entities = browse?.displayEntities || []
  const categories = browse?.nav?.categories || []
  const nameColWidth = useMemo(
    () => resourceNameColumnWidth(entities),
    [entities],
  )

  useEffect(() => {
    document.documentElement.style.setProperty('--entity-col-name-width', `${nameColWidth}px`)
    return () => {
      document.documentElement.style.removeProperty('--entity-col-name-width')
    }
  }, [nameColWidth])

  const applyExplorerAutoWidth = shell?.applyExplorerAutoWidth
  const explorerAutoFit = shell?.explorerAutoFit ?? false

  // Measure after commit, only while auto-fit applies (explorer expanded and not
  // sized by the user). Re-runs when categories change or auto-fit turns back on;
  // applyExplorerAutoWidth is stable, so drags and count refreshes don't trigger it.
  useLayoutEffect(() => {
    if (!explorerAutoFit || !categories.length) return
    const width = measureExplorerFitWidth(document, categories)
      ?? explorerWidthForLabels(categories, { max: EXPLORER_AUTO_MAX })
    applyExplorerAutoWidth?.(width)
  }, [applyExplorerAutoWidth, categories, explorerAutoFit])

  return null
}
