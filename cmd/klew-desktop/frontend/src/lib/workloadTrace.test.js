import { describe, expect, it } from 'vitest'
import { buildWorkloadTrace, isWorkloadTraceKind } from './workloadTrace.js'

const catalogRows = [
  {
    key: 'Deployment/prod/payment-api',
    kind: 'Deployment',
    name: 'payment-api',
    namespace: 'prod',
    ref: { kind: 'Deployment', name: 'payment-api', namespace: 'prod' },
    desiredReplicas: 2,
    readyReplicas: 2,
  },
  {
    key: 'ReplicaSet/prod/payment-api-58f46b4685',
    kind: 'ReplicaSet',
    name: 'payment-api-58f46b4685',
    namespace: 'prod',
    ownerKind: 'Deployment',
    ownerName: 'payment-api',
    ref: { kind: 'ReplicaSet', name: 'payment-api-58f46b4685', namespace: 'prod' },
  },
  {
    key: 'Pod/prod/payment-api-58f46b4685-abc12',
    kind: 'Pod',
    name: 'payment-api-58f46b4685-abc12',
    namespace: 'prod',
    ownerKind: 'ReplicaSet',
    ownerName: 'payment-api-58f46b4685',
    ready: 1,
    total: 1,
    status: 'healthy',
    ref: { kind: 'Pod', name: 'payment-api-58f46b4685-abc12', namespace: 'prod' },
  },
  {
    key: 'Service/prod/payment-api',
    kind: 'Service',
    name: 'payment-api',
    namespace: 'prod',
    selector: 'app=payment-api',
    ref: { kind: 'Service', name: 'payment-api', namespace: 'prod' },
  },
  {
    key: 'Ingress/prod/payment-api',
    kind: 'Ingress',
    name: 'payment-api',
    namespace: 'prod',
    serviceName: 'payment-api',
    ref: { kind: 'Ingress', name: 'payment-api', namespace: 'prod' },
  },
]

describe('isWorkloadTraceKind', () => {
  it('includes core workload and network kinds', () => {
    expect(isWorkloadTraceKind('Pod')).toBe(true)
    expect(isWorkloadTraceKind('Deployment')).toBe(true)
    expect(isWorkloadTraceKind('ConfigMap')).toBe(false)
  })
})

describe('buildWorkloadTrace', () => {
  it('builds an ordered trace path from catalog rows without investigation', () => {
    const focusRow = catalogRows.find((r) => r.kind === 'Pod')
    const trace = buildWorkloadTrace({ focusRow, catalogRows })

    expect(trace.nodes.length).toBeGreaterThanOrEqual(4)
    expect(trace.nodes.map((n) => n.kind)).toEqual([
      'Ingress',
      'Service',
      'Deployment',
      'ReplicaSet',
      'Pod',
    ])
    expect(trace.edges).toHaveLength(trace.nodes.length - 1)
    expect(trace.nodes.find((n) => n.isFocus)?.kind).toBe('Pod')
  })

  it('marks the selected deployment as focus', () => {
    const focusRow = catalogRows.find((r) => r.kind === 'Deployment')
    const trace = buildWorkloadTrace({ focusRow, catalogRows })

    expect(trace.nodes.some((n) => n.kind === 'Service')).toBe(true)
    expect(trace.nodes.find((n) => n.isFocus)?.kind).toBe('Deployment')
  })

  it('returns empty trace for non-workload kinds', () => {
    const trace = buildWorkloadTrace({
      focusRow: { kind: 'ConfigMap', name: 'cfg', ref: { kind: 'ConfigMap', name: 'cfg' } },
      catalogRows,
    })
    expect(trace.nodes).toEqual([])
  })
})
