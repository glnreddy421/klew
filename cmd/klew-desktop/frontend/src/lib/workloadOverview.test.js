/**
 * Run: node --test src/lib/workloadOverview.test.js
 */
import test from 'node:test'
import assert from 'node:assert/strict'
import { buildWorkloadKindCardsFromRows } from './workloadOverview.js'

const kindGroups = [
  { kind: 'Pod', label: 'Pods', resourceId: 'v1/pods', count: 0, countState: { state: 'loaded', value: 0 } },
  { kind: 'Deployment', label: 'Deployments', resourceId: 'apps/v1/deployments', count: 2, countState: { state: 'loaded', value: 2 } },
  { kind: 'ReplicaSet', label: 'ReplicaSets', resourceId: 'apps/v1/replicasets', count: 5, countState: { state: 'loaded', value: 5 } },
]

test('buildWorkloadKindCardsFromRows skips non-overview kinds', () => {
  const cards = buildWorkloadKindCardsFromRows(kindGroups, {
    'v1/pods': [
      { kind: 'Pod', name: 'a', phase: 'Running', ready: true, signal: 'Running', status: 'healthy' },
      { kind: 'Pod', name: 'b', phase: 'Pending', ready: false, signal: 'Pending', status: 'degraded' },
    ],
    'apps/v1/deployments': [
      { kind: 'Deployment', name: 'web', readyReplicas: 2, desiredReplicas: 3, signal: '2/3 ready' },
    ],
  })
  assert.deepEqual(cards.map((c) => c.kind), ['Pod', 'Deployment'])
  assert.equal(cards[0].total, 2)
  assert.equal(cards[1].total, 1)
  assert.ok(cards[0].segments.some((s) => s.label === 'Running'))
  assert.ok(cards[1].segments.some((s) => s.label === 'Ready'))
})

test('buildWorkloadKindCardsFromRows falls back to catalog counts', () => {
  const cards = buildWorkloadKindCardsFromRows(kindGroups, {})
  assert.equal(cards[1].total, 2)
})
