import { describe, expect, it } from 'vitest'
import { createElement as h } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import {
  actorBadge,
  changesTargetFor,
  diffRows,
  historyFooter,
  podRevisionLine,
  recentChangeBadge,
  relativeTime,
  replicaSetRevision,
  revisionLabel,
  withChangesTab,
} from './workloadChanges.js'
import { entryFreshness, REVALIDATE_MS, changesCacheKey, peekChanges, storeChanges, clearChangesCache, MAX_ENTRIES } from './changesCache.js'
import { consumeInspectorTab, requestInspectorTab } from './inspectorTabRequest.js'
import { ChangeDiffTable, ChangesTab, PodRevisionLine, RecentChangeBadge } from '../components/inspect/WorkloadChanges.jsx'

const now = new Date('2026-10-21T21:00:00Z')
const wc = {
  kind: 'Deployment', name: 'payment-api', namespace: 'klew-lab', accessState: 'allowed', historyLimit: 10,
  currentRevision: 14, lastChangedAt: '2026-10-21T20:48:00Z',
  changes: [
    { revision: 14, time: '2026-10-21T20:48:00Z', source: 'rollout', actor: 'kubectl-edit', current: true,
      summary: 'image v2.3.1 → v2.4.0, env DB_POOL_SIZE 20 → 5', objectName: 'payment-api-7d9',
      diffs: [
        { path: 'containers[app].image', before: 'repo/pay:v2.3.1', after: 'repo/pay:v2.4.0', kind: 'changed', area: 'image' },
        { path: 'containers[app].env.DB_PASS', before: 'secret/db-v1:password', after: 'secret/db-v2:password', kind: 'changed', area: 'env' },
        { path: 'containers[app].env.NEW_FLAG', after: 'on', kind: 'added', area: 'env' },
      ] },
    { revision: 13, time: '2026-10-20T10:00:00Z', source: 'rollback', rollbackTo: 11, summary: 'Rollback to rev 11 template', objectName: 'payment-api-5c4' },
    { revision: 12, time: '2026-10-19T10:00:00Z', source: 'rollout', summary: 'Oldest retained revision', objectName: 'payment-api-abc' },
  ],
}

describe('Changes timeline render', () => {
  it('renders newest first with revision, current marker, source, actor, and footer', () => {
    const html = renderToStaticMarkup(h(ChangesTab, { state: { data: wc }, now }))
    expect(html.indexOf('rev 14')).toBeLessThan(html.indexOf('rev 13'))
    expect(html).toContain('12m ago')
    expect(html).toContain('current')
    expect(html).toContain('rollback')
    expect(html).toContain('kubectl')
    expect(html).toContain('image v2.3.1 → v2.4.0')
    expect(html).toContain('Older revisions not retained by the cluster (revisionHistoryLimit = 10).')
  })

  it('shows no actor badge when the actor is unknown', () => {
    const html = renderToStaticMarkup(h(ChangesTab, { state: { data: { ...wc, changes: [wc.changes[2]] } }, now }))
    expect(html).not.toContain('changes-actor')
  })

  it('expanded diff shows field, before, after — secret refs as refs', () => {
    const html = renderToStaticMarkup(h(ChangeDiffTable, { change: wc.changes[0] }))
    expect(html).toContain('containers[app].image')
    expect(html).toContain('secret/db-v2:password')
    expect(diffRows(wc.changes[0])[2]).toEqual(['containers[app].env.NEW_FLAG', '—', 'on'])
  })

  it('empty, RBAC-denied, and loading states', () => {
    expect(renderToStaticMarkup(h(ChangesTab, { state: { data: { ...wc, changes: [] } } })))
      .toContain('No changes recorded by the cluster for this workload.')
    expect(renderToStaticMarkup(h(ChangesTab, { state: { data: { accessState: 'denied', accessMessage: 'No permission to read revision history (replicasets list)' } } })))
      .toContain('No permission to read revision history (replicasets list)')
    expect(renderToStaticMarkup(h(ChangesTab, { state: { loading: true } }))).toContain('Loading revision history')
  })
})

