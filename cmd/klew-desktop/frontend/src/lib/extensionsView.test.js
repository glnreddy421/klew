import { describe, expect, it } from 'vitest'
import { entriesByFile, folderProblems, testSummary } from './extensionsView'

describe('extensionsView', () => {
  it('groups documents by file', () => {
    const files = entriesByFile([
      { file: '/k/ext/a.yaml', folder: '/k/ext', document: 1, kind: 'Rule', name: 'r1', status: 'loaded' },
      { file: '/k/ext/a.yaml', folder: '/k/ext', document: 2, kind: 'Rule', name: 'r2', status: 'skipped', reason: 'bad' },
      { file: '/repo/b.yml', folder: '/repo', document: 1, kind: 'Collector', name: 'c', status: 'loaded' },
    ])
    expect(files.map((f) => [f.name, f.entries.length])).toEqual([['a.yaml', 2], ['b.yml', 1]])
  })

  it('keeps only problems not already shown on a document', () => {
    expect(folderProblems([
      'extensions: a.yaml document 2: bad',
      'extensions: rule typo: resource "x" is not a collector',
      'extensions: linked folder /gone was not found',
      'extensions: open /locked: permission denied',
    ])).toEqual(['open /locked: permission denied'])
  })

  it('lists matches before errors and counts the rest', () => {
    const [run] = testSummary({ runs: [{
      rule: 'r', resource: 'pods', warnings: ['w'],
      checks: [
        { object: { kind: 'Pod', name: 'ok' } },
        { object: { kind: 'Pod', name: 'bad' }, error: 'no such key' },
        { object: { kind: 'Pod', name: 'hit' }, matched: true, facts: ['f1'], factErrors: ['fact 2: x'] },
      ],
    }] })
    expect(run).toMatchObject({ checked: 3, matched: 1, errors: 1, hidden: 0, warnings: ['w'] })
    expect(run.lines.map((l) => [l.kind, l.object, l.text])).toEqual([
      ['match', 'Pod/hit', 'f1 · fact failed: fact 2: x'],
      ['skipped', 'Pod/bad', 'no such key'],
    ])
  })
})
