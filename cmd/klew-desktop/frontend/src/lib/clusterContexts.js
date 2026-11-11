/**
 * Favorite kube contexts (many), launch-default context (one), and user tags.
 */

export const MAX_CONTEXT_TAGS = 8
export const MAX_CONTEXT_TAG_LEN = 24
/** Soft display cap for context labels in tabs / tiles (CSS also ellipsizes). */
export const CONTEXT_LABEL_SOFT_MAX = 28
export const MAX_CONTEXT_ALIAS_LEN = 48

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

/** Contexts opted into local event recording (independent of Home favorites). */
export const normalizeRecordedContexts = normalizeFavoriteContexts
export const isRecordedContext = isFavoriteContext

/** Ensure name is in the recorded list (opening a context opts in). */
export function enableRecordedContext(recorded, name) {
  const n = String(name || '').trim()
  if (!n) return normalizeRecordedContexts(recorded)
  const list = normalizeRecordedContexts(recorded)
  return list.includes(n) ? list : [...list, n]
}

/** Contexts that have successfully connected at least once (sticky Connected). */
export const normalizeConnectedContexts = normalizeFavoriteContexts
export const isConnectedContext = isFavoriteContext
export const enableConnectedContext = enableRecordedContext

/**
 * Flip recording for one context. Returns { recordedContexts, recordingOptOutContexts }.
 * Opt-out remembers an explicit "off" so opening the context again does not re-enable it.
 */
export function toggleRecordedContext(recorded, optOut, name) {
  const n = String(name || '').trim()
  const list = normalizeRecordedContexts(recorded)
  const off = normalizeRecordedContexts(optOut)
  if (!n) return { recordedContexts: list, recordingOptOutContexts: off }
  if (list.includes(n)) {
    return {
      recordedContexts: list.filter((x) => x !== n),
      recordingOptOutContexts: off.includes(n) ? off : [...off, n],
    }
  }
  return {
    recordedContexts: [...list, n],
    recordingOptOutContexts: off.filter((x) => x !== n),
  }
}

/** Opening a context enables recording unless the user opted out. */
export function enableRecordingUnlessOptedOut(recorded, optOut, name) {
  const n = String(name || '').trim()
  if (!n) {
    return { recordedContexts: recorded, recordingOptOutContexts: optOut }
  }
  const off = normalizeRecordedContexts(optOut)
  if (off.includes(n) || normalizeRecordedContexts(recorded).includes(n)) {
    // Keep caller refs when nothing changes (avoids prefs churn).
    return { recordedContexts: recorded, recordingOptOutContexts: optOut }
  }
  return {
    recordedContexts: enableRecordedContext(recorded, n),
    recordingOptOutContexts: optOut,
  }
}

/**
 * Connect / display priority:
 * 1. launch default (if present)
 * 2. favorites in saved order (first favorite is the fallback when many)
 * 3. remaining contexts A→Z
 * @param {{ name: string }[]} contexts
 * @returns {string[]}
 */
export function preferredContextNames(contexts, {
  defaultContext = '',
  favoriteContexts = [],
} = {}) {
  if (!Array.isArray(contexts) || contexts.length === 0) return []
  const names = new Set(contexts.map((c) => String(c?.name || '').trim()).filter(Boolean))
  const out = []
  const seen = new Set()
  const push = (raw) => {
    const name = String(raw || '').trim()
    if (!name || !names.has(name) || seen.has(name)) return
    seen.add(name)
    out.push(name)
  }
  push(defaultContext)
  for (const fav of normalizeFavoriteContexts(favoriteContexts)) push(fav)
  for (const c of [...contexts].sort((a, b) => String(a.name || '').localeCompare(String(b.name || '')))) {
    push(c.name)
  }
  return out
}

/**
 * Default first, then favorites (saved order), then remaining A→Z.
 * @param {{ name: string }[]} contexts
 */
export function orderContextsForDisplay(contexts, {
  defaultContext = '',
  favoriteContexts = [],
} = {}) {
  if (!Array.isArray(contexts) || contexts.length === 0) return []
  const byName = new Map(contexts.map((c) => [c.name, c]))
  return preferredContextNames(contexts, { defaultContext, favoriteContexts })
    .map((n) => byName.get(n))
    .filter(Boolean)
}

/** Next context to try after `current` in preferred order (skips alreadyTried). */
export function nextPreferredContext(contexts, {
  defaultContext = '',
  favoriteContexts = [],
  current = '',
  alreadyTried = [],
} = {}) {
  const order = preferredContextNames(contexts, { defaultContext, favoriteContexts })
  if (order.length === 0) return ''
  const tried = new Set(
    [...(Array.isArray(alreadyTried) ? alreadyTried : []), current]
      .map((n) => String(n || '').trim())
      .filter(Boolean),
  )
  for (const name of order) {
    if (!tried.has(name)) return name
  }
  return ''
}

