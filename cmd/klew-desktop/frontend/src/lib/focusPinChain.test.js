/**
 * Pin → inspect link → pin → inspect link …
 * Proves focus chain navigation stays coherent across key formats.
 */
import { describe, expect, it } from 'vitest'
import {
  buildInspectKey,
  findRowByKey,
  inspectRowForKey,
  isInspectableKey,
  rowKeysMatch,
  synthesizeFocusRow,
} from './matches.js'
import { buildChainRows, buildFocusScope } from './focusScope.js'

const NS = 'klew-lab'

function emptyView(snapshot = {}) {
  return { state: { snapshot }, summary: { namespace: NS } }
}

function catalogFixture() {
  return [
    {
      key: buildInspectKey('Deployment', 'payment-api', NS),
      kind: 'Deployment',
      name: 'payment-api',
      namespace: NS,
      ref: { kind: 'Deployment', name: 'payment-api', namespace: NS },
      ready: 2,
      total: 2,
      status: 'healthy',
    },
    {
      key: buildInspectKey('ReplicaSet', 'payment-api-75495887df', NS),
      kind: 'ReplicaSet',
      name: 'payment-api-75495887df',
      namespace: NS,
      ref: { kind: 'ReplicaSet', name: 'payment-api-75495887df', namespace: NS },
      ownerKind: 'Deployment',
      ownerName: 'payment-api',
    },
    {
      key: buildInspectKey('Pod', 'payment-api-75495887df-abc12', NS),
      kind: 'Pod',
      name: 'payment-api-75495887df-abc12',
      namespace: NS,
      ref: { kind: 'Pod', name: 'payment-api-75495887df-abc12', namespace: NS },
      ownerKind: 'ReplicaSet',
      ownerName: 'payment-api-75495887df',
      ready: 1,
      total: 1,
      status: 'healthy',
    },
    {
      key: buildInspectKey('Service', 'payment-api', NS),
      kind: 'Service',
      name: 'payment-api',
      namespace: NS,
      ref: { kind: 'Service', name: 'payment-api', namespace: NS },
      selector: 'app=payment-api',
    },
    {
      key: buildInspectKey('ConfigMap', 'payment-api-config', NS),
      kind: 'ConfigMap',
      name: 'payment-api-config',
      namespace: NS,
      ref: { kind: 'ConfigMap', name: 'payment-api-config', namespace: NS },
    },
  ]
}

function pinChain(focusRow, catalogRows, view = emptyView()) {
  const scope = buildFocusScope(view, focusRow, { catalogRows })
  const chain = buildChainRows(view, scope, catalogRows)
  return { scope, chain }
}

function simulateInspect(chain, key) {
  expect(isInspectableKey(key, emptyView(), chain)).toBe(true)
  const row = findRowByKey(chain, key)
  expect(row).toBeTruthy()
  return row
}

