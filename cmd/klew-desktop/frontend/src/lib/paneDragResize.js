/**
 * Pointer-based pane resize (works with mouse, trackpad, pen).
 */

export function clampSize(value, min, max) {
  return Math.min(max, Math.max(min, value))
}

export function runPointerDragResize(triggerEvent, { axis = 'x', onMove, onEnd, cursor }) {
  triggerEvent.preventDefault()
  const target = triggerEvent.currentTarget
  const pointerId = triggerEvent.pointerId
  if (target.setPointerCapture) {
    try {
      target.setPointerCapture(pointerId)
    } catch {
      // ignore
    }
  }

  const cursorStyle = cursor || (axis === 'x' ? 'col-resize' : 'row-resize')
  document.body.style.cursor = cursorStyle
  document.body.style.userSelect = 'none'

  // Listen on window, not the handle: the handle may unmount mid-drag (e.g. the
  // inspector shutter), which drops capture and would strand cursor/userSelect.
  function cleanup() {
    document.body.style.cursor = ''
    document.body.style.userSelect = ''
    window.removeEventListener('pointermove', onPointerMove)
    window.removeEventListener('pointerup', onPointerUp)
    window.removeEventListener('pointercancel', onPointerUp)
    try {
      target.releasePointerCapture?.(pointerId)
    } catch {
      // ignore
    }
  }

  function onPointerMove(ev) {
    if (ev.pointerId !== pointerId) return
    onMove(ev)
  }

  // onEnd receives { cancelled } so callers can tell a real release from the
  // OS cancelling the gesture (which must not count as a click).
  function onPointerUp(ev) {
    if (ev.pointerId !== pointerId) return
    cleanup()
    onEnd?.({ cancelled: ev.type === 'pointercancel' })
  }

  window.addEventListener('pointermove', onPointerMove)
  window.addEventListener('pointerup', onPointerUp)
  window.addEventListener('pointercancel', onPointerUp)
}

/** Horizontal resize from a vertical handle (explorer: drag right = wider). */
export function runPointerColResize(triggerEvent, { startX, startSize, onSize, onEnd, min, max, invert = false }) {
  runPointerDragResize(triggerEvent, {
    axis: 'x',
    onMove: (ev) => {
      const delta = ev.clientX - startX
      const next = startSize + (invert ? -delta : delta)
      onSize(clampSize(next, min, max))
    },
    onEnd,
  })
}

/** Vertical resize from a horizontal handle. */
export function runPointerRowResize(triggerEvent, { startY, startSize, onSize, onEnd, min, max, invert = false }) {
  runPointerDragResize(triggerEvent, {
    axis: 'y',
    onMove: (ev) => {
      const delta = ev.clientY - startY
      const next = startSize + (invert ? -delta : delta)
      onSize(clampSize(next, min, max))
    },
    onEnd,
  })
}

/** Hard cap on inspector size (drag limit), as a fraction of the viewport axis. */
export const INSPECTOR_MAX_SCREEN_FRACTION = 0.7

/** Default size when the inspector opens: a side panel, not a takeover. */
export const INSPECTOR_OPEN_WIDTH_FRACTION = 0.4
export const INSPECTOR_OPEN_WIDTH_MIN = 440
export const INSPECTOR_OPEN_WIDTH_MAX = 720
export const INSPECTOR_OPEN_HEIGHT_FRACTION = 0.4
export const INSPECTOR_OPEN_HEIGHT_MIN = 260

/** Largest inspector width (right placement). */
export function maxInspectorWidth(viewportWidth = window.innerWidth) {
  return Math.floor((viewportWidth || 1280) * INSPECTOR_MAX_SCREEN_FRACTION)
}

/** Largest inspector height (bottom placement). */
export function maxInspectorHeight(viewportHeight = window.innerHeight) {
  return Math.floor((viewportHeight || 800) * INSPECTOR_MAX_SCREEN_FRACTION)
}

/** Inspector size when opened (row select, shutter click, toggle). Never exceeds the drag cap. */
export function openInspectorPaneSize(placement = 'right', viewport = window) {
  if (placement === 'bottom') {
    const vh = viewport.innerHeight || 800
    const h = clampSize(Math.floor(vh * INSPECTOR_OPEN_HEIGHT_FRACTION), INSPECTOR_OPEN_HEIGHT_MIN, Infinity)
    return Math.min(h, maxInspectorHeight(vh))
  }
  const vw = viewport.innerWidth || 1280
  const w = clampSize(Math.floor(vw * INSPECTOR_OPEN_WIDTH_FRACTION), INSPECTOR_OPEN_WIDTH_MIN, INSPECTOR_OPEN_WIDTH_MAX)
  return Math.min(w, maxInspectorWidth(vw))
}

export function maxExplorerWidth(viewportWidth = window.innerWidth) {
  return Math.min(560, Math.floor(viewportWidth * 0.42))
}
