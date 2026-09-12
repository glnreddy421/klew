/**
 * Detect and normalize Kubernetes RBAC / authorization errors for UI copy.
 */

const FORBIDDEN_HINTS = [
  'forbidden',
  'access denied',
  'accessdenied',
  'not authorized',
  "don't have permission",
  'do not have permission',
  'cannot get resource',
  'cannot get ',
  'cannot list resource',
  'cannot list ',
  'cannot watch resource',
  'cannot watch ',
  'is forbidden:',
]

export function isRbacForbiddenMessage(raw) {
  const text = String(raw || '').trim().toLowerCase()
  if (!text) return false
  return FORBIDDEN_HINTS.some((hint) => text.includes(hint))
}

export function rbacAccessDeniedTitle(action = 'list') {
  return action === 'get' ? 'Access denied (get)' : 'Access denied'
}

export function formatRbacAccessError(raw, { action = 'list', kind = 'resource', name = '' } = {}) {
  if (!isRbacForbiddenMessage(raw)) return String(raw || '').trim()
  const label = kind || 'resource'
  const target = name ? `${label} "${name}"` : label
  if (action === 'get') {
    return `Access denied — you cannot get ${target} with the current identity.`
  }
  return `Access denied — you cannot list ${label} with the current identity.`
}

/** Normalize catalog LIST/GET results — stderr-style forbidden → accessState forbidden. */
export function normalizeCatalogAccessState(accessState, errorMessage = '') {
  if (accessState === 'forbidden' || isRbacForbiddenMessage(errorMessage)) {
    return 'forbidden'
  }
  if (accessState === 'allowed' || accessState === 'unavailable') {
    return accessState
  }
  if (accessState === 'error' && isRbacForbiddenMessage(errorMessage)) {
    return 'forbidden'
  }
  return accessState || 'unknown'
}
