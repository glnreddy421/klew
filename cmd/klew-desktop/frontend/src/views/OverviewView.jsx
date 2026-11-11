import { useMemo, useRef } from 'react'
import { CollectingMatchesSplash } from '../components/incident/CollectingMatchesSplash'
import { RecordedHistoryLine } from '../components/overview/RecordedHistoryLine.jsx'
import { InvestigationTimeline } from '../components/overview/InvestigationTimeline'
import { EvidencePreview } from '../components/overview/EvidencePreview'
import { ClusterContextBar } from '../components/overview/ClusterContextBar'
import { buildClusterContext } from '../lib/clusterContext'
import { deriveMatchRows, getMatchedObjects } from '../lib/matches'
import { buildInvestigationOverview } from '../lib/investigationOverview'
import { InvestigationSurfaceGuide } from '../components/incident/InvestigationSurfaceGuide.jsx'
import { OverviewBrief } from '../components/overview/OverviewBrief.jsx'
import { buildOverviewBrief } from '../lib/overviewBrief.js'

/**
 * Overview — the investigation brief: verdict, what is failing, why, what
 * changed, warning patterns and next checks. Timeline and evidence are collapsed.
 */
export function OverviewView({
  view,
  cluster,
  clusterStatus,
  running = false,
  investigationActive = false,
  syncing = false,
  collecting,
  inspectRow,
  onNavigate,
  onOpenEvidence,
  onOpenSettings,
  onInspectKeyChange,
  timeWindowLabel = 'Last 15m',
  live = false,
  snapshotRefreshSec = 10,
}) {
  const allMatches = getMatchedObjects(view)
  const allRows = useMemo(() => deriveMatchRows(view, allMatches), [view, allMatches])
  const overview = useMemo(
    () => buildInvestigationOverview(view, {
      rows: allRows,
      timeWindowLabel,
      live,
      snapshotRefreshSec,
    }),
    [view, allRows, timeWindowLabel, live, snapshotRefreshSec],
  )

  const brief = useMemo(() => buildOverviewBrief(view), [view])
  const visibilityRef = useRef(null)

  const clusterContext = useMemo(
    () => buildClusterContext(cluster, view, { running, syncing, clusterStatus }),
    [cluster, view, running, syncing, clusterStatus],
  )

  const handleChipAction = (action) => {
    if (action === 'settings') onOpenSettings?.()
    if (action === 'visibility') {
      visibilityRef.current?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
    }
  }

  if (collecting) {
    return (
      <div className="workbench-surface overview-brief">
        <ClusterContextBar
          context={clusterContext}
          onNavigate={onNavigate}
          onChipAction={handleChipAction}
        />
        <CollectingMatchesSplash />
      </div>
    )
  }

  if (!investigationActive || overview.phase === 'empty') {
    return (
      <div className="workbench-surface overview-brief">
        <ClusterContextBar
          context={clusterContext}
          onNavigate={onNavigate}
          onChipAction={handleChipAction}
        />
        <InvestigationSurfaceGuide surfaceId="overview" onNavigate={onNavigate} />
      </div>
    )
  }

  const { stats, timelineVisual, evidencePreview, visibilityWarning, windowLabel } = overview
  const meta = (
    <>
      {live ? <span className="brief-live">● Live</span> : null}
      {live && ' · '}
      {windowLabel || timeWindowLabel}
      <RecordedHistoryLine view={view} className="ob-history" />
    </>
  )

  return (
    <div className="workbench-surface overview-brief overview-visual">
      <ClusterContextBar
        context={clusterContext}
        onNavigate={onNavigate}
        onChipAction={handleChipAction}
      />

      {visibilityWarning && (
        <div
          ref={visibilityRef}
          id="brief-visibility-warn"
          className="brief-visibility-warn"
          role="status"
        >
          <strong>{visibilityWarning.title}</strong>
          <p className="muted">{visibilityWarning.message}</p>
        </div>
      )}

      <OverviewBrief
        brief={brief}
        meta={meta}
        onInspect={(key) => {
          onInspectKeyChange?.(key)
          onNavigate?.('resources')
        }}
        onInspectKeyChange={onInspectKeyChange}
        onNavigate={onNavigate}
      >
        <details className="ob-details">
          <summary>Timeline</summary>
          <InvestigationTimeline timelineVisual={timelineVisual} onNavigate={onNavigate} />
        </details>
        <details className="ob-details">
          <summary>Evidence · {stats.evidence}</summary>
          <EvidencePreview
            items={evidencePreview}
            totalCount={stats.evidence}
            onNavigate={onNavigate}
            onOpenEvidence={onOpenEvidence}
          />
        </details>
      </OverviewBrief>

      {inspectRow && (
        <p className="brief-context-note muted">
          Active context: {inspectRow.kind}/{inspectRow.name}
        </p>
      )}
    </div>
  )
}
