/**
 * Run: node --test src/lib/clusterContext.test.js
 */
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { buildClusterContext } from './clusterContext.js'

const cluster = {
  selectedContext: 'prod-east',
  cluster: 'prod-cluster',
  selectedNamespace: 'payments',
  syncedAt: new Date().toISOString(),
  syncError: '',
}

test('buildClusterContext includes scope path and pod stats when running', () => {
  const view = {
    summary: { query: 'payment-api' },
    state: {
      snapshot: {
        pods: [
          { name: 'a', ready: true, phase: 'Running', containers: [{ state: 'Running', ready: true }] },
          { name: 'b', ready: false, containers: [{ lastReason: 'CrashLoopBackOff' }] },
        ],
        nodes: [{ name: 'node-1', ready: true }],
        metrics: { available: true },
        permissions: [{ resource: 'secrets', allowed: false }],
      },
    },
  }

  const ctx = buildClusterContext(cluster, view, { running: true })
  assert.equal(ctx.context, 'prod-east')
  assert.equal(ctx.namespace, 'payments')
  assert.equal(ctx.pods.total, 2)
  assert.equal(ctx.pods.failing, 1)
  assert.equal(ctx.nodes.total, 1)
  assert.equal(ctx.metricsAvailable, true)
  assert.equal(ctx.rbacLimited, true)
  assert.ok(ctx.chips.some((c) => c.id === 'pods' && c.navTab === 'failures'))
  assert.ok(ctx.showBar)
})

test('buildClusterContext surfaces sync errors with settings action', () => {
  const ctx = buildClusterContext(
    { ...cluster, syncError: 'forbidden', syncedAt: null },
    {},
    { running: false },
  )
  const syncChip = ctx.chips.find((c) => c.id === 'sync')
  assert.equal(syncChip.tone, 'crit')
  assert.equal(syncChip.action, 'settings')
})

test('buildClusterContext shows syncing state', () => {
  const ctx = buildClusterContext(cluster, {}, { syncing: true })
  const syncChip = ctx.chips.find((c) => c.id === 'sync')
  assert.equal(syncChip.label, 'Syncing…')
})

test('buildClusterContext hides scope chips when idle', () => {
  const ctx = buildClusterContext(cluster, {}, { running: false })
  assert.ok(ctx.showBar)
  assert.ok(!ctx.chips.some((c) => c.id === 'pods'))
})

test('buildClusterContext adds cluster-wide status chips', () => {
  const ctx = buildClusterContext(cluster, {}, {
    running: false,
    clusterStatus: {
      available: true,
      apiReachable: true,
      kubernetesVersion: 'v1.29.4',
      platform: 'linux/amd64',
      nodes: { total: 3, ready: 3, notReady: 0, pressured: 0 },
      versions: {
        apiServer: 'v1.29.4',
        skewed: false,
        controlPlane: { count: 1, label: 'v1.29.4', skewed: false },
        workers: { count: 2, label: 'v1.29.4', skewed: false },
      },
    },
  })
  assert.ok(ctx.chips.some((c) => c.id === 'k8s-version' && c.clusterWide))
  assert.ok(ctx.chips.some((c) => c.id === 'cluster-nodes' && c.navTab === 'nodes'))
})

test('buildClusterContext rbac chip scrolls to visibility warning', () => {
  const view = {
    state: {
      snapshot: {
        permissions: [{ resource: 'secrets', allowed: false }],
      },
    },
  }
  const ctx = buildClusterContext(cluster, view, { running: true })
  const rbac = ctx.chips.find((c) => c.id === 'rbac')
  assert.equal(rbac.action, 'visibility')
})
