import { useEventHistoryContext } from '../../context/EventHistoryContext.jsx'
import {
  contextRecordingLabel,
  isRecordingStatus,
  recordingTooltip,
  shouldShowContextRecording,
} from '../../lib/eventHistory'
import { isRecordedContext } from '../../lib/clusterContexts.js'

/**
 * Per-context recording control for Home tiles / list / popover.
 * Click toggles recording for that cluster (Settings global switch still wins).
 */
export function ContextRecordingHint({ contextName, className = '' }) {
  const history = useEventHistoryContext()
  const status = history?.byContext?.[contextName]
  const optedIn = isRecordedContext(history?.recordedContexts, contextName)
  const globalOff = history?.settings && history.settings.enabled === false
  const canToggle = Boolean(history?.onToggleRecorded) && !globalOff && history?.settings?.available !== false
  const show = shouldShowContextRecording(status) || optedIn || canToggle
  if (!show || !contextName) return null

  const label = globalOff
    ? 'Recording off in Settings'
    : (contextRecordingLabel(status) || (optedIn ? 'Recording' : 'Not recording'))
  const title = globalOff
    ? 'Turn on “Record Kubernetes events locally” in Settings, then toggle per cluster.'
    : `${recordingTooltip(status)}${canToggle ? (optedIn ? ' — click to stop' : ' — click to record') : ''}`
  const cls = [
    'event-recording-hint',
    isRecordingStatus(status) ? 'is-recording' : '',
    canToggle ? 'is-toggle' : '',
    className,
  ].filter(Boolean).join(' ')

  if (!canToggle) {
    return (
      <span className={cls} title={title}>
        {label}
      </span>
    )
  }

  return (
    <button
      type="button"
      className={cls}
      title={title}
      aria-pressed={optedIn}
      onClick={(e) => {
        e.preventDefault()
        e.stopPropagation()
        history.onToggleRecorded(contextName)
      }}
    >
      {label}
    </button>
  )
}
