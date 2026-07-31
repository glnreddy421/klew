import { useEffect, useMemo, useState } from 'react'
import { StatusBadge } from './StatusBadge'
import { KindIcon } from '../KindIcon'
import {
  deriveSignalStats,
  hasAnomalyIssues,
} from '../../lib/incidentLayout'

/**
 * Kind-aware object inspector.
 * Prefer live `detail` sections from GetObjectDetails; fall back to snapshot inspect.
 */
export function ComponentInspectPanel({
  inspect,
  emptyHint,
  layoutMode = 'detail-tabs',
  onFocus,
  focusPinned = false,
  showFocusCta = false,
  loading = false,
  error = null,
}) {
  const groups = useMemo(
    () => (inspect?.groups?.length ? inspect.groups : fallbackGroups(inspect)),
    [inspect],
  )

  if (!inspect) {
    return (
      <div className="inspect-empty muted">
        {loading
          ? 'Loading object details…'
          : (emptyHint || 'Select a component to inspect.')}
      </div>
    )
  }

  const unhealthy = hasAnomalyIssues(inspect)
  const focusKey = inspect.key

  const header = (
    <InspectIdentityHeader
      inspect={inspect}
      showFocusCta={showFocusCta && !focusPinned}
      onFocus={() => onFocus?.(focusKey)}
      loading={loading}
      error={error}
    />
  )

  switch (layoutMode) {
    case 'signal-first':
      return (
        <SignalFirstPanel
          inspect={inspect}
          unhealthy={unhealthy}
          header={header}
          groups={groups}
        />
      )

    case 'detail-tabs':
      return (
        <DetailTabsPanel
          inspect={inspect}
          unhealthy={unhealthy}
          header={header}
          groups={groups}
        />
      )

    case 'master-detail':
    case 'unified-select':
    case 'dense-list':
    case 'current':
    default:
      return (
        <div className={`inspect-panel mode-${layoutMode || 'current'}`}>
          {header}
          {layoutMode === 'unified-select' && !focusPinned && (
            <p className="inspect-unified-banner">
              Selected for inspect — use <strong>Focus chain</strong> to isolate related resources.
            </p>
          )}
          <SignalsBlock inspect={inspect} unhealthy={unhealthy} quietHealthy />
          <StackedSections groups={groups} />
        </div>
      )
  }
}

function fallbackGroups(inspect) {
  if (!inspect) return []
  const groups = []
  if (inspect.status?.fields?.length) {
    groups.push({
      id: 'status',
      label: 'Status',
      sections: [{
        id: 'status',
        title: 'Status',
        fields: inspect.status.fields.map((f) => ({ key: f.k, value: f.v })),
      }],
    })
  }
  if (inspect.resourceBars?.length) {
    groups.push({
      id: 'runtime',
      label: 'Runtime',
      sections: [{ id: 'resources', title: 'Resources', _resourceBars: inspect.resourceBars }],
    })
  }
  const labels = inspect.meta?.labels || []
  const annotations = inspect.meta?.annotations || []
  if (labels.length || annotations.length) {
    groups.push({
      id: 'metadata',
      label: 'Metadata',
      sections: [{
        id: 'labels',
        title: 'Labels & annotations',
        _labels: labels,
        _annotations: annotations,
      }],
    })
  }
  if (inspect.events?.length) {
    groups.push({
      id: 'events',
      label: 'Events',
      sections: [{ id: 'events', title: 'Events', _events: inspect.events }],
    })
  }
  return groups
}

function InspectIdentityHeader({ inspect, showFocusCta, onFocus, loading, error }) {
  return (
    <div className="inspect-header inspect-header-actions">
      <div className="inspect-title-block">
        <span className="inspect-category">{inspect.categoryLabel}</span>
        <h4 className="inspect-name">
          <KindIcon kind={inspect.kind} size={16} />
          <span className="inspect-name-text">{inspect.name}</span>
        </h4>
        {inspect.namespace && (
          <span className="inspect-ns muted mono">{inspect.namespace}</span>
        )}
      </div>
      <div className="inspect-header-right">
        {loading && <span className="muted inspect-loading">Updating…</span>}
        {error && <span className="inspect-error" title={error}>Live fetch failed</span>}
        <StatusBadge status={inspect.status.tone} label={inspect.status.label} />
        {showFocusCta && (
          <button
            type="button"
            className="btn btn-outline inspect-focus-cta"
            onClick={onFocus}
            aria-label={`Focus chain for ${inspect.kind}/${inspect.name}`}
          >
            Focus chain
          </button>
        )}
      </div>
    </div>
  )
}

