import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
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
import { resolveInspectRef } from '../../lib/inspectEnrich.js'
import { canStreamKind, streamLogsTitle } from '../../lib/logStream.js'
import { useLogStreamActions } from '../../context/LogStreamContext.jsx'
import { InspectorManifestProvider } from '../../context/InspectorManifestContext.jsx'
import {
  canStepInspectHistory,
  EMPTY_INSPECT_HISTORY,
  stepInspectHistory,
  visitInspectKey,
} from '../../lib/inspectHistory.js'

/** Back / forward through the objects opened in the inspector. */
function useInspectHistory(currentKey, open) {
  const [history, setHistory] = useState(EMPTY_INSPECT_HISTORY)
  const steppingRef = useRef(false)

  useEffect(() => {
    if (!currentKey) return
    // A step already moved the index; only fresh opens are recorded.
    if (steppingRef.current) {
      steppingRef.current = false
      return
    }
    setHistory((h) => visitInspectKey(h, currentKey))
  }, [currentKey])

  const step = useCallback((delta) => {
    const result = stepInspectHistory(history, delta)
    if (!result) return
    steppingRef.current = true
    setHistory(result.history)
    open?.(result.key)
  }, [history, open])

  return {
    back: () => step(-1),
    forward: () => step(1),
    canBack: canStepInspectHistory(history, -1),
    canForward: canStepInspectHistory(history, 1),
  }
}

function statusTone(inspect, inspectRow) {
  const raw = inspect?.status?.tone || inspectRow?.status || 'unknown'
  return raw === 'degraded' ? 'warning' : raw
}

/**
 * Inspector chrome + body. The read-only kubectl manifest is the details' YAML tab.
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

  const inspectRow = workbench?.inspectRow
  const nav = useInspectHistory(workbench?.inspectKey, workbench?.handleInspect)
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
  const detailNamespace = inspect?.namespace
  const namespaceRef = detailNamespace && scope.namespaced && workbench?.handleInspect
    ? resolveInspectRef(detailNamespace, { fieldKey: 'Namespace', inspectNamespace: detailNamespace })
    : null
  const statusLabel = inspect?.status?.label
    || (inspectRow?.status === 'healthy' ? 'Ready' : inspectRow?.signal)
    || null

  const investigable = useInvestigable()
  const investigate = rowInvestigateState(investigable?.index, inspectRow, kindGroup, { loaded: investigable?.loaded !== false })
  const manifestTarget = useMemo(
    () => buildManifestTarget(inspectRow, kindGroup),
    [inspectRow, kindGroup],
  )
  const manifestCluster = cluster || workbench?.cluster
  const manifestValue = useMemo(
    () => (manifestTarget ? { target: manifestTarget, cluster: manifestCluster } : null),
    [manifestTarget, manifestCluster],
  )
  const podStreamContainers = useMemo(() => {
    const t = resolvePodLogsTarget(inspectRow, workbench?.view)
    return t?.containers || []
  }, [inspectRow, workbench?.view])

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
          {(nav.canBack || nav.canForward) && (
            <div className="inspector-history" role="group" aria-label="Inspector history">
              <button
                type="button"
                className="inspector-history-btn"
                onClick={nav.back}
                disabled={!nav.canBack}
                title="Back to the previous object"
                aria-label="Back"
              >
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
                  <path d="M15 6l-6 6 6 6" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
              </button>
              <button
                type="button"
                className="inspector-history-btn"
                onClick={nav.forward}
                disabled={!nav.canForward}
                title="Forward to the next object"
                aria-label="Forward"
              >
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
                  <path d="M9 6l6 6-6 6" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
              </button>
            </div>
          )}
          {inspectRow ? (
            <>
              <span className={`inspector-accent-led tone-${tone}`} aria-hidden="true" />
              <span className="inspector-header-icon">
                <KindIcon kind={inspectRow.kind} size={16} />
              </span>
              <div className="inspector-header-copy">
                <h2 className="inspector-header-name" title={inspectRow.name}>
                  {inspectRow.name}
                </h2>
                <p className="inspector-header-meta">
                  <span>{inspectRow.kind}</span>
                  {scope.label ? (
                    <>
                      <span className="inspect-meta-sep">·</span>
                      {namespaceRef ? (
                        <button
                          type="button"
                          className="inspector-header-ns mono"
                          onClick={() => workbench.handleInspect(namespaceRef.key)}
                          title={`Open Namespace/${detailNamespace}`}
                        >
                          {scope.label}
                        </button>
                      ) : (
                        <span className={scope.namespaced ? 'mono' : undefined}>{scope.label}</span>
                      )}
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
        <div className="inspector-pane inspector-pane-details">
          <InspectorManifestProvider value={manifestValue}>
            {children}
          </InspectorManifestProvider>
        </div>
      </div>
    </>
  )
}
