import { describe, expect, it } from 'vitest'
import { groupResourceFromId, investigableIndex, investigableType, investigateState, rowInvestigateState } from './investigable'

const index = investigableIndex([
  { group: 'apps', resource: 'deployments', kind: 'Deployment', collector: 'workloads' },
  { group: '', resource: 'pods', kind: 'Pod', collector: 'pods' },
  { group: 'cert-manager.io', resource: 'certificates', collector: 'certificates', custom: true },
])

describe('investigable', () => {
  it('parses catalog ids', () => {
    expect(groupResourceFromId('apps/v1/deployments')).toEqual({ group: 'apps', resource: 'deployments' })
    expect(groupResourceFromId('v1/pods')).toEqual({ group: '', resource: 'pods' })
    expect(groupResourceFromId('')).toBeNull()
  })

  it('looks types up by group and resource, any version', () => {
    expect(investigableType(index, 'apps/v1beta2/deployments')?.collector).toBe('workloads')
    expect(investigableType(index, 'v1/pods')?.kind).toBe('Pod')
    expect(investigableType(index, 'coordination.k8s.io/v1/leases')).toBeNull()
    expect(investigableType(index, 'klew/virtual/v1/helmreleases')).toBeNull()
  })

  it('explains why an object cannot be investigated', () => {
    expect(investigateState(index, { resourceId: 'v1/pods', kind: 'Pod', name: 'a', namespace: 'shop' }))
      .toEqual({ ok: true, title: 'Investigate Pod/a in shop' })
    expect(investigateState(index, { resourceId: 'cert-manager.io/v1/certificates', kind: 'Certificate', name: 'c', namespace: 'shop' }).title)
      .toContain('custom collector certificates')
    const lease = investigateState(index, { resourceId: 'coordination.k8s.io/v1/leases', kind: 'Lease', name: 'l', namespace: 'shop' })
    expect(lease.ok).toBe(false)
    expect(lease.title).toContain('no collector for Lease')
    expect(investigateState(index, { resourceId: 'v1/pods', kind: 'Node', name: 'n' }).title).toContain('cluster-scoped')
    expect(investigateState(index, {}, { loaded: false }).ok).toBe(false)
  })

  it('uses the selected type only for rows of that type', () => {
    const deployments = { kind: 'Deployment', resourceId: 'apps/v1/deployments' }
    expect(rowInvestigateState(index, { kind: 'Deployment', name: 'api', namespace: 'shop' }, deployments).ok).toBe(true)
    expect(rowInvestigateState(index, { kind: 'Pod', name: 'p', namespace: 'shop' }, deployments).ok).toBe(true)
    expect(rowInvestigateState(index, { kind: 'Lease', name: 'l', namespace: 'shop' }, deployments).ok).toBe(false)
  })
})
