import { useMemo, useState } from 'react'
import { KindIcon } from '../KindIcon'
import { relativeTime } from '../../lib/workloadChanges.js'
import { groupWarnings } from '../../lib/recentWarnings.js'

const ATTENTION_PREVIEW = 8

function ObjectButton({ item, onInspect, children, className = '' }) {
  return (
    <button
      type="button"
      className={`overview-activity-object ${className}`.trim()}
      onClick={() => onInspect?.(item.key)}
      title={`Inspect ${item.kind} ${item.namespace ? `${item.namespace}/` : ''}${item.name}`}
    >
      {children}
    </button>
  )
}

export function AttentionList({ items = [], loading, onInspect }) {
  const [expanded, setExpanded] = useState(false)
  const shown = expanded ? items : items.slice(0, ATTENTION_PREVIEW)
  const hidden = items.length - shown.length

  return (
    <section className="overview-activity-card" aria-label="Needs attention">
      <div className="overview-activity-head">
        <h3 className="workload-overview-section-title">Needs attention</h3>
        {items.length > 0 && <span className="overview-activity-count overview-num">{items.length}</span>}
      </div>
      {items.length === 0 ? (
        <p className="overview-activity-empty muted">
          {loading ? 'Loading…' : 'No unhealthy workloads or pods in scope.'}
        </p>
      ) : (
        <ul className="overview-activity-list">
          {shown.map((item) => (
            <li key={item.key}>
              <ObjectButton item={item} onInspect={onInspect} className={`overview-attention-row tone-${item.tone}`}>
                <span className={`workload-donut-swatch tone-${item.tone}`} aria-hidden="true" />
                <KindIcon kind={item.kind} size={14} />
                <span className="overview-activity-name">{item.name}</span>
                {item.namespace && <span className="overview-activity-ns muted">{item.namespace}</span>}
                <span className="overview-activity-reason">{item.reason}</span>
              </ObjectButton>
            </li>
          ))}
        </ul>
      )}
      {(hidden > 0 || expanded) && items.length > ATTENTION_PREVIEW && (
        <button type="button" className="overview-activity-more" onClick={() => setExpanded((v) => !v)}>
          {expanded ? 'Show fewer' : `Show ${hidden} more`}
        </button>
      )}
    </section>
  )
}

export function RecentWarningsList({ events = [], total = 0, accessState, error, loading, onInspect }) {
  const groups = useMemo(() => groupWarnings(events), [events])
  const denied = accessState === 'forbidden'

  let empty = null
  if (denied) empty = 'Events are not listable with the current identity.'
  else if (accessState === 'error') empty = 'Events could not be loaded.'
  else if (!groups.length) empty = loading ? 'Loading…' : 'No warning events in scope.'

  return (
    <section className="overview-activity-card" aria-label="Recent warnings">
      <div className="overview-activity-head">
        <h3 className="workload-overview-section-title">Recent warnings</h3>
        {total > 0 && <span className="overview-activity-count overview-num">{total}</span>}
      </div>
      {empty ? (
        <p className="overview-activity-empty muted" title={accessState === 'error' ? error : undefined}>{empty}</p>
      ) : (
        <ul className="overview-activity-list">
          {groups.map((g) => (
            <li key={g.reason} className="overview-warning-group">
              <div className="overview-warning-line">
                <span className="overview-warning-reason">{g.reason}</span>
                {g.count > 1 && <span className="overview-warning-times overview-num">×{g.count}</span>}
                <span className="overview-warning-age muted" title={g.lastSeen}>{relativeTime(g.lastSeen)}</span>
              </div>
              {g.message && <p className="overview-warning-message muted">{g.message}</p>}
              {g.objects.length > 0 && (
                <div className="overview-warning-objects">
                  {g.objects.map((o) => (
                    <ObjectButton key={o.key} item={o} onInspect={onInspect} className="overview-warning-chip">
                      <KindIcon kind={o.kind} size={12} />
                      <span className="overview-activity-name">{o.name}</span>
                    </ObjectButton>
                  ))}
                  {g.moreObjects > 0 && (
                    <span className="overview-warning-more muted">+{g.moreObjects} more</span>
                  )}
                </div>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