/** Normalize one tag label for storage and matching. */
export function normalizeTag(raw) {
  const s = String(raw || '')
    .trim()
    .toLowerCase()
    .replace(/[\s_]+/g, '-')
    .replace(/[^a-z0-9:./+-]/g, '')
    .replace(/-+/g, '-')
    .replace(/^[-.]+|[-.]+$/g, '')
  if (!s) return ''
  return s.slice(0, MAX_CONTEXT_TAG_LEN)
}

/**
 * @param {Record<string, string[]>|null|undefined} map
 * @returns {Record<string, string[]>}
 */
export function normalizeContextTags(map) {
  if (!map || typeof map !== 'object' || Array.isArray(map)) return {}
  const out = {}
  for (const [rawName, rawTags] of Object.entries(map)) {
    const name = String(rawName || '').trim()
    if (!name || !Array.isArray(rawTags)) continue
    const seen = new Set()
    const tags = []
    for (const raw of rawTags) {
      const tag = normalizeTag(raw)
      if (!tag || seen.has(tag)) continue
      seen.add(tag)
      tags.push(tag)
      if (tags.length >= MAX_CONTEXT_TAGS) break
    }
    if (tags.length > 0) out[name] = tags
  }
  return out
}

/** @param {Record<string, string[]>} map */
export function tagsForContext(map, contextName) {
  const name = String(contextName || '').trim()
  if (!name) return []
  const tags = normalizeContextTags(map)[name]
  return tags ? [...tags] : []
}

/**
 * @param {Record<string, string[]>} map
 * @returns {Record<string, string[]>}
 */
export function addContextTag(map, contextName, rawTag) {
  const name = String(contextName || '').trim()
  const tag = normalizeTag(rawTag)
  if (!name || !tag) return normalizeContextTags(map)
  const next = normalizeContextTags(map)
  const current = next[name] ? [...next[name]] : []
  if (current.includes(tag) || current.length >= MAX_CONTEXT_TAGS) {
    return next
  }
  next[name] = [...current, tag]
  return next
}

/**
 * @param {Record<string, string[]>} map
 * @returns {Record<string, string[]>}
 */
export function removeContextTag(map, contextName, rawTag) {
  const name = String(contextName || '').trim()
  const tag = normalizeTag(rawTag)
  if (!name || !tag) return normalizeContextTags(map)
  const next = normalizeContextTags(map)
  const current = next[name]
  if (!current) return next
  const filtered = current.filter((t) => t !== tag)
  if (filtered.length === 0) {
    delete next[name]
  } else {
    next[name] = filtered
  }
  return next
}

/** Normalize a user alias for a kube context (empty clears). */
export function normalizeAlias(raw) {
  return String(raw || '').trim().replace(/\s+/g, ' ').slice(0, MAX_CONTEXT_ALIAS_LEN)
}

/** @param {unknown} map @returns {Record<string, string>} */
export function normalizeContextAliases(map) {
  if (!map || typeof map !== 'object' || Array.isArray(map)) return {}
  const out = {}
  for (const [rawName, rawAlias] of Object.entries(map)) {
    const name = String(rawName || '').trim()
    const alias = normalizeAlias(rawAlias)
    if (!name || !alias) continue
    out[name] = alias
  }
  return out
}

/** @param {Record<string, string>} map */
export function aliasForContext(map, contextName) {
  const name = String(contextName || '').trim()
  if (!name) return ''
  return normalizeContextAliases(map)[name] || ''
}

/**
 * Set or clear the display alias for a context. Empty alias removes it.
 * @returns {Record<string, string>}
 */
export function setContextAlias(map, contextName, rawAlias) {
  const name = String(contextName || '').trim()
  if (!name) return normalizeContextAliases(map)
  const next = normalizeContextAliases(map)
  const alias = normalizeAlias(rawAlias)
  if (!alias || alias === name) {
    if (!(name in next)) return next
    delete next[name]
    return next
  }
  if (next[name] === alias) return next
  next[name] = alias
  return next
}

/** Ellipsize a label when it exceeds max runes (for titles / compact chrome). */
export function truncateContextLabel(label, max = CONTEXT_LABEL_SOFT_MAX) {
  const s = String(label || '')
  const limit = Math.max(1, Number(max) || CONTEXT_LABEL_SOFT_MAX)
  if (s.length <= limit) return s
  if (limit <= 1) return '…'
  return `${s.slice(0, limit - 1)}…`
}

/**
 * Preferred display label: alias if set, else real kube context name.
 * Pass truncate:false for tooltips / rename fields.
 */
export function displayContextName(map, contextName, { truncate = true, max = CONTEXT_LABEL_SOFT_MAX } = {}) {
  const real = String(contextName || '').trim()
  if (!real) return ''
  const label = aliasForContext(map, real) || real
  return truncate ? truncateContextLabel(label, max) : label
}

/**
 * Learned cluster details per context (version, optional cluster label).
 * Kept in preferences so Home tiles stay clean and do not depend on a live probe.
 * @typedef {{ version?: string, cluster?: string }} ContextDetail
 */

