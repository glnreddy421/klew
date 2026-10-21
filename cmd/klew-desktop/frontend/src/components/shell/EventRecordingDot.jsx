import { useEventHistoryContext } from '../../context/EventHistoryContext.jsx'
import { recordingTone, recordingTooltip } from '../../lib/eventHistory'

/** Small dot beside the cluster cloud: local event recording for the active context. */
export function EventRecordingDot({ contextName = '' }) {
  const history = useEventHistoryContext()
  if (!history || !contextName) return null
  const status = history.byContext?.[contextName] || null
  const tone = recordingTone(status)
  const title = recordingTooltip(status)
  return (
    <span
      className={`event-recording-dot tone-${tone}`}
      title={title}
      aria-label={title}
      role="img"
    />
  )
}
