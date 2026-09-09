import { describe, expect, it } from 'vitest'
import {
  buildInspectKey,
  inspectRowForKey,
  parseInspectKey,
} from './matches.js'

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
