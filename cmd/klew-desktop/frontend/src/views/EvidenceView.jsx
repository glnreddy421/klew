import { useMemo } from 'react'
import { StatusBadge } from '../components/incident/StatusBadge'
import { EvidenceBoardPanel } from '../components/evidence/EvidenceBoardPanel'
import {
  confidenceLabel,
  formatClock,
  getState,
  groupEvidence,
  investigationWindowLabel,
  rankedVerdictSignals,
} from '../lib/investigationViews'

/**
 * Evidence — correlated pattern links first, then supporting claims & next checks.
 */
export function EvidenceView({ view, onFilterLogs }) {
  const state = getState(view)
  const verdict = state.verdict || {}
  const healthy = String(verdict.status || '').toLowerCase() === 'healthy'
    || String(verdict.status || '').toLowerCase() === 'ok'

  const hypothesis = healthy
    ? 'No active incident'
    : (view?.hypothesis
      || state.hypothesisLabel
      || verdict.likelyTrigger
      || 'Collecting evidence…')

  const conf = verdict.confidence ?? 0
  const confTrend = view?.confidenceTrend || state.confidenceTrend || ''
  const confText = conf > 0
    ? `${confidenceLabel(conf)}${confTrend ? ` · ${confTrend}` : ''}`
    : '—'

  const signals = useMemo(() => {
    const ranked = rankedVerdictSignals(verdict)
    return ranked.length ? ranked : (view?.signals || [])
  }, [verdict, view?.signals])

  const evidence = view?.evidence || state.liveEvidence || []
  const groups = useMemo(() => groupEvidence(evidence), [evidence])
  const nextChecks = view?.nextChecks || state.nextChecks || verdict.recommendedNextChecks || []
  const gaps = verdict.missingDataWarnings || state.warnings || []
  const causal = view?.causalChain || state.causalChain || []
  const correlation = view?.correlation || state.correlation || []
  const alts = view?.hypothesisAlternatives || state.hypothesisAlternatives || []
  const patterns = view?.logPatterns || state.logPatterns || null
  const evidenceBoard = patterns?.evidenceBoard || null

  const supportItems = useMemo(() => {
    const out = []
    for (const e of groups.event || []) out.push({ ...e, _bucket: 'Event' })
    for (const e of groups.change || []) out.push({ ...e, _bucket: 'Change' })
    for (const e of groups.metric || []) out.push({ ...e, _bucket: 'Metric' })
    return out.slice(0, 10)
  }, [groups])

  const confPct = Math.min(100, Math.round((conf || 0) * 100))
  const live = state.mode === 'live' && !state.paused

  return (
    <div className="inv-page evidence-page ev-revamp">
      {/* One composition: status + hypothesis + confidence */}
      <header className="ev-verdict">
        <div className="ev-verdict-lead">
          <StatusBadge
            status={healthy ? 'healthy' : (conf >= 0.6 ? 'warning' : 'unknown')}
            label={healthy ? 'HEALTHY' : 'INVESTIGATING'}
          />
          {live && <span className="ev-live-dot" title="Live">Live</span>}
          <h2 className="ev-hypothesis">{hypothesis}</h2>
        </div>
        <div className="ev-verdict-meta">
          <div className="ev-conf-inline">
            <div className="ev-conf-track" aria-hidden="true">
              <div className="ev-conf-fill" style={{ width: `${confPct}%` }} />
            </div>
            <span className="ev-conf-label">{confText}{conf > 0 ? ` · ${confPct}%` : ''}</span>
          </div>
          <span className="muted ev-window">{investigationWindowLabel(state)}</span>
        </div>
      </header>

      <EvidenceBoardPanel board={evidenceBoard} onFilterLogs={onFilterLogs} />

      {(causal.length > 0 || correlation.length > 0) && (
        <section className="ev-causal" aria-label="Causal notes">
          {causal.length > 0 && (
            <ol className="timeline-causal compact">
              {causal.map((c, i) => <li key={`c-${i}`}>{c}</li>)}
            </ol>
          )}
          {correlation.length > 0 && (
            <ul className="corr-list">
              {correlation.map((c, i) => <li key={i}>{c}</li>)}
            </ul>
          )}
        </section>
      )}

      {/* Supporting: observations + claims */}
      <div className="ev-support">
        <section className="ev-panel" aria-labelledby="ev-obs-title">
          <h3 id="ev-obs-title">Observations</h3>
          {supportItems.length === 0 ? (
            <p className="muted ev-panel-empty">No events or changes grouped yet.</p>
          ) : (
            <ul className="ev-obs-list">
              {supportItems.map((e, i) => (
                <li key={e.id || i}>
                  <span className="mono muted">{formatClock(e.timestamp)}</span>
                  <span className="ev-obs-bucket">{e._bucket}</span>
                  <span className="ev-obs-body">
                    <strong>{e.reason || e.sourceKind || e.sourceType}</strong>
                    {' '}
                    {e.message}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </section>

        <section className="ev-panel" aria-labelledby="ev-claims-title">
          <h3 id="ev-claims-title">Claims</h3>
          {signals.length === 0 ? (
            <p className="muted ev-panel-empty">No scored signals yet.</p>
          ) : (
            <ul className="ev-claims">
              {signals.slice(0, 12).map((s, i) => (
                <li
                  key={s.id || `${s.label}-${i}`}
                  className={`ev-claim strength-${s.strength || 'medium'}`}
                >
                  <div className="ev-claim-head">
                    <strong>{s.label}</strong>
                    <span className="ev-claim-strength">{s.strength || 'signal'}</span>
                  </div>
                  {s.evidence && <p className="muted">{s.evidence}</p>}
                  {s.source && <span className="muted ev-claim-src">{s.source}</span>}
                </li>
              ))}
            </ul>
          )}
          {alts.length > 0 && (
            <ul className="ev-alts">
              {alts.map((a, i) => (
                <li key={i}>
                  <span>{a.label}</span>
                  {a.confidence != null && (
                    <span className="muted">{Math.round(a.confidence * 100)}%</span>
                  )}
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>

      {(gaps.length > 0 || nextChecks.length > 0) && (
        <section className="ev-next" aria-labelledby="ev-next-title">
          <h3 id="ev-next-title">Next</h3>
          <div className="ev-next-grid">
            {gaps.length > 0 && (
              <div>
                <h5 className="inv-section-title">Gaps</h5>
                <ul className="gaps-list">
                  {gaps.map((g, i) => <li key={i} className="muted">{g}</li>)}
                </ul>
              </div>
            )}
            {nextChecks.length > 0 && (
              <div>
                <h5 className="inv-section-title">Checks</h5>
                <ol className="next-checks">
                  {nextChecks.map((n, i) => <li key={i}>{n}</li>)}
                </ol>
              </div>
            )}
          </div>
        </section>
      )}
    </div>
  )
}
