/**
 * Small fuzzy matcher for pickers: the query's characters must appear in the
 * text in order, case-insensitively. Matches at word starts and runs of
 * consecutive characters score higher, so "bil" ranks "billing" above
 * "mobile-lb" and "svc" finds "Service".
 */

const SEPARATORS = new Set(['-', '_', '.', '/', ':', ' '])

function isWordStart(text, i) {
  if (i === 0) return true
  const prev = text[i - 1]
  if (SEPARATORS.has(prev)) return true
  const ch = text[i]
  return ch !== ch.toLowerCase() && prev === prev.toLowerCase() && prev !== prev.toUpperCase()
}

/** Greedy match of query in text starting at start; null when it does not fit. */
function matchFrom(query, text, lower, start) {
  const indices = []
  let score = 0
  let ti = start
  let prev = -2
  for (const qc of query) {
    while (ti < lower.length && lower[ti] !== qc) ti++
    if (ti >= lower.length) return null
    score += 1
    if (ti === prev + 1) score += 5
    else if (prev >= 0) score -= Math.min(ti - prev - 1, 6) * 0.25
    if (isWordStart(text, ti)) score += 8
    indices.push(ti)
    prev = ti
    ti++
  }
  if (indices[0] === 0) score += 4
  else if (!isWordStart(text, indices[0])) score -= 6
  if (indices.length === text.length) score += 6
  score -= (text.length - indices.length) * 0.05 // closer, shorter matches first
  return { score, indices }
}

/**
 * Scores text against one query token. Returns { score, indices } with the
 * matched character positions, or null when the token does not match.
 */
export function fuzzyMatch(query, text) {
  const q = String(query || '').toLowerCase()
  const t = String(text || '')
  if (!q) return { score: 0, indices: [] }
  const lower = t.toLowerCase()
  let best = null
  for (let i = lower.indexOf(q[0]); i !== -1; i = lower.indexOf(q[0], i + 1)) {
    const m = matchFrom(q, t, lower, i)
    if (!m) break
    if (!best || m.score > best.score) best = m
  }
  return best
}

/**
 * Matches a picker query against named fields, e.g. { name, kind }. Every
 * whitespace-separated token must match some field; "kind/name" tokens match
 * both parts. Earlier fields are preferred, weighted by the given order.
 * Returns { score, hits: { field: Set<index> } } or null.
 */
export function fuzzyMatchFields(query, fields) {
  const tokens = String(query || '').trim().split(/\s+/).filter(Boolean)
  const keys = Object.keys(fields)
  const hits = Object.fromEntries(keys.map((k) => [k, new Set()]))
  let score = 0
  for (const token of tokens) {
    const m = matchToken(token, fields, keys)
    if (!m) return null
    score += m.score
    for (const [field, idx] of m.parts) for (const i of idx) hits[field].add(i)
  }
  return { score, hits }
}

function matchToken(token, fields, keys) {
  const slash = token.indexOf('/')
  if (slash > 0 && keys.length >= 2) {
    const [nameKey, kindKey] = keys
    const kind = fuzzyMatch(token.slice(0, slash), fields[kindKey])
    const rest = token.slice(slash + 1)
    const name = rest ? fuzzyMatch(rest, fields[nameKey]) : { score: 0, indices: [] }
    if (kind && name) {
      return { score: kind.score + name.score, parts: [[kindKey, kind.indices], [nameKey, name.indices]] }
    }
  }
  let best = null
  keys.forEach((key, order) => {
    const m = fuzzyMatch(token, fields[key])
    if (!m) return
    const score = m.score - order * 3
    if (!best || score > best.score) best = { score, parts: [[key, m.indices]] }
  })
  return best
}

/**
 * Splits text into runs for highlighting: [{ text, hit }]. indices is a Set
 * or array of matched character positions.
 */
export function highlightRuns(text, indices) {
  const s = String(text || '')
  const set = indices instanceof Set ? indices : new Set(indices || [])
  if (!set.size) return [{ text: s, hit: false }]
  const runs = []
  for (let i = 0; i < s.length; i++) {
    const hit = set.has(i)
    const last = runs[runs.length - 1]
    if (last && last.hit === hit) last.text += s[i]
    else runs.push({ text: s[i], hit })
  }
  return runs
}
