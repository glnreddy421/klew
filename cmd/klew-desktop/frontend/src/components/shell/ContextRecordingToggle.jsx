import { useEventHistoryContext } from '../../context/EventHistoryContext.jsx'
import { isRecordedContext } from '../../lib/clusterContexts.js'
import { isRecordingStatus } from '../../lib/eventHistory'
import { HoverTip } from '../HoverTip.jsx'
import { RecordHistoryIcon } from './ContextActionIcons.jsx'

/**
 * Small history-icon toggle for per-context event recording (tiles / list / top bar).
 * Master Settings switch still wins when global recording is off.
 */
export function ContextRecordingToggle({ contextName, className = '', size = 16 }) {
  const history = useEventHistoryContext()
  if (!history || !contextName) return null

  const status = history.byContext?.[contextName]
  const on = isRecordedContext(history.recordedContexts, contextName)
  const live = isRecordingStatus(status)
  const globalOff = history.settings && history.settings.enabled === false
  const unavailable = history.settings?.available === false
  const disabled = globalOff || unavailable || !history.onToggleRecorded

  let tipLabel = on ? 'Recording events' : 'Not recording'
  let tipDetail = on
    ? 'Klew saves cluster Events locally in the background. Click to stop.'
    : 'Click to record Events for this cluster while Klew is open.'
  if (globalOff) {
    tipLabel = 'Recording off in Settings'
    tipDetail = 'Turn on “Record Kubernetes events locally” in Settings → Investigation.'
  } else if (unavailable) {
    tipLabel = 'Event history unavailable'
    tipDetail = 'Local history could not be opened on this machine.'
  } else if (on && live) {
    tipLabel = 'Recording now'
    tipDetail = 'Events for this cluster are being saved. Click to stop.'
  }

  return (
    <HoverTip label={tipLabel} detail={tipDetail} side="bottom">
      <button
        type="button"
        className={[
          'context-record-icon-btn',
          on ? 'is-on' : '',
          live ? 'is-live' : '',
          className,
        ].filter(Boolean).join(' ')}
        role="switch"
        aria-checked={on}
        aria-label={on ? `Stop recording ${contextName}` : `Record ${contextName}`}
        disabled={disabled}
        onClick={(e) => {
          e.preventDefault()
          e.stopPropagation()
          if (!disabled) history.onToggleRecorded(contextName)
        }}
      >
        <RecordHistoryIcon active={on} size={size} />
      </button>
    </HoverTip>
  )
}
