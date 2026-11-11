import { useState } from 'react'
import { buildInspectKey } from '../../lib/matches'
import { requestInspectorTab } from '../../lib/inspectorTabRequest'
import { RecordedBadge } from './RecordedHistoryLine.jsx'

const STATUS_LABEL = {
  failing: 'Failing',
  recovered: 'Recovered',
  healthy: 'Healthy',
}

const TREND_GLYPH = { rising: '↑', falling: '↓', steady: '·', new: '●' }

function since(ts, now = Date.now()) {
  if (!ts) return ''
  const ms = now - new Date(ts).getTime()
  if (!Number.isFinite(ms) || ms < 0) return ''
  const m = Math.floor(ms / 60000)
  if (m < 1) return 'just now'
  if (m < 60) return `${m}m ago`
  const h = Math.floor(m / 60)
  if (h < 48) return `${h}h ago`
  return `${Math.floor(h / 24)}d ago`
}

/** 1. The answer: status, what is wrong, likely trigger, confidence. */
function BriefVerdict({ brief, meta }) {
  return (
    <header className={`ob-verdict is-${brief.status}`}>
      <div className="ob-verdict-line">
        <span className={`ob-status is-${brief.status}`}>{STATUS_LABEL[brief.status]}</span>
        <h1 className="ob-headline">{brief.headline}</h1>
        {brief.confidenceText && (
          <span className="ob-confidence" title="How strongly the evidence supports this verdict">
            Confidence {brief.confidenceText}
          </span>
        )}
      </div>
      {brief.trigger && (
        <p className="ob-trigger">
          <span className="ob-label">Likely trigger</span>
          {brief.trigger}
        </p>
      )}
      {meta && <p className="ob-meta">{meta}</p>}
    </header>
  )
}

const HEAT_STATES = [
  { id: 'failing', label: 'Failing' },
  { id: 'degraded', label: 'Degraded' },
  { id: 'healthy', label: 'Healthy' },
  { id: 'done', label: 'Completed' },
]

/** Pod heat map: impact at a glance, one cell per pod, grouped by workload. */
function PodHeatmap({ heatmap, onInspect }) {
  if (!heatmap?.total) return null
  const { counts, total, impactedPct, groups } = heatmap
  return (
    <section className="ob-section" aria-labelledby="ob-heat-title">
      <div className="ob-section-head">
        <h2 id="ob-heat-title" className="ob-section-title">Pod impact</h2>
        <span className="ob-section-count">
          {impactedPct}% of running pods impacted · {total} pods
        </span>
      </div>
      <div className="ob-impact-bar" role="img" aria-label={`${counts.failing} failing, ${counts.degraded} degraded, ${counts.healthy} healthy, ${counts.done} completed`}>
        {HEAT_STATES.map((st) => (counts[st.id] > 0 ? (
          <span key={st.id} className={`ob-impact-seg is-${st.id}`} style={{ flexGrow: counts[st.id] }} />
        ) : null))}
      </div>
      <ul className="ob-heat-legend">
        {HEAT_STATES.map((st) => (
          <li key={st.id} className={counts[st.id] ? '' : 'is-zero'}>
            <span className={`ob-heat-swatch is-${st.id}`} aria-hidden="true" />
            {counts[st.id]} {st.label.toLowerCase()}
          </li>
        ))}
      </ul>
      <div className="ob-heat-groups">
        {groups.map((g) => (
          <div key={g.id} className="ob-heat-row">
            <span className="ob-heat-label" title={`${g.kind}/${g.name}`}>
              <span className="mono">{g.name}</span>
              <span className="ob-heat-kind">{g.kind}</span>
            </span>
            <span className="ob-heat-cells">
              {g.cells.map((c) => (
                <button
                  key={c.key}
                  type="button"
                  className={`ob-heat-cell is-${c.state}`}
                  onClick={() => onInspect?.(c.key)}
                  title={`${c.name}\n${c.reason}${c.restarts ? ` · ${c.restarts} restarts` : ''}`}
                  aria-label={`${c.name}: ${c.reason}`}
                />
              ))}
            </span>
          </div>
        ))}
      </div>
    </section>
  )
}

