import { describe, expect, it } from 'vitest'
import {
  browseDraftToScope,
  browseScopeToDraft,
  browseScopesEqual,
  multiBrowseScope,
  singleBrowseScope,
} from './browseScope.js'

describe('browse scope draft (namespace picker)', () => {
  it('apply multi-select without flipping to all', () => {
    const draft = { mode: 'multi', selected: new Set(['a', 'b', 'c']) }
    const scope = browseDraftToScope(draft)
    expect(scope.mode).toBe('multi')
    expect(scope.namespaces).toEqual(['a', 'b', 'c'])
  })

  it('explicit all mode only via draft.mode', () => {
    const draft = browseScopeToDraft(multiBrowseScope(['a', 'b']), ['a', 'b', 'c'])
    expect(draft.mode).toBe('multi')
    const allDraft = { mode: 'all', selected: new Set(['a', 'b', 'c']) }
    expect(browseDraftToScope(allDraft).mode).toBe('all')
  })

  it('scopes stay equal across representation', () => {
    expect(browseScopesEqual(
      singleBrowseScope('prod'),
      singleBrowseScope('prod'),
    )).toBe(true)
    expect(browseScopesEqual(
      multiBrowseScope(['a', 'b']),
      multiBrowseScope(['b', 'a']),
    )).toBe(false)
  })
})
