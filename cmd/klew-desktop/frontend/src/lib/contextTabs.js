/**
 * Chrome-style context tabs: the contexts the user has open, in tab order.
 * One context is connected at a time; the list only remembers which are open.
 */

export const MAX_CONTEXT_TABS = 12

export function normalizeContextTabs(list) {
  if (!Array.isArray(list)) return []
  const out = []
  for (const v of list) {
    const name = String(v || '').trim()
    if (name && !out.includes(name)) out.push(name)
  }
  return out.slice(0, MAX_CONTEXT_TABS)
}

/** Add name at the end; when full, drop the oldest tab that is not `keep`. */
export function addContextTab(tabs, name, { keep = '' } = {}) {
  const list = normalizeContextTabs(tabs)
  const n = String(name || '').trim()
  if (!n || list.includes(n)) return list
  const next = [...list, n]
  while (next.length > MAX_CONTEXT_TABS) {
    const drop = next.findIndex((t) => t !== keep && t !== n)
    next.splice(drop < 0 ? 0 : drop, 1)
  }
  return next
}

/**
 * Close name. `next` is the tab to switch to when the closed one was active:
 * the right-hand neighbour, else the left, else '' (no tabs left).
 */
export function closeContextTab(tabs, name, active = '') {
  const list = normalizeContextTabs(tabs)
  const i = list.indexOf(name)
  if (i < 0) return { tabs: list, next: '' }
  const remaining = list.filter((t) => t !== name)
  if (name !== active) return { tabs: remaining, next: '' }
  const next = list[i + 1] || list[i - 1] || ''
  return { tabs: remaining, next }
}

/** Drop tabs for contexts that are no longer in kubeconfig. */
export function pruneContextTabs(tabs, contextNames) {
  const known = new Set(contextNames || [])
  if (!known.size) return normalizeContextTabs(tabs)
  return normalizeContextTabs(tabs).filter((t) => known.has(t))
}
