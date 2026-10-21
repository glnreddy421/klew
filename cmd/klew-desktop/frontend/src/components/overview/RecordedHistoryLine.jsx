import { getSnapshot } from '../../lib/investigationViews'
import { evidenceHistoryLine } from '../../lib/eventHistory'

/** "Includes recorded events from the last 6h · gaps: …" — only when history was used. */
export function RecordedHistoryLine({ view, className = '' }) {
  const line = evidenceHistoryLine(getSnapshot(view).eventHistory)
  if (!line) return null
  return (
    <p
      className={['evidence-history-line', className].filter(Boolean).join(' ')}
      title="Events Klew recorded locally while it was running. History is incomplete where Klew was not running."
    >
      {line}
    </p>
  )
}

export function RecordedBadge({ show }) {
  if (!show) return null
  return (
    <span className="evidence-recorded-badge" title="From Klew's local event history — no longer in the Kubernetes API">
      recorded
    </span>
  )
}
