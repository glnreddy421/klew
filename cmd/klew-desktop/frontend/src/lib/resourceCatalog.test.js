import { describe, expect, it } from 'vitest'
import { kindDisplayLabel } from './resourceCatalog.js'

describe('kindDisplayLabel', () => {
  it('uses the API plural with the kind capitalisation', () => {
    expect(kindDisplayLabel('CleanupPolicy', undefined, 'cleanuppolicies')).toBe('CleanupPolicies')
    expect(kindDisplayLabel('GlobalContextEntry', undefined, 'globalcontextentries')).toBe('GlobalContextEntries')
    expect(kindDisplayLabel('Ingress', undefined, 'ingresses')).toBe('Ingresses')
    expect(kindDisplayLabel('Endpoints', undefined, 'endpoints')).toBe('Endpoints')
    expect(kindDisplayLabel('PolicyReport', undefined, 'policyreports')).toBe('PolicyReports')
  })

  it('ignores a plural that is a different word', () => {
    expect(kindDisplayLabel('Widget', undefined, 'gadgets')).toBe('Widgets')
  })

  it('falls back to English rules', () => {
    expect(kindDisplayLabel('NetworkPolicy')).toBe('NetworkPolicies')
    expect(kindDisplayLabel('Gateway')).toBe('Gateways')
    expect(kindDisplayLabel('Ingress')).toBe('Ingresses')
    expect(kindDisplayLabel('Pod')).toBe('Pods')
  })

  it('prefers an explicit label', () => {
    expect(kindDisplayLabel('Pod', 'Pods (all)')).toBe('Pods (all)')
    expect(kindDisplayLabel('')).toBe('')
  })
})
