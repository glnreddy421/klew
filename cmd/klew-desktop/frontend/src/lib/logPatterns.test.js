/**
 * Drain + logPatterns tests.
 * Run: node --test src/lib/logPatterns.test.js src/lib/drain.test.js
 */
import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  createDrain,
  tokenizeForDrain,
  mergeSimilarClusters,
  formatTemplate,
  DRAIN_WILDCARD,
} from './drain.js'
import {
  extractLogPatterns,
  normalizeTemplate,
  tokenizeWords,
  extractAttributes,
  rankTopWordsTfIdf,
} from './logPatterns.js'
import { correlateLogPatternsToTimeline } from './logCorrelation.js'

test('tokenizeForDrain masks IPs', () => {
  const toks = tokenizeForDrain('dial tcp 10.0.2.14:5432: connect: connection refused')
  assert.ok(toks.includes(DRAIN_WILDCARD) || toks.includes('***'))
  assert.ok(toks.includes('connection') || toks.includes('refused') || toks.some((t) => t.includes('refused')))
})

test('Drain merges same-length lines that differ in variables', () => {
  const d = createDrain({ similarityThreshold: 0.5, depth: 4 })
  d.add(tokenizeForDrain('dial tcp 10.0.2.14:5432: connect: connection refused'), { weight: 1 })
  d.add(tokenizeForDrain('dial tcp 10.0.2.15:5432: connect: connection refused'), { weight: 1 })
  d.add(tokenizeForDrain('dial tcp 10.0.2.19:5432: connect: connection refused'), { weight: 1 })
  const clusters = d.getClusters()
  assert.equal(clusters.length, 1, `expected 1 cluster, got ${clusters.length}: ${clusters.map((c) => c.templateStr).join(' | ')}`)
  assert.equal(clusters[0].count, 3)
  assert.match(formatTemplate(clusters[0].template), /connection|refused|dial/i)
})

test('Drain keeps dissimilar same-length messages apart', () => {
  const d = createDrain({ similarityThreshold: 0.5 })
  d.add(tokenizeForDrain('connection refused to postgres'), { weight: 1 })
  d.add(tokenizeForDrain('listening on port ready health'), { weight: 1 })
  const clusters = d.getClusters()
  assert.ok(clusters.length >= 2)
})

test('mergeSimilarClusters joins near-duplicate length variants', () => {
  const a = {
    id: 'a',
    template: ['connection', 'refused', 'postgres'],
    count: 5,
    severity: 'high',
    pods: new Set(['p1']),
    samples: [],
    times: [],
  }
  const b = {
    id: 'b',
    template: ['connection', 'refused', 'from', 'postgres'],
    count: 3,
    severity: 'high',
    pods: new Set(['p2']),
    samples: [],
    times: [],
  }
  const merged = mergeSimilarClusters([a, b], { jaccardThreshold: 0.5 })
  assert.equal(merged.length, 1)
  assert.equal(merged[0].count, 8)
})

test('normalizeTemplate uses *** placeholders', () => {
  const t = normalizeTemplate('dial tcp 10.0.2.14:5432: connect: connection refused')
  assert.match(t, /\*\*\*/)
  assert.ok(!t.includes('10.0.2.14'))
})

test('tokenizeWords drops stopwords', () => {
  const words = tokenizeWords('connection refused to the postgres host')
  assert.ok(words.includes('connection'))
  assert.ok(words.includes('refused'))
  assert.ok(!words.includes('the'))
})

test('extractAttributes finds key=value', () => {
  const keys = extractAttributes('GetCartAsync called with userId=abc request.id=9')
  assert.ok(keys.includes('userId'))
})

test('rankTopWordsTfIdf promotes emerging terms over chatter', () => {
  const docs = []
  for (let i = 0; i < 20; i++) {
    docs.push({ tokens: ['health', 'ready', 'listening'], ts: i, weight: 1 })
  }
  for (let i = 20; i < 30; i++) {
    docs.push({
      tokens: ['connection', 'refused', 'postgres', 'health'],
      ts: i,
      weight: 1,
    })
  }
  const ranked = rankTopWordsTfIdf(docs, { maxWords: 5 })
  assert.ok(ranked[0].score > 0)
  const top = ranked.map((w) => w.word)
  assert.ok(top.includes('refused') || top.includes('postgres') || top.includes('connection'))
})

test('extractLogPatterns uses Drain templates', () => {
  const now = Date.now()
  const evidence = [
    {
      sourceType: 'log',
      severity: 'high',
      pod: 'payment-api-abc',
      container: 'app',
      message: 'payment-api-abc/app: dial tcp 10.0.2.14:5432: connect: connection refused',
      timestamp: new Date(now - 2000).toISOString(),
    },
    {
      sourceType: 'log',
      severity: 'high',
      pod: 'payment-api-abc',
      container: 'app',
      message: 'payment-api-abc/app: dial tcp 10.0.2.15:5432: connect: connection refused',
      timestamp: new Date(now - 1000).toISOString(),
    },
    {
      sourceType: 'log',
      severity: 'high',
      pod: 'payment-api-abc',
      container: 'app',
      message: 'payment-api-abc/app: dial tcp 10.0.2.19:5432: connect: connection refused',
      timestamp: new Date(now - 500).toISOString(),
    },
    {
      sourceType: 'log',
      severity: 'info',
      pod: 'payment-api-abc',
      container: 'app',
      message: 'payment-api-abc/app: GET /health 200 12ms userId=u1',
      timestamp: new Date(now).toISOString(),
    },
  ]
  const out = extractLogPatterns(evidence, {
    pods: ['payment-api-abc'],
    errorOnly: false,
  })
  assert.ok(out.window.patternCount > 0)
  const refused = out.templates.find((t) => /refused|dial|connect/i.test(t.template))
  assert.ok(refused, `expected refused/dial template, got ${out.templates.map((t) => t.template).join(' || ')}`)
  assert.ok(refused.count >= 3)
  assert.ok(out.words.some((w) => w.score > 0))
  assert.ok(out.attributes.some((a) => a.key === 'userId'))
})

test('correlateLogPatternsToTimeline emits LOG+EVENT signals', () => {
  const now = Date.now()
  const evidence = [
    {
      sourceType: 'log',
      severity: 'high',
      pod: 'payment-api-xyz',
      container: 'app',
      message: 'payment-api-xyz/app: dial tcp 10.0.2.14:5432: connect: connection refused',
      timestamp: new Date(now - 30_000).toISOString(),
    },
    {
      sourceType: 'log',
      severity: 'high',
      pod: 'payment-api-xyz',
      container: 'app',
      message: 'payment-api-xyz/app: dial tcp 10.0.2.14:5432: connect: connection refused',
      timestamp: new Date(now - 20_000).toISOString(),
    },
    {
      sourceType: 'log',
      severity: 'high',
      pod: 'payment-api-xyz',
      container: 'app',
      message: 'payment-api-xyz/app: dial tcp 10.0.2.14:5432: connect: connection refused',
      timestamp: new Date(now - 10_000).toISOString(),
    },
  ]
  const timeline = [
    {
      timestamp: new Date(now - 45_000).toISOString(),
      type: 'k8s_event',
      severity: 'warning',
      reason: 'BackOff',
      message: 'Back-off restarting failed container',
      sourceKind: 'Pod',
      sourceName: 'payment-api-xyz',
    },
  ]
  const { signals, byEventKey } = correlateLogPatternsToTimeline(evidence, timeline)
  assert.ok(signals.length >= 1)
  assert.ok(Object.keys(byEventKey).length >= 1)
})
