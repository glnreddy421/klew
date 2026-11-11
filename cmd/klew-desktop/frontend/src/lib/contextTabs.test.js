import { describe, expect, it } from 'vitest'
import {
  addContextTab,
  closeContextTab,
  MAX_CONTEXT_TABS,
  normalizeContextTabs,
  pruneContextTabs,
} from './contextTabs.js'

describe('contextTabs', () => {
  it('normalizes: trims, dedupes, drops blanks', () => {
    expect(normalizeContextTabs([' a ', 'b', 'a', '', null])).toEqual(['a', 'b'])
    expect(normalizeContextTabs('nope')).toEqual([])
  })

  it('adds at the end once', () => {
    expect(addContextTab(['a'], 'b')).toEqual(['a', 'b'])
    expect(addContextTab(['a', 'b'], 'a')).toEqual(['a', 'b'])
  })

  it('drops the oldest tab when full, never the kept one', () => {
    const full = Array.from({ length: MAX_CONTEXT_TABS }, (_, i) => `c${i}`)
    const next = addContextTab(full, 'new', { keep: 'c0' })
    expect(next).toHaveLength(MAX_CONTEXT_TABS)
    expect(next).toContain('c0')
    expect(next).not.toContain('c1')
    expect(next.at(-1)).toBe('new')
  })

  it('closing the active tab picks the right neighbour, then the left', () => {
    expect(closeContextTab(['a', 'b', 'c'], 'b', 'b')).toEqual({ tabs: ['a', 'c'], next: 'c' })
    expect(closeContextTab(['a', 'b', 'c'], 'c', 'c')).toEqual({ tabs: ['a', 'b'], next: 'b' })
    expect(closeContextTab(['a'], 'a', 'a')).toEqual({ tabs: [], next: '' })
  })

  it('closing a background tab keeps the active one', () => {
    expect(closeContextTab(['a', 'b'], 'a', 'b')).toEqual({ tabs: ['b'], next: '' })
  })

  it('prunes contexts that left kubeconfig', () => {
    expect(pruneContextTabs(['a', 'gone', 'b'], ['a', 'b'])).toEqual(['a', 'b'])
    expect(pruneContextTabs(['a'], [])).toEqual(['a'])
  })
})
