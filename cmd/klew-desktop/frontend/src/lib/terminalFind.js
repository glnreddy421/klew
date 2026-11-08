import { terminalFindDecorations } from './terminalAppearance'

/** @typedef {{ findNext: Function, findPrevious: Function, clearDecorations: Function, onResults: Function, countMatches: Function }} TerminalSearchApi */

/** Build xterm SearchAddon options with readable match highlights. */
export function buildTerminalFindOptions(themeId = '', overrides = {}) {
  const base = terminalFindDecorations(themeId)
  const { decorations: overrideDecorations, ...restOverrides } = overrides
  return {
    caseSensitive: false,
    wholeWord: false,
    regex: false,
    decorations: {
      ...base,
      ...overrideDecorations,
    },
    ...restOverrides,
  }
}

export function formatFindResultSummary(resultIndex, resultCount) {
  if (!resultCount) return 'No results'
  if (resultIndex < 0) return `${resultCount} matches`
  return `${resultIndex + 1} of ${resultCount}`
}

/** Fallback match count by scanning the xterm buffer (includes scrollback). */
export function countTerminalMatches(term, query, { caseSensitive = false } = {}) {
  if (!term || !query) return 0
  const needle = caseSensitive ? query : query.toLowerCase()
  const buffer = term.buffer.active
  let count = 0
  for (let y = 0; y < buffer.length; y++) {
    const line = buffer.getLine(y)?.translateToString(true) || ''
    const hay = caseSensitive ? line : line.toLowerCase()
    let idx = 0
    while ((idx = hay.indexOf(needle, idx)) !== -1) {
      count += 1
      idx += needle.length
    }
  }
  return count
}

/**
 * Run an xterm search call; a throw (e.g. addon misconfiguration) degrades to
 * "no match" instead of escaping the find input's change handler.
 */
export function runTerminalSearch(fn) {
  try {
    return fn()
  } catch (err) {
    console.error('terminal find failed', err)
    return false
  }
}
