import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  filterPatternTemplates,
  isEmergingTemplate,
  isRecurringTemplate,
  patternExplorerCounts,
  resolvePatternTabKind,
} from './patternFilters.js'

test('isEmergingTemplate detects rising arrow trend', () => {
  assert.equal(isEmergingTemplate({ trend: '↑' }), true)
  assert.equal(isEmergingTemplate({ trend: '·' }), false)
  assert.equal(isEmergingTemplate({ trendPct: 80 }), true)
})

test('filterPatternTemplates applies recurring and emerging filters', () => {
  const templates = [
    { id: 'a', template: 'restart loop', count: 5, trend: '·' },
    { id: 'b', template: 'info line', count: 1, trend: '↑' },
    { id: 'c', template: 'probe failed', count: 4, trend: '↑' },
  ]

  assert.equal(filterPatternTemplates(templates, { kind: 'recurring' }).length, 2)
  assert.equal(filterPatternTemplates(templates, { kind: 'emerging' }).length, 2)
  assert.equal(filterPatternTemplates(templates, { kind: 'recurring', signal: 'restart' }).length, 1)
})

test('patternExplorerCounts and tab resolution', () => {
  const patterns = {
    templates: [{ count: 4, trend: '↑' }, { count: 1, trend: '·' }],
    eventTemplates: [{ count: 3, trend: '·' }],
  }
  const counts = patternExplorerCounts(patterns)
  assert.equal(counts.total, 3)
  assert.equal(counts.recurring, 2)
  assert.equal(counts.emerging, 1)
  assert.equal(resolvePatternTabKind({ kind: 'events' }), 'events')
  assert.equal(resolvePatternTabKind({ kind: 'recurring' }), 'logs')
})
