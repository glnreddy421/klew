/**
 * Run: node --test src/lib/workloadOverview.test.js
 */
import test from 'node:test'
import assert from 'node:assert/strict'
import {
  buildAttentionItems,
  buildWorkloadKindCardsFromRows,
  donutArcs,
  summarizeWorkloadCards,
} from './workloadOverview.js'

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
  assert.deepEqual(cards[1].segments, [{ label: 'Degraded', tone: 'warn', count: 1 }])
})

function cardFor(kind, rows) {
  const group = { kind, label: kind, resourceId: kind, count: rows.length }
  return buildWorkloadKindCardsFromRows([group], { [kind]: rows })[0]
}

function segmentSum(card) {
  return card.segments.reduce((n, s) => n + s.count, 0)
}

test('replica workloads count objects, not replicas', () => {
  const card = cardFor('Deployment', [
    { kind: 'Deployment', desiredReplicas: 3, readyReplicas: 3 },
    { kind: 'Deployment', desiredReplicas: 5, readyReplicas: 2 },
    { kind: 'Deployment', desiredReplicas: 2, readyReplicas: 0 },
    { kind: 'Deployment', desiredReplicas: 0, readyReplicas: 0 },
  ])
  assert.equal(card.total, 4)
  assert.equal(segmentSum(card), 4)
  assert.deepEqual(card.segments.map((s) => [s.label, s.count]), [
    ['Ready', 1], ['Degraded', 1], ['Unavailable', 1], ['Scaled to zero', 1],
  ])
})

test('daemonsets count objects, not scheduled pods', () => {
  const card = cardFor('DaemonSet', [
    { kind: 'DaemonSet', desiredReplicas: 2, readyReplicas: 2 },
    { kind: 'DaemonSet', desiredReplicas: 2, readyReplicas: 2 },
  ])
  assert.equal(card.total, 2)
  assert.deepEqual(card.segments, [{ label: 'Ready', tone: 'ok', count: 2 }])
})

test('jobs use conditions and running jobs are not attention', () => {
  const card = cardFor('Job', [
    { kind: 'Job', conditions: [{ type: 'Complete', status: 'True' }] },
    { kind: 'Job', succeeded: 1, completions: 1 },
    { kind: 'Job', succeeded: 0, completions: 1 },
    { kind: 'Job', conditions: [{ type: 'Failed', status: 'True' }] },
  ])
  assert.deepEqual(card.segments.map((s) => [s.label, s.tone, s.count]), [
    ['Complete', 'ok', 2], ['Running', 'info', 1], ['Failed', 'crit', 1],
  ])
  assert.deepEqual(summarizeWorkloadCards([card]), { total: 4, healthy: 2, attention: 1 })
})

test('cronjobs show schedule state', () => {
  const card = cardFor('CronJob', [
    { kind: 'CronJob', suspend: false, activeJobs: 0 },
    { kind: 'CronJob', suspend: false, activeJobs: 1 },
    { kind: 'CronJob', suspend: true, activeJobs: 0 },
  ])
  assert.deepEqual(card.segments.map((s) => s.label), ['Scheduled', 'Running', 'Suspended'])
})

test('summary counts healthy and attention objects in one unit', () => {
  const cards = [
    cardFor('Pod', [
      { kind: 'Pod', phase: 'Running', ready: true, status: 'healthy' },
      { kind: 'Pod', phase: 'Pending', ready: false, status: 'degraded' },
    ]),
    cardFor('Deployment', [
      { kind: 'Deployment', desiredReplicas: 3, readyReplicas: 3 },
      { kind: 'Deployment', desiredReplicas: 3, readyReplicas: 1 },
    ]),
  ]
  assert.deepEqual(summarizeWorkloadCards(cards), { total: 4, healthy: 2, attention: 2 })
})

test('attention lists unhealthy objects, worst first and controllers before pods', () => {
  const groups = [
    { kind: 'Pod', resourceId: 'pods' },
    { kind: 'Deployment', resourceId: 'deps' },
    { kind: 'Job', resourceId: 'jobs' },
  ]
  const items = buildAttentionItems(groups, {
    pods: [
      { kind: 'Pod', key: 'p1', name: 'web-1', namespace: 'shop', phase: 'Running', ready: true, status: 'healthy' },
      {
        kind: 'Pod', key: 'p2', name: 'web-2', namespace: 'shop', phase: 'Pending', ready: false,
        containerStatuses: [{ ready: false, reason: 'ImagePullBackOff' }], restartCount: 0,
      },
      { kind: 'Pod', key: 'p3', name: 'batch-1', namespace: 'shop', phase: 'Failed', ready: false, restartCount: 2 },
    ],
    deps: [
      { kind: 'Deployment', key: 'd1', name: 'web', namespace: 'shop', desiredReplicas: 3, readyReplicas: 1 },
      { kind: 'Deployment', key: 'd2', name: 'api', namespace: 'shop', desiredReplicas: 2, readyReplicas: 0 },
      { kind: 'Deployment', key: 'd3', name: 'ok', namespace: 'shop', desiredReplicas: 2, readyReplicas: 2 },
    ],
    jobs: [
      { kind: 'Job', key: 'j1', name: 'migrate', namespace: 'shop', conditions: [{ type: 'Failed', status: 'True', reason: 'BackoffLimitExceeded' }] },
    ],
  })
  assert.deepEqual(items.map((i) => [i.key, i.tone, i.reason]), [
    ['d2', 'crit', '0/2 ready'],
    ['j1', 'crit', 'BackoffLimitExceeded'],
    ['p3', 'crit', 'Failed · 2 restarts'],
    ['d1', 'warn', '1/3 ready'],
    ['p2', 'warn', 'ImagePullBackOff'],
  ])
})

test('donutArcs builds stroked paths with gaps between segments', () => {
  const arcs = donutArcs([
    { label: 'Ready', count: 2, tone: 'ok' },
    { label: 'Degraded', count: 1, tone: 'warn' },
  ])
  assert.equal(arcs.length, 2)
  assert.ok(arcs[0].d.startsWith('M '))
  assert.ok(arcs[0].d.includes(' A '))
  assert.equal(arcs[0].tone, 'ok')
  assert.equal(donutArcs([]).length, 0)
  const full = donutArcs([{ label: 'Ready', count: 1, tone: 'ok' }])
  assert.equal(full.length, 1)
  assert.equal(full[0].full, true)
})

test('kinds without rows fall back to an unknown status, not healthy', () => {
  const cards = buildWorkloadKindCardsFromRows(kindGroups, {})
  assert.deepEqual(cards[1].segments, [{ label: 'Status unknown', count: 2, tone: 'muted' }])
})

test('buildWorkloadKindCardsFromRows falls back to catalog counts', () => {
  const cards = buildWorkloadKindCardsFromRows(kindGroups, {})
  assert.equal(cards[1].total, 2)
})

test('buildWorkloadKindCardsFromRows marks denied kinds', () => {
  const cards = buildWorkloadKindCardsFromRows(kindGroups, {}, {
    'apps/v1/deployments': 'forbidden',
  })
  const dep = cards.find((c) => c.kind === 'Deployment')
  assert.equal(dep.denied, true)
  assert.equal(dep.total, 0)
})
