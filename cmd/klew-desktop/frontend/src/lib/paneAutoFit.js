import { clampSize } from './paneDragResize.js'

/** Estimate pixels needed to show a single-line string in UI chrome. */
export function estimateSingleLineWidth(text, {
  min = 120,
  max = 720,
  padding = 72,
  charPx = 7.15,
} = {}) {
  const len = String(text || '').length
  if (len === 0) return min
  return clampSize(Math.ceil(len * charPx + padding), min, max)
}

export function longestString(values = []) {
  return values.reduce((best, v) => {
    const s = String(v || '')
    return s.length > best.length ? s : best
  }, '')
}

export function resourceNameColumnWidth(entities = [], { min = 144, max = 560 } = {}) {
  const longest = longestString(entities.map((e) => e?.name))
  return estimateSingleLineWidth(longest, { min, max, padding: 56, charPx: 7.1 })
}

export function explorerWidthForLabels(categories = [], { min = 180, max = 560 } = {}) {
  const labels = []
  for (const cat of categories) {
    if (cat?.label) labels.push(cat.label)
    for (const k of cat?.kinds || []) {
      if (k?.label) labels.push(k.label)
    }
  }
  const longest = longestString(labels)
  return estimateSingleLineWidth(longest, { min, max, padding: 64, charPx: 6.9 })
}

/** Auto-fit never makes the explorer wider than this; dragging can go further. */
export const EXPLORER_AUTO_MAX = 320

/** Share of nav labels shown in full; longer outliers ellipsize (rows have tooltips). */
export const EXPLORER_FIT_PERCENTILE = 0.9

export function percentile(values = [], p = 0.5) {
  if (!values.length) return null
  const sorted = [...values].sort((a, b) => a - b)
  const idx = Math.min(sorted.length - 1, Math.max(0, Math.ceil(p * sorted.length) - 1))
  return sorted[idx]
}

/** Explorer width at which a row's label shows in full. Labels flex; the rest of the row is fixed. */
export function explorerWidthForRow({ explorerWidth, labelWidth, textWidth }) {
  return explorerWidth - labelWidth + textWidth
}

/** Measures label text with a canvas using an element's computed font — no layout. */
function createTextMeasurer(doc) {
  const ctx = doc.createElement('canvas').getContext('2d')
  return (text, styleEl) => {
    const cs = doc.defaultView.getComputedStyle(styleEl)
    ctx.font = `${cs.fontStyle} ${cs.fontWeight} ${cs.fontSize} ${cs.fontFamily}`
    const shown = cs.textTransform === 'uppercase' ? String(text).toUpperCase() : String(text)
    const spacing = Number.parseFloat(cs.letterSpacing) || 0
    return ctx.measureText(shown).width + spacing * shown.length
  }
}

/**
 * Explorer width that shows most resource-nav labels in full. Every category's
 * labels count (from data, not the DOM — collapsed groups are not rendered), so
 * opening another group never changes the width; a few very long CRD names may
 * ellipsize. One rendered row per label type supplies font and row chrome.
 * Null when the explorer is collapsed or not mounted.
 */
export function measureExplorerFitWidth(doc = document, categories = [], {
  min = 180,
  max = EXPLORER_AUTO_MAX,
  coverage = EXPLORER_FIT_PERCENTILE,
} = {}) {
  const explorer = doc.querySelector('.context-explorer:not(.context-explorer-collapsed)')
  const groupSample = explorer?.querySelector('.resource-nav-group-label')
  if (!groupSample || !categories.length) return null
  const kindSample = explorer.querySelector('.resource-nav-kind-label')
  const explorerWidth = explorer.getBoundingClientRect().width
  const measure = createTextMeasurer(doc)

  const needed = []
  const groupLabelWidth = groupSample.getBoundingClientRect().width
  for (const cat of categories) {
    if (cat?.label) {
      needed.push(explorerWidthForRow({ explorerWidth, labelWidth: groupLabelWidth, textWidth: measure(cat.label, groupSample) }))
    }
  }
  if (kindSample) {
    const kindLabelWidth = kindSample.getBoundingClientRect().width
    for (const cat of categories) {
      for (const kind of cat?.kinds || []) {
        if (kind?.label) {
          needed.push(explorerWidthForRow({ explorerWidth, labelWidth: kindLabelWidth, textWidth: measure(kind.label, kindSample) }))
        }
      }
    }
  }
  // +4 so a label that exactly fits never shows a premature ellipsis.
  return clampSize(Math.ceil(percentile(needed, coverage) + 4), min, max)
}
