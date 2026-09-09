/** @typedef {'horizontal' | 'vertical'} TerminalSplitOrientation */
/** @typedef {{ orientation: TerminalSplitOrientation, firstId: string, secondId: string, focusId: string }} TerminalSplitView */

/** Pick the tab to show beside anchorTabId in a split view. */
export function resolveSplitPartner(tabs, anchorTabId, activeTabId) {
  if (!Array.isArray(tabs) || tabs.length < 2) return null
  const anchor = String(anchorTabId || '')
  if (activeTabId && activeTabId !== anchor) return activeTabId
  return tabs.find((t) => t.id !== anchor)?.id || null
}

/** @returns {TerminalSplitView | null} */
export function createSplitView(orientation, anchorTabId, partnerTabId) {
  if (!anchorTabId || !partnerTabId || anchorTabId === partnerTabId) return null
  return {
    orientation,
    firstId: anchorTabId,
    secondId: partnerTabId,
    focusId: anchorTabId,
  }
}

export function splitIncludesTab(splitView, tabId) {
  if (!splitView || !tabId) return false
  return splitView.firstId === tabId || splitView.secondId === tabId
}

export function isTerminalPaneVisible(splitView, tabId, activeTabId) {
  if (splitView) return splitIncludesTab(splitView, tabId)
  return tabId === activeTabId
}
