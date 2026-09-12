import { describe, expect, it } from 'vitest'
import {
  formatRbacAccessError,
  isRbacForbiddenMessage,
  normalizeCatalogAccessState,
  rbacAccessDeniedTitle,
} from './rbacAccess.js'

describe('isRbacForbiddenMessage', () => {
  it('detects kubectl forbidden output', () => {
    expect(isRbacForbiddenMessage('Error from server (Forbidden): secrets "x" is forbidden')).toBe(true)
  })

  it('detects detail provider permission errors', () => {
    expect(isRbacForbiddenMessage("you don't have permission to view Secret \"foo\"")).toBe(true)
  })

  it('ignores unrelated errors', () => {
    expect(isRbacForbiddenMessage('namespaces "GITLAB_TOKEN" not found')).toBe(false)
  })
})

describe('formatRbacAccessError', () => {
  it('normalizes forbidden list errors', () => {
    const msg = formatRbacAccessError('Forbidden: cannot list secrets', { kind: 'Secret' })
    expect(msg).toContain('Access denied')
    expect(msg).toContain('list Secret')
  })

  it('passes through non-forbidden errors', () => {
    expect(formatRbacAccessError('connection refused', { kind: 'Pod' })).toBe('connection refused')
  })
})

describe('normalizeCatalogAccessState', () => {
  it('maps stderr forbidden to forbidden accessState', () => {
    expect(normalizeCatalogAccessState('error', 'Forbidden: cannot list pods')).toBe('forbidden')
    expect(normalizeCatalogAccessState('unknown', 'cannot list secrets')).toBe('forbidden')
  })

  it('preserves allowed and explicit forbidden', () => {
    expect(normalizeCatalogAccessState('allowed', '')).toBe('allowed')
    expect(normalizeCatalogAccessState('forbidden', '')).toBe('forbidden')
  })
})

describe('rbacAccessDeniedTitle', () => {
  it('labels get vs list', () => {
    expect(rbacAccessDeniedTitle('list')).toBe('Access denied')
    expect(rbacAccessDeniedTitle('get')).toBe('Access denied (get)')
  })
})