function SignalFirstPanel({ inspect, unhealthy, header, groups }) {
  const [metaOpen, setMetaOpen] = useState(!unhealthy)
  const stats = deriveSignalStats(inspect)

  useEffect(() => {
    setMetaOpen(!unhealthy)
  }, [inspect.key, unhealthy])

  return (
    <div className="inspect-panel mode-signal-first">
      {header}
      {unhealthy ? (
        <>
          <div className="inspect-signal-hero">
            <AnomalyCallout inspect={inspect} large />
            {stats.length > 0 && (
              <div className="inspect-signal-stats">
                {stats.map((s) => (
                  <div key={s.label} className="inspect-signal-stat">
                    <span className="signal-stat-value">{s.value}</span>
                    <span className="signal-stat-label">{s.label}</span>
                  </div>
                ))}
              </div>
            )}
          </div>
          <button
            type="button"
            className="inspect-meta-toggle"
            aria-expanded={metaOpen}
            onClick={() => setMetaOpen((v) => !v)}
          >
            {metaOpen ? 'Hide details' : 'Show details'}
          </button>
          {metaOpen && <StackedSections groups={groups} />}
        </>
      ) : (
        <>
          <p className="inspect-quiet-ok muted">No anomalies on this component.</p>
          <StackedSections groups={groups} />
        </>
      )}
    </div>
  )
}

function DetailTabsPanel({ inspect, unhealthy, header, groups }) {
  const [tab, setTab] = useState(groups[0]?.id || 'status')

  useEffect(() => {
    setTab(groups[0]?.id || 'status')
  }, [inspect.key, groups])

  const active = groups.find((g) => g.id === tab) || groups[0]

  return (
    <div className="inspect-panel mode-detail-tabs">
      {header}
      <SignalsBlock inspect={inspect} unhealthy={unhealthy} quietHealthy />
      {groups.length > 0 && (
        <>
          <div className="inspect-tabs" role="tablist" aria-label="Detail sections">
            {groups.map((g) => (
              <button
                key={g.id}
                type="button"
                role="tab"
                aria-selected={tab === g.id}
                className={`inspect-tab ${tab === g.id ? 'active' : ''}`}
                onClick={() => setTab(g.id)}
              >
                {g.label}
              </button>
            ))}
          </div>
          <div className="inspect-tab-panel" role="tabpanel">
            {active && <GroupBody group={active} />}
          </div>
        </>
      )}
    </div>
  )
}

function StackedSections({ groups }) {
  if (!groups?.length) return null
  return (
    <div className="inspect-stack">
      {groups.map((g) => (
        <GroupBody key={g.id} group={g} showHeading />
      ))}
    </div>
  )
}

function GroupBody({ group, showHeading = false }) {
  return (
    <div className="inspect-group">
      {showHeading && <h5 className="inspect-group-title">{group.label}</h5>}
      {group.sections.map((s) => (
        <DetailSection key={s.id || s.title} section={s} />
      ))}
    </div>
  )
}

