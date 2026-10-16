import { describe, expect, it } from 'vitest'
import { shouldDismissInspector } from './inspectorDismiss.js'

/** Minimal Element stand-in: `closest` matches any selector part in `ancestors`. */
function el(...ancestors) {
  return {
    closest(selector) {
      const parts = selector.split(',').map((s) => s.trim())
      return parts.some((p) => ancestors.includes(p)) ? {} : null
    },
  }
}

describe('shouldDismissInspector', () => {
  it('dismisses on plain workspace clicks', () => {
    expect(shouldDismissInspector(el('.app-shell-panes'))).toBe(true)
  })

  it('keeps open for the inspector, its handles, and opt-in elements', () => {
    for (const keep of ['.app-inspector', '.app-inspector-shutter', '.pane-resize-handle', '.context-explorer', '[data-inspector-keep]']) {
      expect(shouldDismissInspector(el('.app-shell-panes', keep))).toBe(false)
    }
  })

  it('ignores clicks outside the pane area (top bar, portals, modals)', () => {
    expect(shouldDismissInspector(el('.app-topbar'))).toBe(false)
    expect(shouldDismissInspector(el())).toBe(false)
  })

  it('ignores non-element targets', () => {
    expect(shouldDismissInspector(null)).toBe(false)
    expect(shouldDismissInspector({})).toBe(false)
  })
})
