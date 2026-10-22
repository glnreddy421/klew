import { useEventHistoryContext } from '../../context/EventHistoryContext.jsx'
import { recordingTone, recordingTooltip } from '../../lib/eventHistory'
import { HistoryGapsPopover } from './HistoryGapsPopover.jsx'

/** Small dot beside the cluster cloud: local event recording for the active context. */
export function EventRecordingDot({ contextName = '' }) {
  const history = useEventHistoryContext()
  if (!history || !contextName) return null
  const status = history.byContext?.[contextName] || null
  const tone = recordingTone(status)
  const summary = recordingTooltip(status)
  return (
    <HistoryGapsPopover
      className="event-recording"
      history={status}
      title={summary}
      align="end"
      trigger={<span className={`event-recording-dot tone-${tone}`} role="img" aria-label={summary} />}
    />
  )
}
