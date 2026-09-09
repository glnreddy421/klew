/** @typedef {{ findNext: Function, findPrevious: Function, clearDecorations: Function, onResults: Function, countMatches: Function }} TerminalSearchApi */

const FIND_DECORATIONS = {
  midnight: {
    matchBackground: '#1a3050',
    matchBorder: '#58a6ff',
    matchOverviewRuler: '#58a6ff',
    activeMatchBackground: '#264a73',
    activeMatchBorder: '#79b8ff',
    activeMatchColorOverviewRuler: '#79b8ff',
  },
  black: {
    matchBackground: '#2a2a2a',
    matchBorder: '#ffffff',
    matchOverviewRuler: '#ffffff',
    activeMatchBackground: '#3d3d3d',
    activeMatchBorder: '#ffffff',
    activeMatchColorOverviewRuler: '#ffffff',
  },
  silver: {
    matchBackground: '#1e2128',
    matchBorder: '#d4dae3',
    matchOverviewRuler: '#d4dae3',
    activeMatchBackground: '#2a3038',
    activeMatchBorder: '#e8ecf1',
    activeMatchColorOverviewRuler: '#e8ecf1',
  },
}

/** Build xterm SearchAddon options with readable match highlights. */
export function buildTerminalFindOptions(appearanceId, overrides = {}) {
  const id = FIND_DECORATIONS[appearanceId] ? appearanceId : 'midnight'
  const { decorations: overrideDecorations, ...restOverrides } = overrides
  return {
    caseSensitive: false,
    wholeWord: false,
    regex: false,
    decorations: {
      ...FIND_DECORATIONS[id],
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
