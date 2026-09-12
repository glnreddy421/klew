import { describe, expect, it } from 'vitest'
import {
  buildFocusScope,
  buildChainRows,
  catalogRowsToSnapshotShape,
  emptyFocusScope,
} from './focusScope.js'

describe('buildFocusScope with catalog rows', () => {
  const catalogRows = [
    {
      key: 'Deployment/payment-api',
      kind: 'Deployment',
      name: 'payment-api',
      namespace: 'prod',
      ref: { kind: 'Deployment', name: 'payment-api', namespace: 'prod' },
      desiredReplicas: 2,
      readyReplicas: 2,
    },
    {
      key: 'ReplicaSet/payment-api-58f46b4685',
      kind: 'ReplicaSet',
      name: 'payment-api-58f46b4685',
      namespace: 'prod',
      ownerKind: 'Deployment',
      ownerName: 'payment-api',
    },
    {
      key: 'Pod/payment-api-58f46b4685-abc12',
      kind: 'Pod',
      name: 'payment-api-58f46b4685-abc12',
      namespace: 'prod',
      ownerKind: 'ReplicaSet',
      ownerName: 'payment-api-58f46b4685',
      ready: 1,
      total: 1,
      status: 'healthy',
    },
    {
      key: 'Service/payment-api',
      kind: 'Service',
      name: 'payment-api',
      namespace: 'prod',
      selector: 'app=payment-api',
    },
  ]

  it('builds a workload chain from catalog entities without investigation snapshot', () => {
    const focusRow = catalogRows[0]
    const scope = buildFocusScope(emptyView(), focusRow, { catalogRows })

    expect(scope.active).toBe(true)
    expect(scope.relatedKeys.has('Deployment/prod/payment-api')).toBe(true)
    expect(scope.relatedKeys.has('Pod/prod/payment-api-58f46b4685-abc12')).toBe(true)
    expect(scope.relatedKeys.has('Service/prod/payment-api')).toBe(true)
  })

  it('maps catalog rows into chain list rows', () => {
    const focusRow = catalogRows[0]
    const scope = buildFocusScope(emptyView(), focusRow, { catalogRows })
    const chainRows = buildChainRows(emptyView(), scope, catalogRows)

    expect(chainRows.some((r) => r.key === 'Pod/payment-api-58f46b4685-abc12')).toBe(true)
    expect(chainRows.some((r) => r.key === 'Service/payment-api')).toBe(true)
  })

  it('uses namespaced keys for catalog browse focus chains', () => {
    const namespacedCatalog = catalogRows.map((row) => ({
      ...row,
      key: `${row.kind}/klew-lab/${row.name}`,
      namespace: 'klew-lab',
      ref: { ...row.ref, namespace: 'klew-lab' },
    }))
    const focusRow = namespacedCatalog.find((r) => r.kind === 'Pod')
    const scope = buildFocusScope(emptyView(), focusRow, { catalogRows: namespacedCatalog })
    const chainRows = buildChainRows(emptyView(), scope, namespacedCatalog)

    expect(chainRows.length).toBeGreaterThan(0)
    expect(chainRows.some((r) => r.kind === 'Pod' && r.name === 'payment-api-58f46b4685-abc12')).toBe(true)
    expect(chainRows.some((r) => r.kind === 'Deployment')).toBe(true)
  })

  it('handles null snapshot list fields from the backend', () => {
    const focusRow = catalogRows[0]
    const scope = buildFocusScope({
      state: {
        snapshot: {
          pods: null,
          workloads: null,
          services: null,
          replicaSets: null,
          ingresses: null,
        },
      },
    }, focusRow, { catalogRows })

    expect(scope.active).toBe(true)
    expect(scope.relatedKeys.has('Pod/prod/payment-api-58f46b4685-abc12')).toBe(true)
  })

  it('returns a minimal scope for the focus target while catalog data is loading', () => {
    const focusRow = {
      key: 'Deployment/prod/missing',
      kind: 'Deployment',
      name: 'missing',
      ref: { kind: 'Deployment', name: 'missing', namespace: 'prod' },
    }
    const scope = buildFocusScope(emptyView(), focusRow, { catalogRows: [] })
    expect(scope.active).toBe(true)
    expect(scope.relatedKeys.has('Deployment/prod/missing')).toBe(true)
  })
})

describe('catalogRowsToSnapshotShape', () => {
  it('maps pod owner refs from catalog rows', () => {
    const snap = catalogRowsToSnapshotShape([{
      kind: 'Pod',
      name: 'web-abc',
      namespace: 'default',
      ownerKind: 'ReplicaSet',
      ownerName: 'web-123',
    }])
    expect(snap.pods[0].ownerRefs).toEqual([{ kind: 'ReplicaSet', name: 'web-123' }])
  })
})

function emptyView() {
  return { state: { snapshot: {} } }
}
