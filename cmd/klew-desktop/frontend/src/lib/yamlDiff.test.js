import { describe, expect, it } from 'vitest'
import { compareOptions, defaultPair, diffLines, diffStats, foldUnchanged } from './yamlDiff.js'

const before = `spec:
  containers:
  - image: pay:v1
    name: app
    env:
    - name: LOG_LEVEL
      value: info
`
const after = `spec:
  containers:
  - image: pay:v2
    name: app
    env:
    - name: LOG_LEVEL
      value: debug
    - name: REGION
      value: eu
`

describe('diffLines', () => {
  it('marks changed lines as removed then added, with both line numbers', () => {
    const rows = diffLines(before, after)
    expect(rows.filter((r) => r.type !== 'same').map((r) => `${r.type} ${r.text.trim()}`)).toEqual([
      'del - image: pay:v1',
      'add - image: pay:v2',
      'del value: info',
      'add value: debug',
      'add - name: REGION',
      'add value: eu',
    ])
    expect(rows[0]).toEqual({ type: 'same', text: 'spec:', oldNo: 1, newNo: 1 })
    const region = rows.find((r) => r.text.includes('REGION'))
    expect(region.oldNo).toBeNull()
    expect(region.newNo).toBe(8)
  })

  it('reports identical documents as all unchanged', () => {
    const rows = diffLines(before, before)
    expect(rows.every((r) => r.type === 'same')).toBe(true)
    expect(rows).toHaveLength(7)
  })

  it('handles empty sides', () => {
    expect(diffStats(diffLines('', 'a\nb\n'))).toEqual({ added: 2, removed: 0 })
    expect(diffStats(diffLines('a\n', ''))).toEqual({ added: 0, removed: 1 })
    expect(diffLines('', '')).toEqual([])
  })

  it('aligns moved blocks rather than replacing everything', () => {
    const rows = diffLines('a\nb\nc\nd\n', 'a\nc\nd\nb\n')
    expect(diffStats(rows)).toEqual({ added: 1, removed: 1 })
  })
})

describe('foldUnchanged', () => {
  it('keeps context around changes and folds long unchanged runs', () => {
    const lines = Array.from({ length: 30 }, (_, i) => `line ${i}`)
    const changed = [...lines]
    changed[15] = 'line 15 changed'
    const groups = foldUnchanged(diffLines(lines.join('\n'), changed.join('\n')), 3)
    expect(groups.map((g) => [g.kind, g.rows.length])).toEqual([
      ['gap', 12],
      ['rows', 8],
      ['gap', 11],
    ])
  })

  it('shows short unchanged runs instead of folding them', () => {
    const groups = foldUnchanged(diffLines('a\nb\nc\n', 'a\nB\nc\n'), 3)
    expect(groups).toHaveLength(1)
    expect(groups[0].kind).toBe('rows')
  })
})

describe('compare pair', () => {
  const wc = {
    changes: [
      { revision: 5, current: true },
      { revision: 0, helmRevision: 3 },
      { revision: 4 },
      { revision: 4 },
      { revision: 2 },
    ],
  }

  it('offers each numbered revision once, newest first', () => {
    expect(compareOptions(wc).map((c) => c.revision)).toEqual([5, 4, 2])
  })

  it('defaults to the newest against the one before', () => {
    expect(defaultPair(compareOptions(wc))).toEqual({ from: 4, to: 5 })
    expect(defaultPair(compareOptions(wc), { from: 2, to: 5 })).toEqual({ from: 2, to: 5 })
    expect(defaultPair(compareOptions(wc), { from: 1, to: 5 })).toEqual({ from: 4, to: 5 })
    expect(defaultPair(compareOptions({ changes: [{ revision: 1 }] }))).toBeNull()
  })
})