/** 2. What is broken right now: only unhealthy pods, with reason, exit code and restarts. */
function FailingNow({ brief, onInspect, onNavigate }) {
  if (!brief.failing.length) return null
  return (
    <section className="ob-section" aria-labelledby="ob-failing-title">
      <div className="ob-section-head">
        <h2 id="ob-failing-title" className="ob-section-title">Failing now</h2>
        <span className="ob-section-count">
          {brief.failingTotal} of {brief.podsTotal} pods
        </span>
      </div>
      <ul className="ob-failing">
        {brief.failing.map((p) => (
          <li key={p.key}>
            <button
              type="button"
              className="ob-failing-row"
              onClick={() => onInspect?.(p.key)}
              title={`Open ${p.name} in Failures`}
            >
              <span className={`ob-dot is-${p.severity}`} aria-hidden="true" />
              <span className="ob-failing-name mono">{p.name}</span>
              <span className={`ob-failing-reason is-${p.severity}`}>{p.reason}</span>
              <span className="ob-failing-detail">{p.detail}</span>
              <span className="ob-failing-restarts mono" title="Restarts">
                {p.restarts > 0 ? `↻ ${p.restarts}` : ''}
              </span>
              <span className="ob-failing-age">{since(p.createdAt)}</span>
            </button>
          </li>
        ))}
      </ul>
      {brief.failingTotal > brief.failing.length && (
        <button
          type="button"
          className="ob-more ob-more-link"
          onClick={() => onNavigate?.('failures')}
        >
          +{brief.failingTotal - brief.failing.length} more in Failures
        </button>
      )}
    </section>
  )
}

/** 3. Why: the engine's causal chain (warning signals only). */
function WhyChain({ chain }) {
  if (chain.length < 2) return null
  return (
    <section className="ob-section ob-half" aria-labelledby="ob-why-title">
      <h2 id="ob-why-title" className="ob-section-title">Why</h2>
      <ol className="ob-chain">
        {chain.map((step, i) => (
          <li key={`${step}-${i}`} className="ob-chain-step">{step}</li>
        ))}
      </ol>
    </section>
  )
}

/** 4. What changed before it: pinned workload change and latest rollout. */
function WhatChanged({ changes, onInspectKeyChange }) {
  if (!changes.length) return null
  const open = (c) => {
    if (!c.kind || !c.name) return
    requestInspectorTab(c.kind, c.name, 'changes')
    onInspectKeyChange?.(buildInspectKey(c.kind, c.name, c.namespace))
  }
  return (
    <section className="ob-section ob-half" aria-labelledby="ob-changed-title">
      <h2 id="ob-changed-title" className="ob-section-title">What changed</h2>
      <ul className="ob-changes">
        {changes.map((c) => (
          <li key={c.key}>
            <button
              type="button"
              className="ob-change"
              onClick={() => open(c)}
              disabled={!c.kind}
              title={c.kind ? "Open this workload's Changes tab" : undefined}
            >
              <span className="ob-change-text">{c.text}</span>
              <span className="ob-change-meta">
                {c.beforeIncident && <span className="ob-tag is-warn">before incident</span>}
                {c.areas?.length > 0 && <span className="ob-tag">{c.areas.join(', ')}</span>}
                {c.at && <span>{since(c.at)}</span>}
              </span>
            </button>
          </li>
        ))}
      </ul>
    </section>
  )
}

