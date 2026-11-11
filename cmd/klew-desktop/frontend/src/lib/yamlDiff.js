/**
 * Line diff for the inspector's Compare tab: two YAML documents in, rows of
 * unchanged, removed and added lines out, like `git diff`.
 */

// Above this many cells (lines × lines, after trimming the shared start and
// end) the middle is shown as removed then added instead of being aligned.
const MAX_LCS_CELLS = 4_000_000

function splitLines(text) {
  if (!text) return []
  const lines = String(text).split('\n')
  if (lines[lines.length - 1] === '') lines.pop()
  return lines
}

/**
 * Rows of { type: 'same' | 'del' | 'add', text, oldNo, newNo }. Line numbers
 * are 1-based; oldNo is null on added rows and newNo on removed rows.
 */
export function diffLines(oldText, newText) {
  const a = splitLines(oldText)
  const b = splitLines(newText)
  let start = 0
  while (start < a.length && start < b.length && a[start] === b[start]) start++
  let endA = a.length
  let endB = b.length
  while (endA > start && endB > start && a[endA - 1] === b[endB - 1]) {
    endA--
    endB--
  }

  const rows = []
  for (let i = 0; i < start; i++) rows.push({ type: 'same', text: a[i], oldNo: i + 1, newNo: i + 1 })
  rows.push(...diffMiddle(a, b, start, endA, endB))
  for (let i = endA, j = endB; i < a.length; i++, j++) {
    rows.push({ type: 'same', text: a[i], oldNo: i + 1, newNo: j + 1 })
  }
  return rows
}

function diffMiddle(a, b, start, endA, endB) {
  const n = endA - start
  const m = endB - start
  const rows = []
  const del = (i) => rows.push({ type: 'del', text: a[i], oldNo: i + 1, newNo: null })
  const add = (j) => rows.push({ type: 'add', text: b[j], oldNo: null, newNo: j + 1 })
  if (n === 0 || m === 0 || (n + 1) * (m + 1) > MAX_LCS_CELLS) {
    for (let i = start; i < endA; i++) del(i)
    for (let j = start; j < endB; j++) add(j)
    return rows
  }
  // lcs[i][j] = longest common subsequence of a[start+i..] and b[start+j..].
  const w = m + 1
  const lcs = new Int32Array((n + 1) * w)
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      lcs[i * w + j] = a[start + i] === b[start + j]
        ? lcs[(i + 1) * w + j + 1] + 1
        : Math.max(lcs[(i + 1) * w + j], lcs[i * w + j + 1])
    }
  }
  let i = 0
  let j = 0
  while (i < n && j < m) {
    if (a[start + i] === b[start + j]) {
      rows.push({ type: 'same', text: a[start + i], oldNo: start + i + 1, newNo: start + j + 1 })
      i++
      j++
    } else if (lcs[(i + 1) * w + j] >= lcs[i * w + j + 1]) {
      del(start + i)
      i++
    } else {
      add(start + j)
      j++
    }
  }
  for (; i < n; i++) del(start + i)
  for (; j < m; j++) add(start + j)
  return rows
}

/** { added, removed } line counts. */
export function diffStats(rows) {
  let added = 0
  let removed = 0
  for (const r of rows || []) {
    if (r.type === 'add') added++
    else if (r.type === 'del') removed++
  }
  return { added, removed }
}

/**
 * Groups rows for display: changed lines with `context` unchanged lines
 * around them, and the unchanged runs between as { kind: 'gap', rows }.
 * A gap shorter than three lines is shown rather than folded.
 */
export function foldUnchanged(rows, context = 3) {
  const keep = new Array(rows.length).fill(false)
  rows.forEach((r, i) => {
    if (r.type === 'same') return
    for (let k = Math.max(0, i - context); k <= Math.min(rows.length - 1, i + context); k++) keep[k] = true
  })
  const out = []
  let i = 0
  while (i < rows.length) {
    const start = i
    const shown = keep[i]
    while (i < rows.length && keep[i] === shown) i++
    const run = rows.slice(start, i)
    if (shown || run.length < 3) {
      const last = out[out.length - 1]
      if (last?.kind === 'rows') last.rows.push(...run)
      else out.push({ kind: 'rows', rows: run })
    } else {
      out.push({ kind: 'gap', rows: run })
    }
  }
  return out
}

/** Revisions the Compare tab can offer, newest first: those with a revision number. */
export function compareOptions(workloadChanges) {
  const seen = new Set()
  const out = []
  for (const c of workloadChanges?.changes || []) {
    if (!c?.revision || seen.has(c.revision)) continue
    seen.add(c.revision)
    out.push(c)
  }
  return out
}

/**
 * The pair to show first: the one asked for when both revisions still
 * exist, otherwise the newest revision against the one before it.
 */
export function defaultPair(options, wanted) {
  const has = (r) => options.some((o) => o.revision === r)
  if (wanted && has(wanted.from) && has(wanted.to)) return { from: wanted.from, to: wanted.to }
  if (options.length < 2) return null
  return { from: options[1].revision, to: options[0].revision }
}