describe('pin-to-pin focus chain', () => {
  const catalogRows = catalogFixture()

  it('pin 1: investigation-style Deployment key builds full chain with canonical keys', () => {
    const focusRow = synthesizeFocusRow('Deployment/payment-api', NS)
    const { scope, chain } = pinChain(focusRow, catalogRows)

    expect(scope.active).toBe(true)
    expect(rowKeysMatch(scope.focusKey, buildInspectKey('Deployment', 'payment-api', NS))).toBe(true)
    expect(chain.length).toBeGreaterThanOrEqual(3)
    expect(findRowByKey(chain, scope.focusKey)).toBeTruthy()
    expect(findRowByKey(chain, 'Pod/payment-api-75495887df-abc12')).toBeTruthy()
    expect(findRowByKey(chain, 'Service/payment-api')).toBeTruthy()
  })

  it('pin 2: inspect Service from chain then re-pin Pod (pin → link → pin)', () => {
    const deployFocus = synthesizeFocusRow('Deployment/payment-api', NS)
    const { chain: chain1 } = pinChain(deployFocus, catalogRows)

    const serviceRow = simulateInspect(chain1, 'Service/payment-api')
    expect(rowKeysMatch(serviceRow.key, buildInspectKey('Service', 'payment-api', NS))).toBe(true)

    const podFocus = synthesizeFocusRow(serviceRow.key, NS)
    // Re-pin from Service — chain should still include workload context
    const { scope: scope2, chain: chain2 } = pinChain(
      findRowByKey(catalogRows, 'Pod/payment-api-75495887df-abc12'),
      catalogRows,
    )

    expect(scope2.active).toBe(true)
    expect(findRowByKey(chain2, 'Pod/payment-api-75495887df-abc12')).toBeTruthy()
    expect(findRowByKey(chain2, 'Deployment/payment-api')).toBeTruthy()
    expect(podFocus.kind).toBe('Service')
  })

  it('pin 3: Pod focus expands to Deployment + Service sequence', () => {
    const podRow = findRowByKey(catalogRows, buildInspectKey('Pod', 'payment-api-75495887df-abc12', NS))
    const { chain } = pinChain(podRow, catalogRows)

    const kinds = chain.map((r) => r.kind)
    expect(kinds).toContain('Pod')
    expect(kinds).toContain('Deployment')
    expect(kinds).toContain('Service')
    expect(rowKeysMatch(chain[0].key, podRow.key)).toBe(true)
  })

  it('pin 4: inspector link to ConfigMap resolves across key formats', () => {
    const view = emptyView({
      pods: [{
        name: 'payment-api-75495887df-abc12',
        namespace: NS,
        ready: true,
        configMapRefs: ['payment-api-config'],
        ownerRefs: [{ kind: 'ReplicaSet', name: 'payment-api-75495887df' }],
      }],
      workloads: [{ kind: 'Deployment', name: 'payment-api', namespace: NS, ready: 2, replicas: 2 }],
      services: [{ name: 'payment-api', namespace: NS, readyEndpoints: 2, totalEndpoints: 2 }],
      replicaSets: [{ name: 'payment-api-75495887df', namespace: NS, ownerRefs: [{ kind: 'Deployment', name: 'payment-api' }] }],
    })

    const focusRow = synthesizeFocusRow('Pod/payment-api-75495887df-abc12', NS)
    const { chain } = pinChain(focusRow, catalogRows, view)

    const cmKey = buildInspectKey('ConfigMap', 'payment-api-config', NS)
    expect([...chain].some((r) => rowKeysMatch(r.key, cmKey)) || isInspectableKey(cmKey, view, chain)).toBe(true)

    const cmRow = inspectRowForKey(cmKey, view, chain)
      || inspectRowForKey('ConfigMap/payment-api-config', view, catalogRows)
    expect(cmRow?.name).toBe('payment-api-config')
  })

  it('pin 5: three-hop navigation — Deploy → Service → Pod selections stay stable', () => {
    let focusKey = 'Deployment/payment-api'
    let inspectKey = focusKey

    const { chain: c0 } = pinChain(synthesizeFocusRow(focusKey, NS), catalogRows)
    expect(findRowByKey(c0, inspectKey)).toBeTruthy()

    inspectKey = 'Service/payment-api'
    const svc = simulateInspect(c0, inspectKey)
    expect(rowKeysMatch(svc.key, buildInspectKey('Service', 'payment-api', NS))).toBe(true)

    const { chain: c1 } = pinChain(synthesizeFocusRow(inspectKey, NS), catalogRows)
    expect(findRowByKey(c1, 'Service/payment-api')).toBeTruthy()

    inspectKey = buildInspectKey('Pod', 'payment-api-75495887df-abc12', NS)
    const pod = simulateInspect(c1, inspectKey)
    expect(pod.kind).toBe('Pod')

    focusKey = inspectKey
    const { chain: c2 } = pinChain(synthesizeFocusRow(focusKey, NS), catalogRows)
    expect(findRowByKey(c2, focusKey)).toBeTruthy()
    expect(findRowByKey(c2, 'Deployment/payment-api')).toBeTruthy()
  })

  it('pin 6: namespaced focus key survives App-style synthesis (no prod/name corruption)', () => {
    const row = synthesizeFocusRow('Deployment/klew-lab/payment-api', '')
    expect(row.ref.name).toBe('payment-api')
    expect(row.ref.namespace).toBe('klew-lab')
    expect(row.key).toBe(buildInspectKey('Deployment', 'payment-api', 'klew-lab'))
  })
})