describe('badges and lines', () => {
  it('Summary badge only for changes within 24h', () => {
    expect(recentChangeBadge(wc, now)).toBe('Changed 12m ago (rev 14)')
    expect(recentChangeBadge({ ...wc, lastChangedAt: '2026-10-19T20:00:00Z' }, now)).toBeNull()
    expect(recentChangeBadge({ ...wc, accessState: 'denied' }, now)).toBeNull()
    expect(renderToStaticMarkup(h(RecentChangeBadge, { state: { data: wc }, now }))).toContain('Changed 12m ago (rev 14)')
  })

  it('Pod revision line: behind vs current vs no owner', () => {
    const behind = podRevisionLine({ accessState: 'allowed', revision: 13, currentRevision: 14, behind: true,
      behindSummary: 'image', currentSince: '2026-10-21T20:48:00Z' }, now)
    expect(behind).toEqual({ text: 'Running rev 13 · current is rev 14 (image changed 12m ago)', behind: true })
    expect(podRevisionLine({ accessState: 'allowed', revision: 14, currentRevision: 14 }, now))
      .toEqual({ text: 'Running current revision (rev 14)', behind: false })
    expect(podRevisionLine({ accessState: 'allowed', revision: 0 }, now)).toBeNull()
    const html = renderToStaticMarkup(h(PodRevisionLine, { state: { data: { accessState: 'allowed', revision: 13, currentRevision: 14, behind: true } }, now }))
    expect(html).toContain('is-behind')
  })

  it('ReplicaSet revision line', () => {
    expect(replicaSetRevision(wc, 'payment-api-7d9')?.text).toBe('Revision 14 of deployment/payment-api (current)')
    expect(replicaSetRevision(wc, 'unknown-rs')).toBeNull()
  })

  it('labels', () => {
    expect(actorBadge('')).toBeNull()
    expect(actorBadge('kubectl-rollout')).toBe('kubectl')
    expect(actorBadge('helm')).toBe('helm')
    expect(actorBadge('my-operator')).toBe('other')
    expect(revisionLabel({ revision: 14, helmRevision: 3 })).toBe('rev 14 · helm rev 3')
    expect(relativeTime('2026-10-21T20:59:30Z', now)).toBe('just now')
    expect(historyFooter({ kind: 'HelmRelease' })).toContain('Helm')
  })
})

describe('inspector wiring', () => {
  it('adds the Changes tab after Summary for workloads only', () => {
    const tabs = [{ id: 'summary' }, { id: 'spec' }]
    expect(withChangesTab(tabs, 'Deployment').map((t) => t.id)).toEqual(['summary', 'changes', 'spec'])
    expect(withChangesTab(tabs, 'Pod')).toBe(tabs)
    expect(withChangesTab(tabs, 'HelmRelease')).toBe(tabs)
  })

  it('ReplicaSets read their owning Deployment history', () => {
    expect(changesTargetFor({ kind: 'Deployment', name: 'pay', generation: 7 })).toEqual({ kind: 'Deployment', name: 'pay', generation: 7 })
    expect(changesTargetFor({ kind: 'ReplicaSet', name: 'payment-api-7d9f8c6b5' })).toEqual({ kind: 'Deployment', name: 'payment-api' })
    expect(changesTargetFor({ kind: 'ReplicaSet', name: 'x', relationships: [{ kind: 'Deployment', name: 'owner' }] }))
      .toEqual({ kind: 'Deployment', name: 'owner' })
    expect(changesTargetFor({ kind: 'Pod', name: 'p' })).toBeNull()
  })

  it('one-shot tab request (Overview pin → Changes)', () => {
    requestInspectorTab('Deployment', 'payment-api', 'changes')
    expect(consumeInspectorTab('Deployment', 'other')).toBeNull()
    expect(consumeInspectorTab('Deployment', 'payment-api')).toBe('changes')
    expect(consumeInspectorTab('Deployment', 'payment-api')).toBeNull()
  })
})

describe('changes cache', () => {
  it('invalidates on a new generation and revalidates after 60s', () => {
    const entry = { data: { generation: 7 }, at: 1_000 }
    expect(entryFreshness(entry, { generation: 7, now: 1_000 + 1000 })).toBe('fresh')
    expect(entryFreshness(entry, { generation: 8, now: 1_000 + 1000 })).toBe('invalid')
    expect(entryFreshness(entry, { generation: 0, now: 1_000 + REVALIDATE_MS + 1 })).toBe('stale')
    expect(entryFreshness(null)).toBe('missing')
  })

  it('is an LRU keyed per cluster and object', () => {
    clearChangesCache()
    const k = (i) => changesCacheKey('cfg|ctx', 'Deployment', 'ns', `d${i}`)
    for (let i = 0; i <= MAX_ENTRIES; i++) storeChanges(k(i), { i })
    expect(peekChanges(k(0))).toBeNull()
    expect(peekChanges(k(MAX_ENTRIES))?.data.i).toBe(MAX_ENTRIES)
    expect(changesCacheKey('cfg|other', 'Deployment', 'ns', 'd1')).not.toBe(k(1))
  })
})
