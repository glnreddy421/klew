/**
 * Run: node --test src/lib/scopeSearch.test.js
 */
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { browseScopeLabel } from './browseScope.js'
import { filterBySubstring, filterScopeMatches, kindFiltersForMatches } from './scopeSearch.js'

test('filterBySubstring matches case-insensitively', () => {
  const items = ['comms-api', 'default', 'kube-system']
  assert.deepEqual(
    filterBySubstring(items, 'COM', (x) => x),
    ['comms-api'],
  )
})

test('filterBySubstring tolerates null items', () => {
  assert.deepEqual(filterBySubstring(null, '', (x) => x), [])
})

test('browseScopeLabel tolerates null namespaces list', () => {
  assert.equal(browseScopeLabel('', { namespaces: null }), 'Namespace')
  assert.equal(browseScopeLabel('', { namespaces: ['prod'] }), 'prod')
})

test('filterScopeMatches filters by kind and text', () => {
  const matches = [
    { ref: { kind: 'Deployment', name: 'payment-api' }, score: 1 },
    { ref: { kind: 'Service', name: 'payment-api' }, score: 0.5 },
  ]
  assert.equal(filterScopeMatches(matches, { kind: 'Deployment' }).length, 1)
  assert.equal(filterScopeMatches(matches, { text: 'service' }).length, 1)
})

test('kindFiltersForMatches orders workloads first', () => {
  const kinds = kindFiltersForMatches([
    { ref: { kind: 'Service', name: 'a' } },
    { ref: { kind: 'Deployment', name: 'b' } },
  ])
  assert.equal(kinds[0], 'Deployment')
})
