import { getSnapshot } from '../../lib/investigationViews'
import { evidenceHistoryLine } from '../../lib/eventHistory'
import { CoverageLine } from '../shell/HistoryGapsPopover.jsx'

/** "Includes recorded events · history since 09:09 · 2 gaps" — only when history was used. */
export function RecordedHistoryLine({ view, className = '' }) {
  const history = getSnapshot(view).eventHistory
  return (
    <CoverageLine
      history={history}
      text={evidenceHistoryLine(history)}
      className={['evidence-history-line', className].filter(Boolean).join(' ')}
    />
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
