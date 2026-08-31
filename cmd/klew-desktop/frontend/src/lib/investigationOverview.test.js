/**
 * Investigation Overview view model tests.
 * Run: node --test src/lib/investigationOverview.test.js
 */
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { buildInvestigationOverview } from './investigationOverview.js'
import { deriveMatchRows } from './matches.js'

function baseView(overrides = {}) {
  return {
    summary: {
      status: 'warning',
      leadingSignal: 'OOMKilled',
      confidence: 0.72,
      live: true,
      window: 900000,
    },
    hypothesis: 'Pod restarts correlate with memory pressure',
    signals: [
      { id: 's1', label: 'OOMKilled', severity: 'critical', strength: 'strong', count: 3 },
    ],
    evidence: [
      {
        id: 'e1',
        timestamp: '2026-08-26T21:31:18Z',
        sourceType: 'k8s_event',
        reason: 'OOMKilled',
        message: 'Container was OOMKilled',
        severity: 'critical',
      },
      {
        id: 'e2',
        timestamp: '2026-08-26T21:32:04Z',
        sourceType: 'k8s_event',
        reason: 'Unhealthy',
        message: 'Readiness probe failed',
        severity: 'warning',
      },
    ],
    state: {
      window: 900000,
      verdict: {
        status: 'warning',
        leadingSignal: 'OOMKilled',
        confidence: 0.72,
        summary: 'Pod restarts detected',
        strongSignals: [
          {
            id: 'vs1',
            label: 'Memory pressure precedes pod restarts',
            severity: 'critical',
            strength: 'strong',
            count: 8,
            evidence: 'OOMKilled → Container restart → Readiness failure',
          },
        ],
        affectedObjects: [{ kind: 'Deployment', name: 'payment-api' }],
      },
      timeline: [
        {
          timestamp: '2026-08-26T21:31:02Z',
          type: 'event',
          severity: 'warning',
          reason: 'Memory pressure',
          message: 'Memory pressure detected',
          sourceKind: 'Node',
          sourceName: 'node-1',
        },
        {
          timestamp: '2026-08-26T21:31:18Z',
          type: 'event',
          severity: 'critical',
          reason: 'OOMKilled',
          message: 'Container OOMKilled',
          sourceKind: 'Pod',
          sourceName: 'payment-api-7dc9f-x82ks',
        },
      ],
      causalChain: ['Memory pressure', 'OOMKilled', 'Container restart', 'Readiness failure'],
      correlation: [],
      liveEvidence: [],
      workloadGraph: {
        nodes: [
          { id: 'd1', kind: 'Deployment', name: 'payment-api', health: 'warning' },
          { id: 'rs1', kind: 'ReplicaSet', name: 'payment-api-7dc9f', health: 'warning' },
          { id: 'p1', kind: 'Pod', name: 'payment-api-7dc9f-x82ks', health: 'critical' },
        ],
        edges: [
          { from: 'd1', to: 'rs1', relation: 'owns' },
          { from: 'rs1', to: 'p1', relation: 'owns' },
        ],
      },
      snapshot: {
        pods: [
          {
            name: 'payment-api-7dc9f-x82ks',
            ready: false,
            restartCount: 3,
            containers: [{ name: 'app', lastReason: 'OOMKilled' }],
          },
        ],
        matchedObjects: [{ ref: { kind: 'Deployment', name: 'payment-api' }, score: 10 }],
      },
      matchedObjects: [{ ref: { kind: 'Deployment', name: 'payment-api' }, score: 10 }],
    },
    ...overrides,
  }
}

test('buildInvestigationOverview detects active incident phase', () => {
  const view = baseView()
  const rows = deriveMatchRows(view, view.state.matchedObjects)
  const overview = buildInvestigationOverview(view, { rows, live: true })

  assert.equal(overview.phase, 'active')
  assert.equal(overview.verdict.headline, 'Pod restarts correlate with memory pressure')
  assert.ok(overview.findings.length >= 1)
  assert.equal(overview.investigationChain.steps.length, 4)
  assert.ok(overview.visualChain)
  assert.equal(overview.visualChain.mode, 'chain')
  assert.ok(overview.visualChain.nodes.length >= 2)
  assert.ok(overview.timelineVisual.items.length >= 1)
  assert.ok(overview.affectedResources.length >= 1)
  assert.ok(overview.evidencePreview.length >= 1)
})

