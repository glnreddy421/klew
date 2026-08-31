/**
 * Run: node --test src/lib/clusterVersion.test.js
 */
import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  formatVersionChipLabel,
  formatVersionTooltip,
  formatNodesVersionLead,
} from './clusterVersion.js'

test('formatVersionChipLabel uses unified label when versions match', () => {
  const label = formatVersionChipLabel({
    kubernetesVersion: 'v1.36.1',
    versions: {
      apiServer: 'v1.36.1',
      skewed: false,
      controlPlane: { count: 1, label: 'v1.36.1', skewed: false },
      workers: { count: 1, label: 'v1.36.1', skewed: false },
    },
  })
  assert.equal(label, 'K8s v1.36.1')
})

test('formatVersionChipLabel splits control plane and worker versions', () => {
  const label = formatVersionChipLabel({
    kubernetesVersion: 'v1.36.1',
    versions: {
      apiServer: 'v1.36.1',
      skewed: true,
      controlPlane: { count: 1, label: 'v1.36.1', skewed: false },
      workers: { count: 2, label: 'v1.35.2', skewed: false },
    },
  })
  assert.match(label, /API v1\.36\.1/)
  assert.match(label, /CP v1\.36\.1/)
  assert.match(label, /workers v1\.35\.2/)
})

test('formatVersionTooltip lists per-version counts', () => {
  const tip = formatVersionTooltip({
    kubernetesVersion: 'v1.36.1',
    platform: 'linux/amd64',
    versions: {
      apiServer: 'v1.36.1',
      controlPlane: { count: 1, label: 'v1.36.1', versions: { 'v1.36.1': 1 } },
      workers: {
        count: 3,
        label: 'v1.35.2–v1.36.1',
        skewed: true,
        versions: { 'v1.35.2': 1, 'v1.36.1': 2 },
      },
    },
  })
  assert.match(tip, /Control plane \(1\): v1\.36\.1/)
  assert.match(tip, /Workers \(3\): v1\.35\.2 \(1\), v1\.36\.1 \(2\)/)
})

test('formatNodesVersionLead combines version and readiness', () => {
  const lead = formatNodesVersionLead({
    kubernetesVersion: 'v1.36.1',
    nodes: { total: 2, ready: 2 },
    versions: {
      apiServer: 'v1.36.1',
      skewed: false,
      controlPlane: { count: 1, label: 'v1.36.1' },
      workers: { count: 1, label: 'v1.36.1' },
    },
  })
  assert.equal(lead, 'K8s v1.36.1 · 2/2 ready')
})
