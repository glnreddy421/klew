import { useEffect, useMemo, useState } from 'react'
import { KindIcon } from '../KindIcon.jsx'
import { LoadingBar } from '../LoadingSpinner.jsx'
import { useScopeBrowse } from '../../context/ScopeBrowseContext.jsx'
import { prefetchResourceManifest } from '../../lib/manifestCache.js'
import { buildManifestTarget } from '../../lib/manifestTarget.js'
import { resolvePodLogsTarget } from '../../lib/podLogsTerminal.js'
import { useResourcesWorkbench } from '../../views/ResourcesWorkbenchView.jsx'
import { InspectorHeaderActions } from './InspectorHeaderActions.jsx'
import { useInvestigable } from '../../context/InvestigableContext.jsx'
import { rowInvestigateState } from '../../lib/investigable.js'
import { inspectScope } from '../../lib/inspectScope.js'
import { canStreamKind, streamLogsTitle } from '../../lib/logStream.js'
import { useLogStreamActions } from '../../context/LogStreamContext.jsx'
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
  onInvestigate,
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
  const scope = inspectScope({
    kind: inspectRow?.kind,
    rowNamespace: inspectRow?.namespace,
    detailNamespace: inspect?.ref?.namespace,
    fallbackNamespace: cluster?.selectedNamespace,
    tree: browse?.tree,
    kindGroup,
  })
  const statusLabel = inspect?.status?.label
    || (inspectRow?.status === 'healthy' ? 'Ready' : inspectRow?.signal)
    || null

  const investigable = useInvestigable()
  const investigate = rowInvestigateState(investigable?.index, inspectRow, kindGroup, { loaded: investigable?.loaded !== false })
  const manifestTarget = useMemo(
    () => buildManifestTarget(inspectRow, kindGroup),
    [inspectRow, kindGroup],
  )
  const manifestEnabled = Boolean(manifestTarget)
  const podStreamContainers = useMemo(() => {
    const t = resolvePodLogsTarget(inspectRow, workbench?.view)
    return t?.containers || []
  }, [inspectRow, workbench?.view])

  useEffect(() => {
    setManifestOpen(false)
  }, [inspectRow?.key])

  const logStreamActions = useLogStreamActions()
  const streamResource = useMemo(() => {
    if (!inspectRow?.kind || !inspectRow?.name || !scope.namespace || !canStreamKind(inspectRow.kind)) return null
    return { kind: inspectRow.kind, name: inspectRow.name, namespace: scope.namespace }
  }, [inspectRow?.kind, inspectRow?.name, scope.namespace])

  // ⌘⇧L streams the inspected resource when no table rows are picked.
  useEffect(() => {
    logStreamActions?.setSelection('inspector', streamResource ? [streamResource] : [])
  }, [logStreamActions, streamResource])

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
                  {scope.label ? (
                    <>
                      <span className="inspect-meta-sep">·</span>
                      <span className={scope.namespaced ? 'mono' : undefined}>{scope.label}</span>
                    </>
                  ) : null}
                  {statusLabel && !detailLoading ? (
                    <>
                      <span className="inspect-meta-sep">·</span>
                      <span className={`inspector-header-status tone-${tone}`} title={statusLabel}>{statusLabel}</span>
                    </>
                  ) : null}
                </p>
              </div>
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
          podStreamContainers={podStreamContainers}
          investigateEnabled={Boolean(onInvestigate && inspectRow?.kind && inspectRow?.name && investigate.ok)}
          investigateTitle={investigate.title}
          onInvestigate={() => onInvestigate?.(inspectRow)}
          streamLogsEnabled={Boolean(streamResource)}
          streamLogsTitle={streamResource ? streamLogsTitle(streamResource.kind, streamResource.name) : ''}
          onStreamLogs={
            logStreamActions && streamResource
              ? (container) => (
                streamResource.kind === 'Pod' && container
                  ? logStreamActions.streamPodLogs({ ...streamResource, container })
                  : logStreamActions.streamResources([streamResource])
              )
              : undefined
          }
        />
      </header>
      {detailLoading ? (
        <LoadingBar className="inspector-loading-bar" label="Loading details" />
      ) : null}
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
