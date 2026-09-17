import { describe, expect, it } from 'vitest'
import {
  buildInspectKey,
  rowKeysMatch,
  findRowByKey,
  deriveMatchRows,
  inspectRowForKey,
  parseInspectKey,
  scopeStatus,
} from './matches.js'
import { catalogEntityToRow } from './resourceCatalog.js'
import { buildWorkloadKindCardsFromRows } from './workloadOverview.js'

describe('parseInspectKey', () => {
  it('parses two-part investigation keys', () => {
    expect(parseInspectKey('Pod/redis-abc')).toEqual({
      kind: 'Pod',
      name: 'redis-abc',
      namespace: '',
      key: 'Pod/redis-abc',
    })
  })

  it('parses three-part catalog keys with namespace', () => {
    expect(parseInspectKey('ReplicaSet/klew-lab/payment-api-75495887df')).toEqual({
      kind: 'ReplicaSet',
      name: 'payment-api-75495887df',
      namespace: 'klew-lab',
      key: 'ReplicaSet/klew-lab/payment-api-75495887df',
    })
  })

  it('parses cluster-scoped keys without namespace', () => {
    expect(parseInspectKey('Node/docker-desktop')).toEqual({
      kind: 'Node',
      name: 'docker-desktop',
      namespace: '',
      key: 'Node/docker-desktop',
    })
  })
})

describe('buildInspectKey', () => {
  it('includes namespace for namespaced kinds', () => {
    expect(buildInspectKey('ReplicaSet', 'rs-1', 'klew-lab')).toBe('ReplicaSet/klew-lab/rs-1')
  })

  it('omits namespace for cluster-scoped kinds', () => {
    expect(buildInspectKey('Node', 'node-1', 'klew-lab')).toBe('Node/node-1')
  })
})

describe('deriveMatchRows health', () => {
  const view = (snapshot) => ({ state: { snapshot } })

  it('treats ready running pods as healthy even with many restarts', () => {
    const rows = deriveMatchRows(view({
      pods: [{
        name: 'api-abc',
        ready: true,
        phase: 'Running',
        restartCount: 5,
        containers: [{ state: 'running' }],
      }],
    }), [{ ref: { kind: 'Pod', name: 'api-abc' }, score: 1 }])
    expect(rows[0].status).toBe('healthy')
    expect(scopeStatus(rows).label).toBe('HEALTHY')
  })

  it('treats succeeded job pods in scope as healthy', () => {
    const rows = deriveMatchRows(view({
      pods: [{
        name: 'hello-world-123',
        ready: false,
        phase: 'Succeeded',
        restartCount: 0,
        containers: [{ state: 'terminated' }],
      }],
    }), [{ ref: { kind: 'Pod', name: 'hello-world-123' }, score: 1 }])
    expect(rows[0].status).toBe('healthy')
    expect(scopeStatus(rows).label).toBe('HEALTHY')
  })

  it('ignores terminal failed pods when scoring a deployment', () => {
    const rows = deriveMatchRows(view({
      workloads: [{ kind: 'Deployment', name: 'api', ready: 2, replicas: 2 }],
      pods: [
        { name: 'api-old', ready: false, phase: 'Failed', restartCount: 10, ownerRefs: [{ kind: 'ReplicaSet', name: 'api-old' }] },
        { name: 'api-a', ready: true, phase: 'Running', restartCount: 0, ownerRefs: [{ kind: 'ReplicaSet', name: 'api-rs' }] },
        { name: 'api-b', ready: true, phase: 'Running', restartCount: 0, ownerRefs: [{ kind: 'ReplicaSet', name: 'api-rs' }] },
      ],
    }), [{ ref: { kind: 'Deployment', name: 'api' }, score: 1 }])
    expect(rows[0].status).toBe('healthy')
    expect(rows[0].ready).toBe(2)
    expect(rows[0].total).toBe(2)
  })
})

describe('catalog succeeded pods', () => {
  it('treats catalog Running pods as running when ready is derived from phase', () => {
    const rows = [
      catalogEntityToRow({
        kind: 'Pod',
        name: 'payment-api-a',
        statusHint: 'Running',
        resourceId: 'v1/pods',
        containers: [{ name: 'app', ready: true, state: 'running' }],
      }),
    ]
    const cards = buildWorkloadKindCardsFromRows(
      [{ kind: 'Pod', label: 'Pods', resourceId: 'v1/pods', count: 1 }],
      { 'v1/pods': rows },
    )
    expect(cards[0].segments).toEqual([{ label: 'Running', count: 1, tone: 'ok' }])
  })

  it('does not mark workload overview degraded for completed cronjob pods', () => {
    const rows = [
      catalogEntityToRow({ kind: 'Pod', name: 'hello-world-1', statusHint: 'Succeeded', resourceId: 'v1/pods' }),
      catalogEntityToRow({
        kind: 'Pod',
        name: 'payment-api-a',
        statusHint: 'Running',
        resourceId: 'v1/pods',
        containers: [{ name: 'app', ready: true, state: 'running' }],
      }),
    ]
    const cards = buildWorkloadKindCardsFromRows(
      [{ kind: 'Pod', label: 'Pods', resourceId: 'v1/pods', count: 2 }],
      { 'v1/pods': rows },
    )
    const podCard = cards[0]
    expect(podCard.segments.find((s) => s.label === 'Pending')).toBeUndefined()
    expect(podCard.segments.find((s) => s.label === 'Running')?.count).toBe(2)
  })
})

describe('rowKeysMatch', () => {
  it('matches investigation and catalog keys for the same object', () => {
    expect(rowKeysMatch('Deployment/payment-api', 'Deployment/prod/payment-api')).toBe(true)
    expect(rowKeysMatch('Deployment/prod/payment-api', 'Deployment/prod/payment-api')).toBe(true)
    expect(rowKeysMatch('Deployment/prod/payment-api', 'Deployment/staging/payment-api')).toBe(false)
  })

  it('finds rows across key formats', () => {
    const rows = [{ key: 'Service/prod/api', kind: 'Service', name: 'api' }]
    expect(findRowByKey(rows, 'Service/api')?.name).toBe('api')
  })
})

describe('inspectRowForKey', () => {
  it('synthesizes on-demand rows with parsed namespace', () => {
    const row = inspectRowForKey('ReplicaSet/klew-lab/payment-api-75495887df', {}, [])
    expect(row).toMatchObject({
      kind: 'ReplicaSet',
      name: 'payment-api-75495887df',
      namespace: 'klew-lab',
      key: 'ReplicaSet/klew-lab/payment-api-75495887df',
      adhoc: true,
    })
  })
})
