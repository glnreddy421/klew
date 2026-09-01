import { useMemo } from 'react'
import { EvidenceBoardPanel } from '../components/evidence/EvidenceBoardPanel'
import {
  formatClock,
  getState,
  groupEvidence,
  rankedVerdictSignals,
} from '../lib/investigationViews'

/**
 * Evidence — correlated pattern links and supporting observations.
 */
export function EvidenceView({ view, onFilterLogs, explorerFilter }) {
  const state = getState(view)

  const signals = useMemo(() => {
    const ranked = rankedVerdictSignals(state.verdict || {})
    return ranked.length ? ranked : (view?.signals || [])
  }, [state.verdict, view?.signals])

  const evidence = useMemo(() => {
    const raw = view?.evidence || state.liveEvidence || []
    const t = explorerFilter?.type
    if (!t) return raw
    return raw.filter((e) => {
      const st = String(e.sourceType || '').toLowerCase()
      if (t === 'log') return st === 'log'
      if (t === 'event') return st === 'k8s_event' || st === 'event'
      if (t === 'change') return st === 'object_change' || st === 'change'
      if (t === 'metric') return st === 'metric' || st === 'metrics'
      return true
    })
  }, [view?.evidence, state.liveEvidence, explorerFilter?.type])

  const groups = useMemo(() => groupEvidence(evidence), [evidence])
  const nextChecks = view?.nextChecks || state.nextChecks || state.verdict?.recommendedNextChecks || []
  const gaps = state.verdict?.missingDataWarnings || state.warnings || []
  const alts = view?.hypothesisAlternatives || state.hypothesisAlternatives || []
  const evidenceBoard = (view?.logPatterns || state.logPatterns)?.evidenceBoard || null

  const supportItems = useMemo(() => {
    const out = []
    for (const e of groups.event || []) out.push({ ...e, _bucket: 'Event' })
    for (const e of groups.change || []) out.push({ ...e, _bucket: 'Change' })
    for (const e of groups.metric || []) out.push({ ...e, _bucket: 'Metric' })
    return out.slice(0, 12)
  }, [groups])

  return (
    <div className="inv-page evidence-page ev-revamp">
      <EvidenceBoardPanel board={evidenceBoard} onFilterLogs={onFilterLogs} />

      <div className="ev-support">
        <section className="ev-panel" aria-labelledby="ev-obs-title">
          <h3 id="ev-obs-title">Observations</h3>
          {supportItems.length === 0 ? (
            <p className="muted ev-panel-empty">Events and changes appear here as they arrive.</p>
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
            <p className="muted ev-panel-empty">Scored signals appear after the first correlation pass.</p>
          ) : (
            <ul className="ev-claims">
              {signals.slice(0, 10).map((s, i) => (
                <li
                  key={s.id || `${s.label}-${i}`}
                  className={`ev-claim strength-${s.strength || 'medium'}`}
                >
                  <div className="ev-claim-head">
                    <strong>{s.label}</strong>
                    <span className="ev-claim-strength">{s.strength || 'signal'}</span>
                  </div>
                  {s.evidence && <p className="muted">{s.evidence}</p>}
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
