import { KindIcon } from '../KindIcon'
import { useWorkloadOverview } from '../../hooks/useWorkloadOverview.js'
import { donutGradient } from '../../lib/workloadOverview.js'

function DonutLegend({ segments }) {
  return (
    <ul className="workload-donut-legend">
      {segments.map((seg) => (
        <li key={seg.label}>
          <span className={`workload-donut-swatch tone-${seg.tone}`} aria-hidden="true" />
          <span>{seg.label}: {seg.count}</span>
        </li>
      ))}
    </ul>
  )
}

function WorkloadKindDonut({ card, onSelectKind }) {
  const hasData = card.total > 0 && card.segments.length > 0
  const gradient = donutGradient(card.segments)

  return (
    <article className={`workload-kind-card ${hasData ? '' : 'is-empty'}`}>
      <button
        type="button"
        className="workload-kind-card-title"
        onClick={() => onSelectKind?.(card)}
      >
        <KindIcon kind={card.kind} size={18} />
        <span>{card.label}</span>
        <span className="workload-kind-card-count">({card.total})</span>
      </button>

      {hasData ? (
        <>
          <div className="workload-donut" style={{ background: gradient }} aria-hidden="true">
            <div className="workload-donut-hole">
              <span className="workload-donut-total mono">{card.total}</span>
            </div>
          </div>
          <DonutLegend segments={card.segments} />
        </>
      ) : (
        <p className="workload-kind-empty muted">No {card.label.toLowerCase()} in scope</p>
      )}
    </article>
  )
}

/**
 * Workloads → Overview — Lens / Kubernetes Dashboard style status donuts.
 * Fetches catalog entities independently of nav selection and investigation rows.
 */
export function WorkloadOverviewPanel({
  cluster,
  browseScope,
  kindGroups = [],
  catalogLoading = false,
  onSelectKind,
}) {
  const { cards, loading, error } = useWorkloadOverview({
    cluster,
    browseScope,
    kindGroups,
    enabled: kindGroups.length > 0,
  })

  const busy = catalogLoading || loading

  if (!kindGroups.length) {
    return (
      <div className="workload-overview-panel">
        <p className="muted">
          {catalogLoading ? 'Loading workload types…' : 'No workload types discovered in this scope.'}
        </p>
      </div>
    )
  }

  return (
    <div className="workload-overview-panel">
      <header className="workload-overview-header">
        <h2 className="workload-overview-title">Overview</h2>
        <p className="workload-overview-lead muted">
          Status breakdown for workload types in the current browse scope.
        </p>
        {busy && <p className="workload-overview-status muted">Loading overview…</p>}
        {error && !busy && (
          <p className="workload-overview-status scope-toolbar-warn" title={error}>
            Some workload lists could not be loaded.
          </p>
        )}
      </header>
      <div className={`workload-overview-grid ${busy ? 'is-loading' : ''}`}>
        {cards.map((card) => (
          <WorkloadKindDonut
            key={card.kind}
            card={card}
            onSelectKind={onSelectKind}
          />
        ))}
      </div>
    </div>
  )
}
