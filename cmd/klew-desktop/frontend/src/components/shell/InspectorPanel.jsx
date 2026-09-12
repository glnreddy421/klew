import { useEffect, useMemo, useState } from 'react'
import { useScopeBrowse } from '../../context/ScopeBrowseContext.jsx'
import { prefetchResourceManifest } from '../../lib/manifestCache.js'
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

  useEffect(() => {
    if (!manifestTarget) return undefined
    prefetchResourceManifest(manifestTarget, cluster || workbench?.cluster)
    return undefined
  }, [manifestTarget, cluster, workbench?.cluster])

  return (
    <>
      <header className="inspector-header">
        <div className="inspector-header-brand">
          <span className="inspector-accent-led" aria-hidden="true" />
          <h2 className="inspector-header-title">
            {manifestOpen ? 'Manifest' : 'Inspector'}
          </h2>
        </div>
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
        {manifestTarget ? (
          <div className="inspector-pane inspector-pane-manifest" hidden={!manifestOpen}>
            <ResourceManifestView
              target={manifestTarget}
              cluster={cluster || workbench?.cluster}
              onClose={() => setManifestOpen(false)}
            />
          </div>
        ) : null}
        <div
          className="inspector-pane inspector-pane-details"
          hidden={Boolean(manifestOpen && manifestTarget)}
        >
          {children}
        </div>
      </div>
    </>
  )
}
