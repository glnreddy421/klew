/** Shapes Settings → Extensions data from the engine for display. */

function baseName(path) {
  return String(path || '').split(/[\\/]/).pop()
}

/** Extension documents grouped by file, in folder then file order. */
export function entriesByFile(entries) {
  const byPath = new Map()
  for (const e of Array.isArray(entries) ? entries : []) {
    if (!byPath.has(e.file)) {
      byPath.set(e.file, { path: e.file, name: baseName(e.file), folder: e.folder, entries: [] })
    }
    byPath.get(e.file).entries.push(e)
  }
  return [...byPath.values()]
}

// Document and rule problems already show on their entry, and a missing
// linked folder is marked on the folder itself.
const ENTRY_PROBLEM = /^extensions: (.+ document \d+:|rule [^:]+: resource |linked folder .+ was not found$)/

/** Problems not shown elsewhere on the page, such as an unreadable folder. */
export function folderProblems(problems) {
  return (Array.isArray(problems) ? problems : [])
    .filter((p) => !ENTRY_PROBLEM.test(p))
    .map((p) => p.replace(/^extensions: /, ''))
}

const MAX_LINES = 20

/**
 * Per-rule test results: matches (with facts) and errors, listed first and
 * capped; objects that did not match are only counted.
 */
export function testSummary(result) {
  return (result?.runs || []).map((run) => {
    const checks = run.checks || []
    const lines = []
    for (const c of checks) {
      const object = `${c.object?.kind || ''}/${c.object?.name || ''}`
      if (c.error) lines.push({ key: `e:${object}`, kind: 'skipped', object, text: c.error })
      else if (c.matched) {
        const text = [...(c.facts || []), ...(c.factErrors || []).map((f) => `fact failed: ${f}`)].join(' · ')
        lines.push({ key: `m:${object}`, kind: 'match', object, text })
      }
    }
    lines.sort((a, b) => (a.kind === b.kind ? 0 : a.kind === 'match' ? -1 : 1))
    return {
      rule: run.rule,
      resource: run.resource,
      checked: checks.length,
      matched: checks.filter((c) => c.matched).length,
      errors: checks.filter((c) => c.error).length,
      lines: lines.slice(0, MAX_LINES),
      hidden: Math.max(0, lines.length - MAX_LINES),
      warnings: run.warnings || [],
    }
  })
}
