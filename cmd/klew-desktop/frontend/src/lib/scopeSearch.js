/** Case-insensitive substring filter for picker lists. */
export function filterBySubstring(items, query, getText) {
  const q = String(query || '').trim().toLowerCase()
  if (!q) return items
  return items.filter((item) => getText(item).toLowerCase().includes(q))
}

/** Filter investigation matches by free-text and optional kind. */
export function filterScopeMatches(matches, { text = '', kind = '' } = {}) {
  const list = Array.isArray(matches) ? matches : []
  const q = String(text || '').trim().toLowerCase()
  const kindFilter = String(kind || '').trim()
  return list.filter((m) => {
    if (kindFilter && m.ref?.kind !== kindFilter) return false
    if (!q) return true
    const name = String(m.ref?.name || '').toLowerCase()
    const kindName = String(m.ref?.kind || '').toLowerCase()
    const key = `${kindName}/${name}`
    return name.includes(q) || kindName.includes(q) || key.includes(q)
  })
}

/** Distinct kinds present in matches, workload kinds first. */
export function kindFiltersForMatches(matches) {
  const kinds = new Set()
  for (const m of matches || []) {
    if (m.ref?.kind) kinds.add(m.ref.kind)
  }
  const workload = ['Deployment', 'StatefulSet', 'DaemonSet', 'Job', 'CronJob']
  const ordered = []
  for (const k of workload) {
    if (kinds.has(k)) ordered.push(k)
  }
  const rest = [...kinds].filter((k) => !workload.includes(k)).sort()
  return [...ordered, ...rest]
}
