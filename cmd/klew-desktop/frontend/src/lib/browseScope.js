/** Cluster scope — Resources browse and Investigate use separate pickers in the UI. */

export const BROWSE_ALL_NAMESPACES = '*'

export function singleBrowseScope(namespace) {
  const ns = String(namespace || '').trim()
  return ns ? { mode: 'single', namespace: ns, namespaces: [] } : { mode: 'single', namespace: '', namespaces: [] }
}

export function allBrowseScope() {
  return { mode: 'all', namespace: BROWSE_ALL_NAMESPACES, namespaces: [] }
}

export function multiBrowseScope(namespaces) {
  const list = [...new Set((namespaces || []).map((n) => String(n).trim()).filter(Boolean))]
  return { mode: 'multi', namespace: '', namespaces: list }
}

export function normalizeBrowseScope(raw) {
  if (!raw) return singleBrowseScope('')
  if (raw === BROWSE_ALL_NAMESPACES) return allBrowseScope()
  if (typeof raw === 'string') return singleBrowseScope(raw)
  if (raw.mode === 'all') return allBrowseScope()
  if (raw.mode === 'multi') return multiBrowseScope(raw.namespaces)
  return singleBrowseScope(raw.namespace)
}

export function browseScopeLabel(scope, { namespaces = [] } = {}) {
  const s = normalizeBrowseScope(scope)
  if (s.mode === 'all') return 'All namespaces'
  if (s.mode === 'multi') {
    const n = s.namespaces.length
    if (n === 0) return 'Select namespaces'
    if (n === 1) return s.namespaces[0]
    return `${n} namespaces`
  }
  return s.namespace || (namespaces[0] || 'Namespace')
}

export function browseScopeApiParams(scope) {
  const s = normalizeBrowseScope(scope)
  if (s.mode === 'all') {
    return { namespace: '', allNamespaces: true, namespaces: [] }
  }
  if (s.mode === 'multi') {
    return { namespace: '', allNamespaces: false, namespaces: s.namespaces }
  }
  return { namespace: s.namespace, allNamespaces: false, namespaces: [] }
}

export function scopeSupportsInvestigate(scope) {
  const s = normalizeInvestigationScope(scope)
  return Boolean(s.namespace)
}

/** Investigation is strictly single-namespace. Coerces all/multi to one namespace. */
export function normalizeInvestigationScope(raw, { fallbackNamespace = '' } = {}) {
  const s = normalizeBrowseScope(raw)
  if (s.mode === 'single' && s.namespace) return s
  if (s.mode === 'multi') {
    const ns = s.namespaces.find(Boolean) || ''
    if (ns) return singleBrowseScope(ns)
  }
  const fb = String(fallbackNamespace || '').trim()
  return singleBrowseScope(fb)
}

/** Namespace scope locked on Resources for the duration of an investigation session. */
export function investigationLockScope(investigationScope, sessionNs = '') {
  const s = normalizeInvestigationScope(investigationScope, { fallbackNamespace: sessionNs })
  return singleBrowseScope(sessionNs || s.namespace)
}

/** Namespace for a live investigation session derived from scope and optional matches. */
export function investigationNamespace(scope, matches = []) {
  const s = normalizeInvestigationScope(scope)
  if (s.namespace) return s.namespace
  const fromMatches = [...new Set((matches || []).map((m) => m.ref?.namespace).filter(Boolean))]
  if (fromMatches.length === 1) return fromMatches[0]
  if (fromMatches.length > 0) return fromMatches[0]
  return ''
}

export function discoverOptionsFromScope(scope, base = {}) {
  const api = browseScopeApiParams(normalizeBrowseScope(scope))
  return {
    ...base,
    namespace: api.namespace,
    allNamespaces: api.allNamespaces,
    namespaces: api.namespaces,
  }
}

/** Resources catalog lens while an investigation is active. */
export const RESOURCES_BROWSE_LENS = {
  MATCHES: 'matches',
  ALL: 'all',
}

export function isResourcesBrowseAll(lens) {
  return lens === RESOURCES_BROWSE_LENS.ALL
}

export function browseScopesEqual(a, b) {
  const left = normalizeBrowseScope(a)
  const right = normalizeBrowseScope(b)
  if (left.mode !== right.mode) return false
  if (left.mode === 'all') return true
  if (left.mode === 'multi') {
    return left.namespaces.length === right.namespaces.length
      && left.namespaces.every((n, i) => n === right.namespaces[i])
  }
  return left.namespace === right.namespace
}
