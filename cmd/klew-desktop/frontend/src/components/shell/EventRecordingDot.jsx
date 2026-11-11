import { ContextRecordingToggle } from './ContextRecordingToggle.jsx'

/**
 * Top-bar recording control — same history icon toggle as Home tiles/list.
 * Gap details live in Settings → Event history, not a hover dump here.
 */
export function EventRecordingDot({ contextName = '' }) {
  if (!contextName) return null
  return (
    <ContextRecordingToggle
      contextName={contextName}
      size={15}
      className="event-recording-topbar-btn"
    />
  )
}
