import { useId, useRef, useState } from 'react'
import { createPortal } from 'react-dom'

function containerTone(container) {
  if (container.state === 'terminated' && container.reason === 'Completed') {
    return 'running'
  }
  if (container.state === 'running' && container.ready) return 'running'
  if (container.state === 'waiting') return 'waiting'
  if (container.state === 'terminated') return 'terminated'
  if (container.state === 'running' && !container.ready) return 'waiting'
  return 'unknown'
}

function containerStateLabel(container) {
  if (container.state === 'terminated' && container.reason === 'Completed') {
    return 'completed'
  }
  return container.state || 'unknown'
}

function ContainerTooltip({ container, tipId, coords }) {
  const stateLabel = containerStateLabel(container)
  const reason = container.reason || container.lastReason || ''
  const message = container.message || ''

  const body = (
    <div
      className="container-status-tooltip container-status-tooltip-fixed"
      role="tooltip"
      id={tipId}
      style={{ left: coords.x, top: coords.y }}
    >
      <div className="container-status-tooltip-head">
        <strong>{container.name}</strong>
        <span className="muted">{stateLabel}</span>
      </div>
      {reason && (
        <div className="container-status-tooltip-row">
          <span className="container-status-tooltip-k">Reason</span>
          <span className="container-status-tooltip-v">{reason}</span>
        </div>
      )}
      {message && (
        <div className="container-status-tooltip-row">
          <span className="container-status-tooltip-k">Message</span>
          <span className="container-status-tooltip-v">{message}</span>
        </div>
      )}
      {!reason && !message && container.init && (
        <div className="container-status-tooltip-row muted">Init container</div>
      )}
    </div>
  )

  return createPortal(body, document.body)
}

function ContainerLed({ container }) {
  const [open, setOpen] = useState(false)
  const [coords, setCoords] = useState({ x: 0, y: 0 })
  const ledRef = useRef(null)
  const tipId = useId()
  const tone = containerTone(container)

  function showTooltip() {
    const rect = ledRef.current?.getBoundingClientRect()
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
      className="container-status-led-wrap"
      onMouseEnter={showTooltip}
      onMouseLeave={() => setOpen(false)}
      onFocus={showTooltip}
      onBlur={() => setOpen(false)}
    >
      <span
        ref={ledRef}
        className={`container-status-led tone-${tone}`}
        aria-describedby={open ? tipId : undefined}
        tabIndex={0}
        title={`${container.name} ${containerStateLabel(container)}`}
      />
      {open && <ContainerTooltip container={container} tipId={tipId} coords={coords} />}
    </span>
  )
}

export function ContainerStatusIndicators({ containers = [] }) {
  if (!containers.length) {
    return <span className="container-status-empty">—</span>
  }
  return (
    <div className="container-status-indicators" aria-label="Container status">
      {containers.map((container) => (
        <ContainerLed key={`${container.init ? 'init' : 'app'}-${container.name}`} container={container} />
      ))}
    </div>
  )
}
