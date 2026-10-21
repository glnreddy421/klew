import { useEventHistoryContext } from '../../context/EventHistoryContext.jsx'
import { contextRecordingLabel, isRecordingStatus, recordingTooltip, shouldShowContextRecording } from '../../lib/eventHistory'

/** "Recording · 3d history" for a context row (Home list, context popover). */
export function ContextRecordingHint({ contextName, className = '' }) {
  const history = useEventHistoryContext()
  const status = history?.byContext?.[contextName]
  if (!shouldShowContextRecording(status)) return null
  const cls = ['event-recording-hint', isRecordingStatus(status) ? 'is-recording' : '', className]
    .filter(Boolean).join(' ')
  return (
    <span className={cls} title={recordingTooltip(status)}>
      {contextRecordingLabel(status)}
    </span>
  )
}
