/**
 * Run: node --test src/lib/clusterIdentity.test.js
 */
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { clusterContextNames, hasClusterContexts } from './clusterIdentity.js'

test('hasClusterContexts is true when contexts array is populated', () => {
  assert.equal(hasClusterContexts({ contexts: [{ name: 'dev' }] }), true)
})

test('hasClusterContexts is false when cluster state is empty', () => {
  assert.equal(hasClusterContexts({ contexts: [] }), false)
})

test('clusterContextNames returns kubeconfig context names', () => {
  assert.deepEqual(
    clusterContextNames({ contexts: [{ name: 'a' }, { name: 'b' }] }),
    ['a', 'b'],
  )
})
