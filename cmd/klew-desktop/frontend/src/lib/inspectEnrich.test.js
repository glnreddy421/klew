import { describe, expect, it } from 'vitest'
import { parseObjectRefCell, resolveInspectRef } from './inspectEnrich.js'

describe('resolveInspectRef', () => {
  it('parses Kind/name refs', () => {
    const ref = resolveInspectRef('Deployment/payment', { inspectNamespace: 'prod' })
    expect(ref?.key).toBe('Deployment/prod/payment')
  })

  it('parses Kind/namespace/name refs', () => {
    const ref = resolveInspectRef('PersistentVolumeClaim/prod/data', {})
    expect(ref?.key).toBe('PersistentVolumeClaim/prod/data')
  })

  it('links storage class field values', () => {
    const ref = resolveInspectRef('standard', {
      fieldKey: 'Storage Class',
      inspectNamespace: 'default',
    })
    expect(ref?.key).toBe('StorageClass/standard')
  })

  it('links owner reference table rows', () => {
    const ref = resolveInspectRef('payment', {
      row: ['Deployment', 'payment', 'prod', 'uid-1'],
      columns: ['Kind', 'Name', 'Namespace', 'UID'],
      columnIndex: 1,
      inspectNamespace: 'prod',
    })
    expect(ref?.key).toBe('Deployment/prod/payment')
  })

  it('parses prefixed secret refs', () => {
    expect(parseObjectRefCell('secret:tls-cert/prod')?.key).toBe('Secret/prod/tls-cert')
  })

  it('does not treat tolerations count as a node', () => {
    const ref = resolveInspectRef('2', {
      fieldKey: 'Tolerations',
      section: { id: 'nodeAssignment', title: 'Node Assignment' },
      groupId: 'relationships',
    })
    expect(ref).toBeNull()
  })

  it('links actual node name from node assignment', () => {
    const ref = resolveInspectRef('desktop-worker', {
      fieldKey: 'Node',
      section: { id: 'nodeAssignment', title: 'Node Assignment' },
      groupId: 'relationships',
    })
    expect(ref?.key).toBe('Node/desktop-worker')
  })

  it('parses target pod refs from backend address rows', () => {
    const ref = resolveInspectRef('Pod/payment-api-abc', {
      columnName: 'Target',
      section: { id: 'backendAddresses', title: 'Backend Addresses' },
      groupId: 'relationships',
      inspectNamespace: 'klew-lab',
    })
    expect(ref?.key).toBe('Pod/klew-lab/payment-api-abc')
  })
})
