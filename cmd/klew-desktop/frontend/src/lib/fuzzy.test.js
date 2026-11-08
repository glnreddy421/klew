import { describe, expect, it } from 'vitest'
import { fuzzyMatch, fuzzyMatchFields, highlightRuns } from './fuzzy'

describe('fuzzyMatch', () => {
  it('matches characters in order, case-insensitively', () => {
    expect(fuzzyMatch('svc', 'Service')?.indices).toEqual([0, 3, 5])
    expect(fuzzyMatch('xyz', 'Service')).toBeNull()
    expect(fuzzyMatch('ecs', 'Service')).toBeNull()
  })

  it('prefers word starts and consecutive runs', () => {
    const start = fuzzyMatch('led', 'ledger').score
    const scattered = fuzzyMatch('led', 'lxexd').score
    expect(start).toBeGreaterThan(scattered)
    expect(fuzzyMatch('pvc', 'PersistentVolumeClaim').indices).toEqual([0, 10, 16])
  })

  it('needs every token to match some field', () => {
    expect(fuzzyMatchFields('svc gw', { name: 'gateway', kind: 'Service' })).not.toBeNull()
    expect(fuzzyMatchFields('svc zz', { name: 'gateway', kind: 'Service' })).toBeNull()
  })
})

describe('highlightRuns', () => {
  it('splits text into matched and unmatched runs', () => {
    expect(highlightRuns('billing', [0, 1, 2])).toEqual([{ text: 'bil', hit: true }, { text: 'ling', hit: false }])
    expect(highlightRuns('api', [])).toEqual([{ text: 'api', hit: false }])
  })
})
