import { useId, useRef, useState } from 'react'
import { createPortal } from 'react-dom'

function conditionTone(condition) {
  return conditionIsTrue(condition?.status) ? 'ok' : 'bad'
}

function conditionIsTrue(status) {
  return String(status || '').toLowerCase() === 'true'
}

function conditionIcon(type) {
  if (type === 'Available') {
    return (
      <svg viewBox="0 0 16 16" width="12" height="12" fill="currentColor" aria-hidden="true">
        <path d="M8 1.5a6.5 6.5 0 1 0 0 13 6.5 6.5 0 0 0 0-13Zm3.03 4.72-3.5 3.5a.75.75 0 0 1-1.06 0l-1.75-1.75a.75.75 0 1 1 1.06-1.06L7.25 8.19l2.97-2.97a.75.75 0 1 1 1.06 1.06Z" />
      </svg>
    )
  }
  if (type === 'Progressing') {
    return (
      <svg viewBox="0 0 16 16" width="12" height="12" fill="currentColor" aria-hidden="true">
        <path d="M8 2.5a.75.75 0 0 1 .75.75v2.19A3.56 3.56 0 0 1 10.94 8a.75.75 0 0 1-1.5 0A2.06 2.06 0 0 0 7.25 6.44V3.25A.75.75 0 0 1 8 2.5Zm0 11a.75.75 0 0 0 .75-.75V10.06A3.56 3.56 0 0 0 10.94 8a.75.75 0 0 0-1.5 0 2.06 2.06 0 0 1-1.69 1.56v2.19A.75.75 0 0 0 8 13.5ZM4.5 8A3.5 3.5 0 0 1 8 4.5a.75.75 0 0 0 0-1.5A5 5 0 1 0 8 13a.75.75 0 0 0 0-1.5A3.5 3.5 0 0 1 4.5 8Z" />
      </svg>
    )
  }
  if (type === 'Complete') {
    return (
      <svg viewBox="0 0 16 16" width="12" height="12" fill="currentColor" aria-hidden="true">
        <path d="M8 1.5a6.5 6.5 0 1 0 0 13 6.5 6.5 0 0 0 0-13Zm3.03 4.72-3.5 3.5a.75.75 0 0 1-1.06 0l-1.75-1.75a.75.75 0 1 1 1.06-1.06L7.25 8.19l2.97-2.97a.75.75 0 1 1 1.06 1.06Z" />
      </svg>
    )
  }
  if (type === 'Failed') {
    return (
      <svg viewBox="0 0 16 16" width="12" height="12" fill="currentColor" aria-hidden="true">
        <path d="M8 1.5a6.5 6.5 0 1 0 0 13 6.5 6.5 0 0 0 0-13ZM5.28 5.28a.75.75 0 0 1 1.06 0L8 6.94l1.66-1.66a.75.75 0 1 1 1.06 1.06L9.06 8l1.66 1.66a.75.75 0 1 1-1.06 1.06L8 9.06l-1.66 1.66a.75.75 0 1 1-1.06-1.06L6.94 8 5.28 6.34a.75.75 0 0 1 0-1.06Z" />
      </svg>
    )
  }
  return (
    <svg viewBox="0 0 16 16" width="12" height="12" fill="currentColor" aria-hidden="true">
      <circle cx="8" cy="8" r="4" />
    </svg>
  )
}

function ConditionTooltip({ condition, tipId, coords }) {
  return createPortal(
    <div
      className="deployment-condition-tooltip deployment-condition-tooltip-fixed"
      role="tooltip"
      id={tipId}
      style={{ left: coords.x, top: coords.y }}
    >
      <div className="deployment-condition-tooltip-head">
        <strong>{condition.type}</strong>
        <span className={`deployment-condition-status ${conditionTone(condition)}`}>
          {condition.status || 'Unknown'}
        </span>
      </div>
      {condition.reason && (
        <div className="deployment-condition-tooltip-row">
          <span className="deployment-condition-tooltip-k">Reason</span>
          <span className="deployment-condition-tooltip-v">{condition.reason}</span>
        </div>
      )}
      {condition.message && (
        <div className="deployment-condition-tooltip-row">
          <span className="deployment-condition-tooltip-k">Message</span>
          <span className="deployment-condition-tooltip-v">{condition.message}</span>
        </div>
      )}
    </div>,
    document.body,
  )
}

function ConditionIcon({ condition }) {
  const [open, setOpen] = useState(false)
  const [coords, setCoords] = useState({ x: 0, y: 0 })
  const iconRef = useRef(null)
  const tipId = useId()
  const tone = conditionTone(condition)

  function showTooltip() {
    const rect = iconRef.current?.getBoundingClientRect()
    if (rect) {
      setCoords({
        x: rect.left + rect.width / 2,
        y: rect.top - 8,
      })
    }
    setOpen(true)
  }

  return (
    <span
      className="deployment-condition-icon-wrap"
      onMouseEnter={showTooltip}
      onMouseLeave={() => setOpen(false)}
      onFocus={showTooltip}
      onBlur={() => setOpen(false)}
    >
      <span
        ref={iconRef}
        className={`deployment-condition-icon tone-${tone}`}
        aria-describedby={open ? tipId : undefined}
        aria-label={`${condition.type} ${condition.status || 'Unknown'}`}
        title={`${condition.type}: ${condition.status || 'Unknown'}`}
        tabIndex={0}
      >
        {conditionIcon(condition.type)}
      </span>
      {open && <ConditionTooltip condition={condition} tipId={tipId} coords={coords} />}
    </span>
  )
}

export function DeploymentConditionIndicators({ conditions = [] }) {
  if (!conditions?.length) {
    return <span className="deployment-condition-empty">—</span>
  }
  return (
    <div className="deployment-condition-indicators" aria-label="Workload conditions">
      {conditions.map((condition) => (
        <ConditionIcon key={condition.type} condition={condition} />
      ))}
    </div>
  )
}
