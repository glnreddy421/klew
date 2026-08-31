import { ComponentInspectPanel } from './ComponentInspectPanel'
import { KindIcon } from '../KindIcon'
import { formatEntityAge } from '../../lib/entityTable'
import { getSnapshot } from '../../lib/investigationViews'

function countSignals(view) {
  const signals = view?.signals || []
  let errors = 0
  let warnings = 0
  let info = 0
  for (const s of signals) {
    const level = s.level || s.severity || ''
    if (level === 'crit' || level === 'critical' || level === 'error') errors += 1
    else if (level === 'warn' || level === 'warning') warnings += 1
    else info += 1
  }
  return { errors, warnings, info }
}

function topSignals(view, inspect, limit = 4) {
  const fromInspect = (inspect?.anomalies || []).map((a) => ({
    text: a.text,
    count: 1,
    level: a.level,
  }))
  if (fromInspect.length) return fromInspect.slice(0, limit)

  const signals = view?.signals || []
  return signals.slice(0, limit).map((s) => ({
    text: s.text || s.summary || s.kind || 'Signal',
    count: s.count || 1,
    level: s.level || 'info',
  }))
}

function entityDetailFields(inspect, inspectRow, view) {
  if (!inspect && !inspectRow) return []
  const snap = getSnapshot(view)
  const pod = inspectRow?.kind === 'Pod'
    ? snap.pods?.find((p) => p.name === inspectRow.name)
    : null
  const fields = []
  if (inspectRow?.name) fields.push({ label: 'Name', value: inspectRow.name })
  if (inspectRow?.namespace || inspect?.namespace) {
    fields.push({ label: 'Namespace', value: inspectRow?.namespace || inspect?.namespace })
  }
  const status = inspect?.status?.label || pod?.phase || inspectRow?.signal
  if (status) fields.push({ label: 'Status', value: status })
  if (pod?.node) fields.push({ label: 'Node', value: pod.node })
  if (pod?.createdAt) fields.push({ label: 'Age', value: formatEntityAge(pod.createdAt) })
  return fields.slice(0, 6)
}

/**
 * Right investigation panel — summary, top signals, entity details.
 */
export function InvestigationSignalsPanel({
  view,
  inspect,
  inspectRow,
  layoutMode,
  focusPinned,
  showFocusCta,
  onFocus,
  onInspect,
  loading,
  error,
  emptyHint,
  onViewAllSignals,
  expanded = false,
}) {
  const counts = countSignals(view)
  const signals = topSignals(view, inspect)
  const details = entityDetailFields(inspect, inspectRow, view)
  const hasSelection = Boolean(inspect || inspectRow)

  if (!hasSelection && !loading) {
    return (
      <div className="signals-panel">
        <header className="signals-panel-header">
          <h3>Signals & details</h3>
        </header>
        <div className="signals-panel-empty muted">{emptyHint}</div>
      </div>
    )
  }

  if (expanded || layoutMode === 'detail-tabs') {
    return (
      <div className="signals-panel signals-panel-expanded">
        <header className="signals-panel-header">
          <h3>Investigation</h3>
        </header>
        <div className="signals-panel-body inspect-card-body">
          <ComponentInspectPanel
            inspect={inspect}
            layoutMode={layoutMode}
            focusPinned={focusPinned}
            showFocusCta={showFocusCta}
            onFocus={onFocus}
            onInspect={onInspect}
            loading={loading}
            error={error}
            emptyHint={emptyHint}
          />
        </div>
      </div>
    )
  }

  return (
    <div className="signals-panel">
      <header className="signals-panel-header">
        <h3>Signals & details</h3>
        {inspectRow && (
          <div className="signals-panel-entity">
            <KindIcon kind={inspectRow.kind} size={16} />
            <span className="mono">{inspectRow.name}</span>
          </div>
        )}
      </header>

      <div className="signals-panel-body">
        <section className="signals-section">
          <h4 className="signals-section-label">Summary</h4>
          <div className="signals-summary-grid">
            <SummaryStat label="Errors" value={counts.errors} tone="crit" />
            <SummaryStat label="Warnings" value={counts.warnings} tone="warn" />
            <SummaryStat label="Info" value={counts.info} tone="info" />
          </div>
        </section>

        <section className="signals-section">
          <h4 className="signals-section-label">Top signals</h4>
          {signals.length ? (
            <ul className="signals-top-list">
              {signals.map((s, i) => (
                <li key={`${s.text}-${i}`} className={`signals-top-item level-${s.level || 'info'}`}>
                  <span className="signals-top-mark" aria-hidden="true" />
                  <span className="signals-top-text">{s.text}</span>
                  {s.count > 1 && <span className="signals-top-count">{s.count}</span>}
                </li>
              ))}
            </ul>
          ) : (
            <p className="muted signals-none">No signals on this entity.</p>
          )}
          {onViewAllSignals && (
            <button type="button" className="signals-link-btn" onClick={onViewAllSignals}>
              View all signals →
            </button>
          )}
        </section>

        {details.length > 0 && (
          <section className="signals-section">
            <h4 className="signals-section-label">Entity details</h4>
            <dl className="signals-entity-props">
              {details.map((f) => (
                <div key={f.label} className="signals-entity-row">
                  <dt>{f.label}</dt>
                  <dd title={f.value}>{f.value}</dd>
                </div>
              ))}
            </dl>
          </section>
        )}

        {inspect && (
          <div className="signals-panel-footer">
            <p className="muted signals-deep-hint">Full object details load when you expand investigation flow layout or select deeper inspect tabs.</p>
          </div>
        )}
      </div>
    </div>
  )
}

function SummaryStat({ label, value, tone }) {
  return (
    <div className={`signals-stat signals-stat-${tone}`}>
      <span className="signals-stat-value">{value}</span>
      <span className="signals-stat-label">{label}</span>
    </div>
  )
}
