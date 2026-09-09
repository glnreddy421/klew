import { describe, expect, it } from 'vitest'
import { buildManifestTarget } from './manifestTarget.js'

describe('buildManifestTarget', () => {
  const podsNav = {
    kind: 'Pod',
    resourceId: 'v1/pods',
    namespaced: true,
  }

  it('uses nav resourceId when inspect kind matches sidebar selection', () => {
    expect(buildManifestTarget({
      kind: 'Pod',
      name: 'redis-abc',
      namespace: 'klew-lab',
    }, podsNav)).toEqual({
      kind: 'Pod',
      name: 'redis-abc',
      namespace: 'klew-lab',
      resourceId: 'v1/pods',
      clusterScoped: false,
    })
  })

  it('does not reuse nav resourceId when inspecting a different kind', () => {
    expect(buildManifestTarget({
      kind: 'ReplicaSet',
      name: 'payment-api-75495887df',
      namespace: 'klew-lab',
    }, podsNav)).toEqual({
      kind: 'ReplicaSet',
      name: 'payment-api-75495887df',
      namespace: 'klew-lab',
      resourceId: '',
      clusterScoped: false,
    })
  })

  it('prefers inspect row resourceId from catalog entities', () => {
    expect(buildManifestTarget({
      kind: 'ReplicaSet',
      name: 'payment-api-75495887df',
      namespace: 'klew-lab',
      resourceId: 'apps/v1/replicasets',
    }, podsNav)).toEqual({
      kind: 'ReplicaSet',
      name: 'payment-api-75495887df',
      namespace: 'klew-lab',
      resourceId: 'apps/v1/replicasets',
      clusterScoped: false,
    })
  })

  it('marks cluster-scoped kinds regardless of nav selection', () => {
    expect(buildManifestTarget({
      kind: 'Node',
      name: 'docker-desktop',
    }, podsNav)).toEqual({
      kind: 'Node',
      name: 'docker-desktop',
      namespace: '',
      resourceId: '',
      clusterScoped: true,
    })
  })
})
