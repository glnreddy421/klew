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

  it('parses prefixed secret refs using inspect namespace, not data keys', () => {
    expect(parseObjectRefCell('secret:tls-cert', 'prod')?.key).toBe('Secret/prod/tls-cert')
    expect(parseObjectRefCell('secret:oci-publishing-gitlab-token/GITLAB_TOKEN', 'dtool')?.key)
      .toBe('Secret/dtool/oci-publishing-gitlab-token')
  })

  it('does not link pod template enum values as pods', () => {
    const ref = resolveInspectRef('ClusterFirst', {
      fieldKey: 'DNS Policy',
      section: { id: 'podTemplate', title: 'Pod Template' },
      groupId: 'spec',
      inspectNamespace: 'klew-lab',
    })
    expect(ref).toBeNull()
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

  it('parses prefixed secret refs from object column using inspect namespace', () => {
    const ref = resolveInspectRef('secret:oci-publishing-gitlab-token/GITLAB_TOKEN', {
      columnName: 'object',
      section: { id: 'env', title: 'Environment' },
      inspectNamespace: 'dtool',
    })
    expect(ref?.key).toBe('Secret/dtool/oci-publishing-gitlab-token')
  })

  it('links pod names from Job summary fields', () => {
    const ref = resolveInspectRef('hello-world-29384756', {
      fieldKey: 'Pod',
      inspectNamespace: 'klew-lab',
    })
    expect(ref?.key).toBe('Pod/klew-lab/hello-world-29384756')
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
