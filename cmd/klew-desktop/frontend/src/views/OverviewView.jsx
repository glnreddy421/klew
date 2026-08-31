import { useMemo, useRef, useState } from 'react'
import { CollectingMatchesSplash } from '../components/incident/CollectingMatchesSplash'
import { InvestigationHeader } from '../components/overview/InvestigationHeader'
import { InvestigationChain } from '../components/overview/InvestigationChain'
import { KeyFindings } from '../components/overview/KeyFindings'
import { InvestigationTimeline } from '../components/overview/InvestigationTimeline'
import { AffectedResourcePath, AffectedResourcesList } from '../components/overview/AffectedResourcePath'
import { EvidencePreview } from '../components/overview/EvidencePreview'
import { ClusterContextBar } from '../components/overview/ClusterContextBar'
import { buildClusterContext } from '../lib/clusterContext'
import { deriveMatchRows, getMatchedObjects } from '../lib/matches'
import { buildInvestigationOverview } from '../lib/investigationOverview'

/**
 * Overview — Investigation Brief with visual correlation graphics.
 */
export function OverviewView({
  view,
  cluster,
  clusterStatus,
  running = false,
  syncing = false,
  collecting,
  inspectRow,
  onNavigate,
  onOpenEvidence,
  onOpenSettings,
  onInspectKeyChange,
  timeWindowLabel = 'Last 15m',
  live = false,
}) {
  const allMatches = getMatchedObjects(view)
  const allRows = useMemo(() => deriveMatchRows(view, allMatches), [view, allMatches])
  const overview = useMemo(
    () => buildInvestigationOverview(view, { rows: allRows, timeWindowLabel, live }),
    [view, allRows, timeWindowLabel, live],
  )

  const [highlightedNodeIds, setHighlightedNodeIds] = useState(() => new Set())
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

  const handleResourceSelect = (resource) => {
    if (!resource?.key) return
    onInspectKeyChange?.(resource.key)
    onNavigate?.('resources')
  }

  const handleChainNodeSelect = (node) => {
    if (node?.navTab) onNavigate?.(node.navTab)
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

  if (overview.phase === 'empty') {
    return (
      <div className="workbench-surface overview-brief">
        <ClusterContextBar
          context={clusterContext}
          onNavigate={onNavigate}
          onChipAction={handleChipAction}
        />
        <div className="overview-empty-state">
          <h2 className="overview-empty-title">Start investigating</h2>
          <p className="overview-empty-lead muted">
            Klew correlates Kubernetes resources, events, logs, signals, failures and
            relationships to help explain what happened.
          </p>
          <div className="overview-empty-actions">
            <button type="button" className="btn btn-outline btn-sm" onClick={() => onNavigate?.('resources')}>
              Browse resources
            </button>
          </div>
        </div>
      </div>
    )
  }

  const {
    verdict,
    stats,
    findings,
    visualChain,
    timelineVisual,
    affectedResources,
    resourcePathVisual,
    evidencePreview,
    visibilityWarning,
    recentObservations,
    nextPaths,
    phase,
    windowLabel,
  } = overview

  const showChainEmpty = phase === 'active' && !visualChain

  return (
    <div className="workbench-surface overview-brief overview-visual">
      <ClusterContextBar
        context={clusterContext}
        onNavigate={onNavigate}
        onChipAction={handleChipAction}
      />
      <InvestigationHeader
        verdict={verdict}
        stats={stats}
        timeWindowLabel={timeWindowLabel}
        visibilityWarning={visibilityWarning}
        onNavigate={onNavigate}
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

      {phase === 'quiet' && (
        <section className="inv-visual-section inv-quiet-banner">
          <h2 className="inv-section-title">No significant failure chain detected</h2>
          <p className="muted">
            Klew is correlating signals, Kubernetes events, resource state and evidence
            in the current window.
          </p>
          <p className="inv-quiet-meta muted">
            {live && <span className="brief-live">● Live</span>}
            {live && ' · '}
            {windowLabel || timeWindowLabel}
          </p>
        </section>
      )}

      {phase === 'quiet' && recentObservations.length > 0 && (
        <section className="inv-visual-section inv-quiet-activity">
          <h2 className="inv-section-title">Recent observations</h2>
          <ul className="brief-activity-list">
            {recentObservations.map((obs, i) => (
              <li key={`${obs.text}-${i}`}>
                {obs.timestamp && (
                  <span className="mono muted brief-activity-time">
                    {new Date(obs.timestamp).toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', hour12: false })}
                  </span>
                )}
                <span>{obs.text}</span>
              </li>
            ))}
          </ul>
        </section>
      )}

      {showChainEmpty && (
        <section className="inv-visual-section inv-chain-empty">
          <h2 className="inv-section-title">What Klew connected</h2>
          <p className="muted">
            Not enough correlated observations yet to draw a connection chain.
            Klew continues correlating signals, events, and evidence in the current window.
          </p>
        </section>
      )}

      {visualChain && (
        <div className="inv-chain-row">
          <InvestigationChain
            chain={visualChain}
            highlightedNodeIds={highlightedNodeIds}
            onNodeSelect={handleChainNodeSelect}
            onNavigate={onNavigate}
          />
          {resourcePathVisual && resourcePathVisual.length >= 2 && (
            <AffectedResourcePath
              path={resourcePathVisual}
              onNavigate={onNavigate}
              onResourceSelect={handleResourceSelect}
              compact
            />
          )}
        </div>
      )}

      <div className="inv-visual-grid">
        <div className="inv-visual-main">
          {findings.length > 0 && (
            <KeyFindings
              findings={findings}
              onNavigate={onNavigate}
              onHighlightNodes={(ids) => setHighlightedNodeIds(new Set(ids))}
            />
          )}

          <InvestigationTimeline
            timelineVisual={timelineVisual}
            onNavigate={onNavigate}
          />

          <EvidencePreview
            items={evidencePreview}
            totalCount={stats.evidence}
            onNavigate={onNavigate}
            onOpenEvidence={onOpenEvidence}
          />
        </div>

        <aside className="inv-visual-aside">
          {!visualChain && resourcePathVisual && resourcePathVisual.length >= 2 && (
            <AffectedResourcePath
              path={resourcePathVisual}
              onNavigate={onNavigate}
              onResourceSelect={handleResourceSelect}
            />
          )}

          <AffectedResourcesList
            resources={affectedResources}
            onNavigate={onNavigate}
            onResourceSelect={handleResourceSelect}
          />
        </aside>
      </div>

      {nextPaths.length > 0 && phase === 'active' && (
        <footer className="brief-next-paths">
          <span className="inv-section-title">Next</span>
          <div className="brief-next-links">
            {nextPaths.map((p) => (
              <button
                key={p.tab}
                type="button"
                className="brief-next-link"
                onClick={() => onNavigate?.(p.tab)}
              >
                {p.label} →
              </button>
            ))}
          </div>
        </footer>
      )}

      {inspectRow && (
        <p className="brief-context-note muted">
          Active context: {inspectRow.kind}/{inspectRow.name}
        </p>
      )}
    </div>
  )
}
