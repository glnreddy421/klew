import { incidentRows } from '../../lib/incidents'
import { RecordedBadge } from './RecordedHistoryLine.jsx'

/** Incidents in the window: time range, leading reason, recovered or ongoing. */
export function IncidentList({ incidents }) {
  const rows = incidentRows(incidents)
  if (!rows.length) return null
  return (
    <ul className="inv-incidents" aria-label="Incidents in this window">
      {rows.map((r) => (
        <li key={r.key} className={`inv-incident is-${r.status}`}>
          <span className="inv-incident-range mono">{r.range}</span>
          <span className="inv-incident-reason">{r.reason}</span>
          {r.count > 1 && <span className="inv-incident-count muted">×{r.count}</span>}
          <span className={`inv-incident-status tone-${r.status === 'recovered' ? 'ok' : 'warn'}`}>{r.status}</span>
          <RecordedBadge show={r.recorded} />
        </li>
      ))}
    </ul>
  )
}