function DetailSection({ section }) {
  if (!section) return null

  if (section._resourceBars) {
    return (
      <section className="inspect-section">
        <h5>{section.title}</h5>
        <div className="resource-bars">
          {section._resourceBars.map((bar) => (
            <ResourceBar key={bar.id} bar={bar} />
          ))}
        </div>
      </section>
    )
  }

  if (section._labels || section._annotations) {
    const labels = section._labels || []
    const annotations = section._annotations || []
    if (!labels.length && !annotations.length) return null
    return (
      <section className="inspect-section">
        <h5>{section.title}</h5>
        {labels.length > 0 && (
          <div className="inspect-meta-block">
            <span className="inspect-meta-label">Labels</span>
            <div className="inspect-chips">
              {labels.map((l) => (
                <span key={l.k} className="inspect-chip" title={`${l.k}=${l.v}`}>
                  <span className="chip-k">{l.k}</span>
                  <span className="chip-v">{l.v}</span>
                </span>
              ))}
            </div>
          </div>
        )}
        {annotations.length > 0 && (
          <div className="inspect-meta-block">
            <span className="inspect-meta-label">Annotations</span>
            <div className="inspect-chips">
              {annotations.map((a) => (
                <span key={a.k} className="inspect-chip annot" title={`${a.k}=${a.v}`}>
                  <span className="chip-k">{a.k}</span>
                  <span className="chip-v">{a.v}</span>
                </span>
              ))}
            </div>
          </div>
        )}
      </section>
    )
  }

  if (section._events) {
    return (
      <section className="inspect-section">
        <h5>{section.title}</h5>
        <ul className="inspect-events">
          {section._events.map((ev, i) => (
            <li key={`${ev.time}-${i}`} className={`inspect-event sev-${ev.severity}`}>
              <span className="inspect-event-time mono">{ev.time}</span>
              <span className="inspect-event-reason">
                {ev.reason || ev.type}
                {ev.count > 1 ? ` ×${ev.count}` : ''}
              </span>
              <span className="inspect-event-msg" title={ev.message}>{ev.message}</span>
            </li>
          ))}
        </ul>
      </section>
    )
  }

  const hasFields = section.fields?.length > 0
  const hasKV = section.keyValues?.length > 0
  const hasTable = section.table?.rows?.length > 0
  const hasNotes = section.notes?.length > 0
  if (!hasFields && !hasKV && !hasTable && !hasNotes) return null

  return (
    <section className="inspect-section">
      <h5>{section.title}</h5>
      {hasFields && (
        <dl className="inspect-fields">
          {section.fields.map((f, i) => (
            <div key={`${f.key}-${i}`} className="inspect-field">
              <dt>{f.key}</dt>
              <dd title={f.value}>{f.value}</dd>
            </div>
          ))}
        </dl>
      )}
      {hasKV && (
        <div className="inspect-chips">
          {section.keyValues.map((kv) => (
            <span key={kv.key} className="inspect-chip" title={`${kv.key}=${kv.value}`}>
              <span className="chip-k">{kv.key}</span>
              <span className="chip-v">{kv.value}</span>
            </span>
          ))}
        </div>
      )}
      {hasTable && (
        <div className="inspect-table-wrap">
          <table className="inspect-table">
            <thead>
              <tr>
                {section.table.columns.map((c) => (
                  <th key={c}>{c}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {section.table.rows.map((row, i) => (
                <tr key={i}>
                  {row.map((cell, j) => (
                    <td key={j} title={cell}>{cell || '—'}</td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {hasNotes && (
        <ul className="inspect-notes">
          {section.notes.map((n, i) => (
            <li key={i} className="muted">{n}</li>
          ))}
        </ul>
      )}
    </section>
  )
}

function SignalsBlock({ inspect, unhealthy, quietHealthy }) {
  if (quietHealthy && !unhealthy) {
    return <p className="inspect-quiet-ok muted">No anomalies on this component.</p>
  }

  return (
    <section className={`inspect-section inspect-anomalies ${unhealthy ? 'has-issues' : 'clear'}`}>
      <h5>Signals</h5>
      <p className="inspect-events-hint muted">
        Anomalies for this component only — not the overall investigation.
      </p>
      {inspect.anomalies?.length ? (
        <ul className="inspect-anomaly-list">
          {inspect.anomalies.map((a, i) => (
            <li key={`${a.text}-${i}`} className={`inspect-anomaly level-${a.level}`}>
              <span className="anomaly-mark">{a.level === 'crit' ? '!' : a.level === 'warn' ? '▲' : '·'}</span>
              <span className="anomaly-text">{a.text}</span>
              {a.source && <span className="anomaly-src muted">{a.source}</span>}
            </li>
          ))}
        </ul>
      ) : (
        <p className="inspect-ok">No anomalies on this component.</p>
      )}
    </section>
  )
}

function AnomalyCallout({ inspect, large = false }) {
  const primary = (inspect.anomalies || []).find((a) => a.level === 'crit' || a.level === 'warn')
    || inspect.anomalies?.[0]
  if (!primary) return null
  return (
    <div className={`inspect-anomaly-callout ${large ? 'large' : ''} level-${primary.level}`}>
      <span className="callout-icon" aria-hidden="true">
        {primary.level === 'crit' ? '!' : '▲'}
      </span>
      <div className="callout-body">
        <strong className="callout-title">{primary.text}</strong>
        {primary.source && (
          <p className="callout-sub muted">{primary.source}</p>
        )}
        {(inspect.anomalies || []).length > 1 && (
          <ul className="inspect-anomaly-list callout-rest">
            {inspect.anomalies.slice(1).map((a, i) => (
              <li key={`${a.text}-${i}`} className={`inspect-anomaly level-${a.level}`}>
                <span className="anomaly-text">{a.text}</span>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}

function ResourceBar({ bar }) {
  if (bar.empty) {
    return (
      <div className="resource-bar empty">
        <div className="resource-bar-head">
          <span>{bar.label}</span>
          <span className="muted">{bar.source}</span>
        </div>
        <p className="muted inspect-none">{bar.detail}</p>
      </div>
    )
  }

  return (
    <div className={`resource-bar ${bar.compact ? 'compact' : ''}`}>
      <div className="resource-bar-head">
        <span>{bar.label}</span>
        <span className="resource-bar-detail mono">{bar.detail}</span>
      </div>
      <div className="resource-track" title={bar.detail}>
        {bar.limitPct > 0 && (
          <div className="resource-fill limit" style={{ width: `${bar.limitPct}%` }} />
        )}
        <div className="resource-fill request" style={{ width: `${bar.requestPct}%` }} />
        {bar.usagePct != null && (
          <div className="resource-fill usage" style={{ width: `${bar.usagePct}%` }} />
        )}
      </div>
      <div className="resource-legend">
        <span><i className="swatch request" /> request</span>
        {bar.limit > 0 && <span><i className="swatch limit" /> limit</span>}
        {bar.usage != null && <span><i className="swatch usage" /> usage</span>}
        <span className="muted">{bar.source}</span>
      </div>
    </div>
  )
}
