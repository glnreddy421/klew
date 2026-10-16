import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  RAIL_MODE,
  RAIL_WIDTH,
  SHELL_LAYOUT_KEY,
  loadShellLayout,
  nextRailMode,
  resolveRailCollapsed,
  shouldAutoCollapseExplorer,
} from './shellLayout.js'

function stubStorage(initial = {}) {
  const store = { ...initial }
  vi.stubGlobal('localStorage', {
    getItem: (k) => (k in store ? store[k] : null),
    setItem: (k, v) => { store[k] = String(v) },
    removeItem: (k) => { delete store[k] },
  })
  return store
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('rail mode', () => {
  it('auto is icon-only beside an explorer and labelled elsewhere', () => {
    expect(resolveRailCollapsed(RAIL_MODE.AUTO, true)).toBe(true)
    expect(resolveRailCollapsed(RAIL_MODE.AUTO, false)).toBe(false)
  })

  it('explicit modes ignore context', () => {
    expect(resolveRailCollapsed(RAIL_MODE.EXPANDED, true)).toBe(false)
    expect(resolveRailCollapsed(RAIL_MODE.COLLAPSED, false)).toBe(true)
  })

  it('toggle flips the visible state and returns to auto when that matches', () => {
    // Beside an explorer, auto is collapsed → toggle expands explicitly.
    expect(nextRailMode(RAIL_MODE.AUTO, true)).toBe(RAIL_MODE.EXPANDED)
    // Toggling back collapses, which auto would do anyway → auto.
    expect(nextRailMode(RAIL_MODE.EXPANDED, true)).toBe(RAIL_MODE.AUTO)
    // Without an explorer, auto is expanded → toggle collapses explicitly.
    expect(nextRailMode(RAIL_MODE.AUTO, false)).toBe(RAIL_MODE.COLLAPSED)
    expect(nextRailMode(RAIL_MODE.COLLAPSED, false)).toBe(RAIL_MODE.AUTO)
  })

  it('migrates the legacy railCollapsed boolean', () => {
    stubStorage({ [SHELL_LAYOUT_KEY]: JSON.stringify({ railCollapsed: true }) })
    const layout = loadShellLayout()
    expect(layout.railMode).toBe(RAIL_MODE.COLLAPSED)
    expect('railCollapsed' in layout).toBe(false)

    stubStorage({ [SHELL_LAYOUT_KEY]: JSON.stringify({ railCollapsed: false }) })
    expect(loadShellLayout().railMode).toBe(RAIL_MODE.AUTO)
  })

  it('defaults to auto', () => {
    stubStorage()
    expect(loadShellLayout().railMode).toBe(RAIL_MODE.AUTO)
  })
})

describe('shouldAutoCollapseExplorer', () => {
  const base = { railWidth: RAIL_WIDTH.collapsed, explorerWidth: 240, inspectorWidth: 640 }

  it('collapses when the workspace would drop below the minimum', () => {
    expect(shouldAutoCollapseExplorer({ ...base, viewportWidth: 1280 })).toBe(true)
  })

  it('keeps the explorer when there is room', () => {
    expect(shouldAutoCollapseExplorer({ ...base, viewportWidth: 1600 })).toBe(false)
  })
})
