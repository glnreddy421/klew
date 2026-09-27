/**
 * Favorite kube contexts (many) vs default-on-launch context (one).
 */

export function normalizeFavoriteContexts(list) {
  if (!Array.isArray(list)) return []
  const seen = new Set()
  const out = []
  for (const raw of list) {
    const name = String(raw || '').trim()
    if (!name || seen.has(name)) continue
    seen.add(name)
    out.push(name)
  }
  return out
}

/** @param {string[]} favorites */
export function toggleFavoriteContext(favorites, name) {
  const n = String(name || '').trim()
  if (!n) return normalizeFavoriteContexts(favorites)
  const list = normalizeFavoriteContexts(favorites)
  const idx = list.indexOf(n)
  if (idx >= 0) {
    return list.filter((_, i) => i !== idx)
  }
  return [...list, n]
}

export function isFavoriteContext(favorites, name) {
  const n = String(name || '').trim()
  if (!n) return false
  return normalizeFavoriteContexts(favorites).includes(n)
}

/**
 * Favorites first (saved order), then remaining contexts A→Z.
 * @param {{ name: string }[]} contexts
 */
export function orderContextsForDisplay(contexts, { favoriteContexts = [] } = {}) {
  if (!Array.isArray(contexts) || contexts.length === 0) return []
  const byName = new Map(contexts.map((c) => [c.name, c]))
  const favOrder = normalizeFavoriteContexts(favoriteContexts).filter((n) => byName.has(n))
  const favSet = new Set(favOrder)
  const rest = contexts
    .filter((c) => !favSet.has(c.name))
    .sort((a, b) => a.name.localeCompare(b.name))
  const favItems = favOrder.map((n) => byName.get(n)).filter(Boolean)
  return [...favItems, ...rest]
}
