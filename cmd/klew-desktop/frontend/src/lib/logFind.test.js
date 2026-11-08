import { describe, expect, it } from 'vitest'
import { buildLogFindMatches, findSubstringRanges } from './logFind.js'

describe('findSubstringRanges', () => {
  it('finds overlapping case-insensitive matches', () => {
    expect(findSubstringRanges('aaAA', 'a')).toEqual([
      { start: 0, end: 1 },
      { start: 1, end: 2 },
      { start: 2, end: 3 },
      { start: 3, end: 4 },
    ])
  })

  it('returns empty for empty query', () => {
    expect(findSubstringRanges('hello', '  ')).toEqual([])
  })
})

describe('buildLogFindMatches', () => {
  it('flattens line matches in order', () => {
    const lines = [
      { seq: 1, text: 'foo bar' },
      { seq: 2, text: 'bar foo' },
    ]
    const m = buildLogFindMatches(lines, (r) => r.text, 'o')
    expect(m.length).toBe(4)
    expect(m[0]).toMatchObject({ row: lines[0], start: 1, end: 2 })
    expect(m[1]).toMatchObject({ row: lines[0], start: 2, end: 3 })
    expect(m[2]).toMatchObject({ row: lines[1], start: 5, end: 6 })
    expect(m[3]).toMatchObject({ row: lines[1], start: 6, end: 7 })
  })
})
