import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  clampSize,
  maxExplorerWidth,
  maxInspectorHeight,
  maxInspectorWidth,
  openInspectorPaneSize,
  runPointerDragResize,
} from './paneDragResize.js'

describe('paneDragResize', () => {
  it('clamps size', () => {
    expect(clampSize(50, 100, 200)).toBe(100)
    expect(clampSize(250, 100, 200)).toBe(200)
    expect(clampSize(150, 100, 200)).toBe(150)
  })

  it('caps inspector and explorer to viewport fractions', () => {
    expect(maxInspectorWidth(1200)).toBe(840)
    expect(maxExplorerWidth(1200)).toBe(504)
  })

  it('opens the inspector as a side panel, within the drag cap', () => {
    expect(openInspectorPaneSize('right', { innerWidth: 1280, innerHeight: 800 })).toBe(512)
    expect(openInspectorPaneSize('right', { innerWidth: 2560, innerHeight: 1440 })).toBe(720)
    expect(openInspectorPaneSize('right', { innerWidth: 960, innerHeight: 640 })).toBe(440)
    expect(openInspectorPaneSize('bottom', { innerWidth: 1280, innerHeight: 900 })).toBe(360)
    for (const vw of [600, 960, 1280, 2560]) {
      expect(openInspectorPaneSize('right', { innerWidth: vw, innerHeight: 800 })).toBeLessThanOrEqual(maxInspectorWidth(vw))
    }
  })
})

describe('runPointerDragResize', () => {
  afterEach(() => vi.unstubAllGlobals())

  function startDrag(onEnd) {
    vi.stubGlobal('window', new EventTarget())
    vi.stubGlobal('document', { body: { style: {} } })
    const trigger = {
      pointerId: 7,
      preventDefault() {},
      currentTarget: { setPointerCapture() {}, releasePointerCapture() {} },
    }
    runPointerDragResize(trigger, { onMove() {}, onEnd })
  }

  function fire(type) {
    window.dispatchEvent(Object.assign(new Event(type), { pointerId: 7 }))
  }

  it('reports a normal release as not cancelled', () => {
    const onEnd = vi.fn()
    startDrag(onEnd)
    fire('pointerup')
    expect(onEnd).toHaveBeenCalledWith({ cancelled: false })
  })

  it('reports pointercancel so it is never treated as a click', () => {
    const onEnd = vi.fn()
    startDrag(onEnd)
    fire('pointercancel')
    expect(onEnd).toHaveBeenCalledWith({ cancelled: true })
    fire('pointerup')
    expect(onEnd).toHaveBeenCalledTimes(1)
  })
})
