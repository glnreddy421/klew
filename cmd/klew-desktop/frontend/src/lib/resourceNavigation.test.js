import { describe, expect, it } from 'vitest'
import {
  entitiesForKind,
  filterEntitiesBySearch,
  pickDefaultKindSelection,
  resolveKindSelection,
  visibleCategories,
} from './resourceNavigation.js'

const sampleTree = {
  count: 4,
  categories: [
    {
      id: 'workloads',
      label: 'Workloads',
      count: 3,
      kinds: [
        {
          kind: 'Deployment',
          label: 'Deployments',
          resourceId: 'apps/v1/deployments',
          builtin: true,
          discovered: true,
          matchCount: 2,
          count: 2,
          countState: { state: 'loaded', value: 2 },
          items: [{ row: { key: 'Deployment/a', name: 'a' } }, { row: { key: 'Deployment/b', name: 'b' } }],
        },
        {
          kind: 'Pod',
          label: 'Pods',
          resourceId: 'v1/pods',
          builtin: true,
          discovered: true,
          matchCount: 1,
          count: 1,
          countState: { state: 'loaded', value: 1 },
          items: [{ row: { key: 'Pod/p1', name: 'p1' } }],
        },
        {
          kind: 'ReplicaSet',
          label: 'ReplicaSets',
          resourceId: 'apps/v1/replicasets',
          builtin: true,
          discovered: true,
          matchCount: 0,
          count: 0,
          countState: { state: 'loaded', value: 0 },
          items: [],
        },
      ],
    },
    {
      id: 'network',
      label: 'Network',
      count: 0,
      kinds: [
        {
          kind: 'Service',
          label: 'Services',
          resourceId: 'v1/services',
          builtin: true,
          discovered: true,
          matchCount: 0,
          count: 0,
          countState: { state: 'loaded', value: 0 },
          items: [],
        },
      ],
    },
  ],
}

describe('resourceNavigation', () => {
  it('keeps empty built-in kinds visible even when showEmpty is false', () => {
    const cats = visibleCategories(sampleTree, false)
    expect(cats.length).toBeGreaterThanOrEqual(1)
    expect(cats[0].kinds.map((k) => k.kind)).toEqual(['Deployment', 'Pod', 'ReplicaSet'])
    const network = cats.find((c) => c.id === 'network')
    expect(network?.kinds.map((k) => k.kind)).toContain('Service')
  })

  it('shows empty built-in kinds always', () => {
    const cats = visibleCategories(sampleTree, false)
    expect(cats[0].kinds.map((k) => k.kind)).toContain('ReplicaSet')
  })

  it('returns entities for selected kind', () => {
    const ents = entitiesForKind(sampleTree, 'workloads', 'Deployment')
    expect(ents).toHaveLength(2)
    expect(ents[0].name).toBe('a')
  })

  it('filters entities by search', () => {
    const ents = entitiesForKind(sampleTree, 'workloads', 'Deployment')
    expect(filterEntitiesBySearch(ents, 'b')).toHaveLength(1)
  })

  it('picks default kind with entities', () => {
    expect(pickDefaultKindSelection(sampleTree, false)).toEqual({
      groupId: 'workloads',
      kind: 'Deployment',
      resourceId: 'apps/v1/deployments',
    })
  })

  it('keeps valid empty built-in selection', () => {
    expect(resolveKindSelection(sampleTree, { groupId: 'network', kind: 'Service' }, false)).toEqual({
      groupId: 'network',
      kind: 'Service',
      resourceId: 'v1/services',
    })
  })
})
