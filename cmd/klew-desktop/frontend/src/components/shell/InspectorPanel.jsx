import { useEffect, useMemo, useState } from 'react'
import { KindIcon } from '../KindIcon.jsx'
import { StatusBadge } from '../incident/StatusBadge.jsx'
import { InlineLoading } from '../LoadingSpinner.jsx'
import { useScopeBrowse } from '../../context/ScopeBrowseContext.jsx'
import { prefetchResourceManifest } from '../../lib/manifestCache.js'
import { buildManifestTarget } from '../../lib/manifestTarget.js'
import { resolvePodLogsTarget, podLogsTerminalEnabled } from '../../lib/podLogsTerminal.js'
import { useResourcesWorkbench } from '../../views/ResourcesWorkbenchView.jsx'
import { InspectorHeaderActions } from './InspectorHeaderActions.jsx'
import { ResourceManifestView } from './ResourceManifestView.jsx'

function statusTone(inspect, inspectRow) {
  const raw = inspect?.status?.tone || inspectRow?.status || 'unknown'
  return raw === 'degraded' ? 'warning' : raw
}

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
  const inspect = workbench?.inspect
  const detailLoading = workbench?.detailLoading
  const kindGroup = browse?.nav?.selectedKindGroup || browse?.effectiveKindGroup
  const tone = statusTone(inspect, inspectRow)
  const statusLabel = inspect?.status?.label
    || (inspectRow?.status === 'healthy' ? 'Ready' : inspectRow?.signal)
    || null

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
      <header className={`inspector-header ${inspectRow ? `tone-${tone}` : ''}`.trim()}>
        <div className="inspector-header-brand">
          {inspectRow ? (
            <>
              <span className={`inspector-accent-led tone-${tone}`} aria-hidden="true" />
              <span className="inspector-header-icon">
                <KindIcon kind={inspectRow.kind} size={16} />
              </span>
              <div className="inspector-header-copy">
                {manifestOpen ? (
                  <span className="inspector-header-kicker">Manifest</span>
                ) : null}
                <h2 className="inspector-header-name" title={inspectRow.name}>
                  {inspectRow.name}
                </h2>
                <p className="inspector-header-meta">
                  <span>{inspectRow.kind}</span>
                  {inspectRow.namespace ? (
                    <>
                      <span className="inspect-meta-sep">·</span>
                      <span className="mono">{inspectRow.namespace}</span>
                    </>
                  ) : (
                    <>
                      <span className="inspect-meta-sep">·</span>
                      <span>Cluster-scoped</span>
                    </>
                  )}
                </p>
              </div>
              {statusLabel && !detailLoading ? (
                <StatusBadge status={inspect?.status?.tone || inspectRow.status} label={statusLabel} />
              ) : null}
              {detailLoading ? (
                <InlineLoading message="Loading…" className="inspector-header-loading muted" />
              ) : null}
            </>
          ) : (
            <>
              <span className="inspector-accent-led" aria-hidden="true" />
              <h2 className="inspector-header-title">Inspector</h2>
            </>
          )}
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