/** @param {unknown} map @returns {Record<string, ContextDetail>} */
export function normalizeContextDetails(map) {
  if (!map || typeof map !== 'object' || Array.isArray(map)) return {}
  const out = {}
  for (const [rawName, raw] of Object.entries(map)) {
    const name = String(rawName || '').trim()
    if (!name || !raw || typeof raw !== 'object' || Array.isArray(raw)) continue
    const version = String(raw.version || '').trim()
    const cluster = String(raw.cluster || '').trim()
    if (!version && !cluster) continue
    out[name] = {
      ...(version ? { version } : {}),
      ...(cluster ? { cluster } : {}),
    }
  }
  return out
}

/** @param {Record<string, ContextDetail>} map */
export function detailsForContext(map, contextName) {
  const name = String(contextName || '').trim()
  if (!name) return null
  const d = normalizeContextDetails(map)[name]
  return d ? { ...d } : null
}

/**
 * Merge learned fields for one context. Returns the same map reference when unchanged.
 * @param {Record<string, ContextDetail>} map
 * @param {string} contextName
 * @param {{ version?: string, cluster?: string }} patch
 */
export function upsertContextDetails(map, contextName, patch = {}) {
  // Nothing to add: return the same object, so callers that compare by
  // reference (effects writing preferences) see "unchanged" and stop.
  const unchanged = map && typeof map === 'object' && !Array.isArray(map) ? map : {}
  const name = String(contextName || '').trim()
  if (!name) return unchanged
  const version = String(patch.version || '').trim()
  const cluster = String(patch.cluster || '').trim()
  if (!version && !cluster) return unchanged

  const next = normalizeContextDetails(map)
  const prev = next[name] || {}
  const merged = {
    ...prev,
    ...(version ? { version } : {}),
    ...(cluster ? { cluster } : {}),
  }
  if (prev.version === merged.version && prev.cluster === merged.cluster) {
    return map && typeof map === 'object' ? map : next
  }
  next[name] = merged
  return next
}

/**
 * Merge details discovered from kube context rows (cached API versions).
 * @param {Record<string, ContextDetail>} map
 * @param {{ name?: string, version?: string, cluster?: string }[]} contexts
 */
export function mergeContextDetailsFromContexts(map, contexts = []) {
  let next = map
  for (const c of contexts || []) {
    const name = String(c?.name || '').trim()
    if (!name) continue
    const version = String(c?.version || '').trim()
    const cluster = String(c?.cluster || '').trim()
    if (!version && !cluster) continue
    // Prefer a distinct cluster label; skip when it only repeats the context name.
    const clusterLabel = cluster && cluster !== name ? cluster : ''
    next = upsertContextDetails(next, name, {
      ...(version ? { version } : {}),
      ...(clusterLabel ? { cluster: clusterLabel } : {}),
    })
  }
  return next
}

/** One-line tile subtitle from prefs details (cluster · version). */
export function formatContextDetailMeta(detail, contextName = '') {
  if (!detail) return ''
  const name = String(contextName || '').trim()
  const cluster = String(detail.cluster || '').trim()
  const version = String(detail.version || '').trim()
  const clusterLabel = cluster && cluster !== name ? cluster : ''
  const versionLabel = version
    ? (version.startsWith('v') ? version : `v${version}`)
    : ''
  return [clusterLabel, versionLabel].filter(Boolean).join(' · ')
}

/** Unique tags across contexts, A→Z. */
export function collectContextTags(map, contextNames) {
  const normalized = normalizeContextTags(map)
  const names = Array.isArray(contextNames) && contextNames.length > 0
    ? new Set(contextNames.map((n) => String(n || '').trim()).filter(Boolean))
    : null
  const seen = new Set()
  for (const [ctx, tags] of Object.entries(normalized)) {
    if (names && !names.has(ctx)) continue
    for (const tag of tags) seen.add(tag)
  }
  return [...seen].sort((a, b) => a.localeCompare(b))
}

/**
 * Ordered contexts filtered by free-text search and optional tag.
 * @param {{ name: string, cluster?: string, user?: string, namespace?: string }[]} contexts
 */
export function filterContextsForHome(contexts, {
  defaultContext = '',
  favoriteContexts = [],
  contextTags = {},
  contextAliases = {},
  query = '',
  tag = '',
} = {}) {
  const ordered = orderContextsForDisplay(contexts, { defaultContext, favoriteContexts })
  const activeTag = normalizeTag(tag)
  const q = String(query || '').trim().toLowerCase()
  const tagsMap = normalizeContextTags(contextTags)
  const aliases = normalizeContextAliases(contextAliases)

  return ordered.filter((c) => {
    const tags = tagsForContext(tagsMap, c.name)
    if (activeTag && !tags.includes(activeTag)) return false
    if (!q) return true
    const haystack = [
      c.name,
      aliases[c.name],
      c.cluster,
      c.user,
      c.namespace,
      ...tags,
    ].filter(Boolean).join(' ').toLowerCase()
    return haystack.includes(q)
  })
}