/** 5. Warning event and log patterns, ranked; routine Normal events are filtered out. */
function TopPatterns({ patterns, onNavigate }) {
  if (!patterns.length) return null
  return (
    <section className="ob-section" aria-labelledby="ob-patterns-title">
      <div className="ob-section-head">
        <h2 id="ob-patterns-title" className="ob-section-title">Warning patterns</h2>
        <button type="button" className="ob-link" onClick={() => onNavigate?.('patterns')}>
          All patterns →
        </button>
      </div>
      <ul className="ob-patterns">
        {patterns.map((p) => (
          <li key={p.id} className="ob-pattern">
            <span className={`ob-dot is-${p.severity === 'warning' ? 'warning' : 'critical'}`} aria-hidden="true" />
            <span className="ob-pattern-text" title={p.text}>{p.text}</span>
            <span className="ob-pattern-source">{p.source}</span>
            <span className="ob-pattern-count mono">×{p.count}</span>
            <span className="ob-pattern-pods">{p.pods ? `${p.pods} pod${p.pods === 1 ? '' : 's'}` : ''}</span>
            <span className="ob-pattern-trend" title={p.trend}>
              {TREND_GLYPH[p.trend] || ''} {p.trend}
            </span>
          </li>
        ))}
      </ul>
    </section>
  )
}

function CopyButton({ text }) {
  const [copied, setCopied] = useState(false)
  async function copy() {
    try {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1200)
    } catch {
      /* clipboard unavailable */
    }
  }
  return (
    <button type="button" className="ob-copy" onClick={copy} aria-label="Copy">
      {copied ? 'Copied' : 'Copy'}
    </button>
  )
}

/** 6. Next checks: suggestions to read and copy. Klew never runs them. */
function NextChecks({ checks }) {
  if (!checks.length) return null
  return (
    <section className="ob-section" aria-labelledby="ob-next-title">
      <div className="ob-section-head">
        <h2 id="ob-next-title" className="ob-section-title">Next checks</h2>
        <span className="ob-section-count">suggestions · never run by Klew</span>
      </div>
      <ul className="ob-checks">
        {checks.map((c, i) => (
          <li key={`${c}-${i}`} className="ob-check">
            <code className="ob-check-text mono">{c}</code>
            <CopyButton text={c} />
          </li>
        ))}
      </ul>
    </section>
  )
}

/** 7. Earlier incidents, collapsed: recovered bursts stay one click away. */
function EarlierIncidents({ incidents }) {
  if (!incidents.length) return null
  return (
    <details className="ob-details">
      <summary>
        Earlier · {incidents.length} recovered incident{incidents.length === 1 ? '' : 's'}
      </summary>
      <ul className="ob-incidents">
        {incidents.map((r) => (
          <li key={r.key} className="ob-incident">
            <span className="mono">{r.range}</span>
            <span>{r.reason}</span>
            {r.count > 1 && <span className="ob-muted">×{r.count}</span>}
            <RecordedBadge show={r.recorded} />
          </li>
        ))}
      </ul>
    </details>
  )
}

/** Ongoing incident bursts when no pod is failing (e.g. admission denials, probe noise). */
function OngoingIncidents({ incidents }) {
  if (!incidents.length) return null
  return (
    <section className="ob-section" aria-labelledby="ob-ongoing-title">
      <h2 id="ob-ongoing-title" className="ob-section-title">Ongoing</h2>
      <ul className="ob-incidents">
        {incidents.map((r) => (
          <li key={r.key} className="ob-incident">
            <span className="mono">{r.range}</span>
            <span>{r.reason}</span>
            {r.count > 1 && <span className="ob-muted">×{r.count}</span>}
          </li>
        ))}
      </ul>
    </section>
  )
}

export function OverviewBrief({ brief, meta, onInspect, onInspectKeyChange, onNavigate, children }) {
  return (
    <div className="ob">
      <BriefVerdict brief={brief} meta={meta} />
      <PodHeatmap heatmap={brief.heatmap} onInspect={onInspect} />
      <FailingNow brief={brief} onInspect={onInspect} onNavigate={onNavigate} />
      {brief.failing.length === 0 && <OngoingIncidents incidents={brief.ongoing} />}
      {(brief.chain.length >= 2 || brief.changes.length > 0) && (
        <div className="ob-row">
          <WhyChain chain={brief.chain} />
          <WhatChanged changes={brief.changes} onInspectKeyChange={onInspectKeyChange} />
        </div>
      )}
      <TopPatterns patterns={brief.patterns} onNavigate={onNavigate} />
      <NextChecks checks={brief.nextChecks} />
      <EarlierIncidents incidents={brief.earlier} />
      {children}
    </div>
  )
}
