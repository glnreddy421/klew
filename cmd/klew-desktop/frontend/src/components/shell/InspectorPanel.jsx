import { useEffect, useMemo, useState } from 'react'
import { useScopeBrowse } from '../../context/ScopeBrowseContext.jsx'
import { buildManifestTarget } from '../../lib/manifestTarget.js'
import { resolvePodLogsTarget, podLogsTerminalEnabled } from '../../lib/podLogsTerminal.js'
import { useResourcesWorkbench } from '../../views/ResourcesWorkbenchView.jsx'
import { InspectorHeaderActions } from './InspectorHeaderActions.jsx'
import { ResourceManifestView } from './ResourceManifestView.jsx'

/**
 * Inspector chrome + body with optional read-only kubectl manifest view.
 */
export function InspectorPanel({
  placement = 'right',
  onPlacementChange,
  onClose,
  onOpenPodLogs,
  children,
  cluster,
}) {
  const workbench = useResourcesWorkbench()
  const browse = useScopeBrowse()
  const [manifestOpen, setManifestOpen] = useState(false)

  const inspectRow = workbench?.inspectRow
  const kindGroup = browse?.nav?.selectedKindGroup || browse?.effectiveKindGroup

  const manifestTarget = useMemo(
    () => buildManifestTarget(inspectRow, kindGroup),
    [inspectRow, kindGroup],
  )
  const manifestEnabled = Boolean(manifestTarget)
  const podLogsTarget = useMemo(
    () => resolvePodLogsTarget(inspectRow, workbench?.view),
    [inspectRow, workbench?.view],
  )
  const podLogsEnabled = podLogsTerminalEnabled(inspectRow)

  useEffect(() => {
    setManifestOpen(false)
  }, [inspectRow?.key])

  return (
    <>
      <header className="inspector-header">
        <h2 className="inspector-header-title">
          {manifestOpen ? 'Manifest' : 'Inspector'}
        </h2>
        <InspectorHeaderActions
          placement={placement}
          onPlacementChange={onPlacementChange}
          onClose={onClose}
          manifestOpen={manifestOpen}
          manifestEnabled={manifestEnabled}
          onToggleManifest={() => setManifestOpen((v) => !v)}
          podLogsEnabled={podLogsEnabled}
          podLogsContainers={podLogsTarget?.containers || []}
          onOpenPodLogs={({ container } = {}) => {
            if (!podLogsTarget) return
            onOpenPodLogs?.({ ...podLogsTarget, container: container || '' })
          }}
        />
      </header>
      <div className="inspector-body">
        {manifestOpen && manifestTarget ? (
          <ResourceManifestView
            target={manifestTarget}
            cluster={cluster || workbench?.cluster}
            onClose={() => setManifestOpen(false)}
          />
        ) : (
          children
        )}
      </div>
    </>
  )
}
