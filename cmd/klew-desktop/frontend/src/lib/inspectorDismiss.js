/**
 * Click-outside rules for the shell inspector.
 *
 * Only clicks inside the workspace dismiss it. Clicks in the explorer (it drives
 * what is inspected, and can yield space to the inspector), the top bar,
 * activity rail, console dock, modals, and portaled popovers are ignored, as are
 * elements that opt out with `data-inspector-keep` (resource rows that choose
 * what to inspect, the inspector toggle).
 */

export const INSPECTOR_DISMISS_SCOPE = '.app-shell-panes'

export const INSPECTOR_KEEP_SELECTOR = [
  '.app-inspector',
  '.app-inspector-shutter',
  '.pane-resize-handle',
  '.context-explorer',
  '[data-inspector-keep]',
].join(', ')

export function shouldDismissInspector(target) {
  if (!target || typeof target.closest !== 'function') return false
  if (!target.closest(INSPECTOR_DISMISS_SCOPE)) return false
  return !target.closest(INSPECTOR_KEEP_SELECTOR)
}