test('buildInvestigationOverview uses correlation language when no causal chain', () => {
  const view = baseView({
    state: {
      ...baseView().state,
      causalChain: [],
      correlation: ['Pod restarts', 'Readiness failures', 'Connection refusals'],
    },
  })
  const rows = deriveMatchRows(view, view.state.matchedObjects)
  const overview = buildInvestigationOverview(view, { rows })

  assert.equal(overview.investigationChain.connector, 'correlated with')
  assert.equal(overview.investigationChain.steps.length, 3)
  assert.equal(overview.visualChain.edgeKind, 'correlation')
  assert.equal(overview.visualChain.edges[0].label, 'correlated with')
})

test('buildInvestigationOverview quiet phase when healthy', () => {
  const view = baseView({
    hypothesis: '',
    summary: { status: 'healthy', live: true },
    signals: [],
    evidence: [],
    state: {
      window: 900000,
      verdict: { status: 'healthy', confidence: 0 },
      timeline: [],
      snapshot: {
        pods: [{ name: 'ok-pod', ready: true, restartCount: 0, containers: [] }],
        matchedObjects: [{ ref: { kind: 'Deployment', name: 'ok' }, score: 1 }],
      },
      matchedObjects: [{ ref: { kind: 'Deployment', name: 'ok' }, score: 1 }],
    },
  })
  const rows = deriveMatchRows(view, view.state.matchedObjects)
  const overview = buildInvestigationOverview(view, { rows })

  assert.equal(overview.phase, 'quiet')
  assert.equal(overview.verdict.headline, 'No significant failures detected')
  assert.equal(overview.verdict.confidence, null)
})

test('buildInvestigationOverview empty phase without scope or activity', () => {
  const view = {
    summary: {},
    signals: [],
    evidence: [],
    state: { snapshot: {}, verdict: {} },
  }
  const overview = buildInvestigationOverview(view, { rows: [] })
  assert.equal(overview.phase, 'empty')
})

test('buildInvestigationOverview surfaces RBAC visibility warning', () => {
  const view = baseView({
    state: {
      ...baseView().state,
      permissions: [
        { resource: 'secrets', verb: 'list', allowed: false },
        { resource: 'events', verb: 'list', allowed: false },
      ],
    },
  })
  const rows = deriveMatchRows(view, view.state.matchedObjects)
  const overview = buildInvestigationOverview(view, { rows })

  assert.ok(overview.visibilityWarning)
  assert.match(overview.visibilityWarning.message, /cannot access/)
})

test('findings prioritize strong verdict signals', () => {
  const view = baseView()
  const rows = deriveMatchRows(view, view.state.matchedObjects)
  const overview = buildInvestigationOverview(view, { rows })

  assert.match(overview.findings[0].title, /Memory pressure|OOMKilled/i)
  assert.equal(overview.findings[0].nav.tab, 'evidence')
})

test('findings link to visual chain nodes when chain steps overlap', () => {
  const view = baseView()
  const rows = deriveMatchRows(view, view.state.matchedObjects)
  const overview = buildInvestigationOverview(view, { rows })

  const withChain = overview.findings.find((f) => f.chainSteps?.length >= 2)
  if (withChain) {
    assert.ok(Array.isArray(withChain.chainNodeIds))
  }
})

test('resource path visual includes ownership steps', () => {
  const view = baseView()
  const rows = deriveMatchRows(view, view.state.matchedObjects)
  const overview = buildInvestigationOverview(view, { rows })

  assert.ok(overview.resourcePathVisual)
  assert.ok(overview.resourcePathVisual.length >= 2)
  assert.equal(overview.resourcePathVisual[0].kind, 'Deployment')
})

test('stats are compact counts not health KPIs', () => {
  const view = baseView()
  const rows = deriveMatchRows(view, view.state.matchedObjects)
  const overview = buildInvestigationOverview(view, { rows })

  assert.ok(typeof overview.stats.signals === 'number')
  assert.ok(typeof overview.stats.evidence === 'number')
  assert.ok(overview.stats.evidence >= 2)
})

test('buildInvestigationOverview tolerates non-array correlation fields', () => {
  const view = baseView({
    state: {
      ...baseView().state,
      causalChain: 'OOMKilled then restart',
      correlation: { unexpected: true },
    },
  })
  const rows = deriveMatchRows(view, view.state.matchedObjects)
  const overview = buildInvestigationOverview(view, { rows })

  assert.equal(overview.phase, 'active')
  assert.ok(overview.verdict.summary)
})
