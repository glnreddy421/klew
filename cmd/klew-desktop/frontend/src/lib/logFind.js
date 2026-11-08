/**
 * Plain-text find in log lines (case-insensitive substring, like default IDE search).
 */

/**
 * @param {string} text
 * @param {string} query
 * @returns {{ start: number, end: number }[]}
 */
export function findSubstringRanges(text, query) {
  const t = String(text ?? '')
  const q = String(query ?? '').trim()
  if (!q) return []
  const lower = t.toLowerCase()
  const needle = q.toLowerCase()
  const out = []
  let i = 0
  while (i < lower.length) {
    const j = lower.indexOf(needle, i)
    if (j === -1) break
    out.push({ start: j, end: j + needle.length })
    i = j + Math.max(1, needle.length)
  }
  return out
}

/**
 * @template T
 * @param {T[]} lines
 * @param {(row: T) => string} getText
 * @param {string} query
 * @returns {{ row: T, start: number, end: number }[]}
 */
export function buildLogFindMatches(lines, getText, query) {
  const q = String(query ?? '').trim()
  if (!q) return []
  const flat = []
  for (const row of lines) {
    const text = getText(row)
    for (const { start, end } of findSubstringRanges(text, q)) {
      flat.push({ row, start, end })
    }
  }
  return flat
}
