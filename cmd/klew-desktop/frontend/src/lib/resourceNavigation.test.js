import { describe, expect, it } from 'vitest'
import {
  CATEGORY_OVERVIEW_KIND,
  entitiesForKind,
  filterCategoriesByQuery,
  filterEntitiesBySearch,
  pickDefaultKindSelection,
  resolveKindSelection,
  expandGroup,
  toggleExpandedGroup,
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

  it('picks workloads overview by default', () => {
    expect(pickDefaultKindSelection(sampleTree, false)).toEqual({
      groupId: 'workloads',
      kind: CATEGORY_OVERVIEW_KIND,
      resourceId: null,
    })
  })

  it('returns no entities for overview selection', () => {
    expect(entitiesForKind(sampleTree, 'workloads', CATEGORY_OVERVIEW_KIND)).toEqual([])
  })

  it('keeps overview selection after tree refresh', () => {
    expect(resolveKindSelection(sampleTree, {
      groupId: 'workloads',
      kind: CATEGORY_OVERVIEW_KIND,
    }, false)).toEqual({
      groupId: 'workloads',
      kind: CATEGORY_OVERVIEW_KIND,
      resourceId: null,
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

describe('toggleExpandedGroup', () => {
  it('opens a category without closing the others', () => {
    expect([...toggleExpandedGroup(new Set(['workloads']), 'network')]).toEqual(['workloads', 'network'])
  })

  it('closes only the toggled category', () => {
    expect([...toggleExpandedGroup(new Set(['workloads', 'network']), 'network')]).toEqual(['workloads'])
  })

  it('opens from fully collapsed', () => {
    expect([...toggleExpandedGroup(new Set(), 'storage')]).toEqual(['storage'])
  })

  it('does not mutate the previous set', () => {
    const prev = new Set(['workloads'])
    toggleExpandedGroup(prev, 'network')
    expect([...prev]).toEqual(['workloads'])
  })
})

describe('expandGroup', () => {
  it('adds the selected category and keeps the rest open', () => {
    expect([...expandGroup(new Set(['workloads']), 'network')]).toEqual(['workloads', 'network'])
  })

  it('returns the same set when already open (no re-render)', () => {
    const prev = new Set(['workloads', 'network'])
    expect(expandGroup(prev, 'network')).toBe(prev)
  })
})

describe('filterCategoriesByQuery', () => {
  const cats = [
    { id: 'workloads', kinds: [
      { kind: 'Pod', label: 'Pods', resource: 'pods' },
      { kind: 'Deployment', label: 'Deployments', resource: 'deployments', group: 'apps' },
    ] },
    { id: 'network', kinds: [
      { kind: 'Service', label: 'Services', resource: 'services' },
      { kind: 'NetworkPolicy', label: 'NetworkPolicies', resource: 'networkpolicies', group: 'networking.k8s.io' },
    ] },
  ]

  it('returns everything for an empty query', () => {
    expect(filterCategoriesByQuery(cats, '  ').categories).toBe(cats)
  })

  it('matches labels and drops empty categories', () => {
    const { categories, best } = filterCategoriesByQuery(cats, 'deploy')
    expect(categories.map((c) => c.id)).toEqual(['workloads'])
    expect(categories[0].kinds.map((k) => k.kind)).toEqual(['Deployment'])
    expect(best.kindGroup.kind).toBe('Deployment')
  })

  it('understands kubectl short names', () => {
    expect(filterCategoriesByQuery(cats, 'svc').best.kindGroup.kind).toBe('Service')
    expect(filterCategoriesByQuery(cats, 'po').best.kindGroup.kind).toBe('Pod')
  })

  it('prefers a prefix match over a substring match', () => {
    const { best } = filterCategoriesByQuery(cats, 'net')
    expect(best.kindGroup.kind).toBe('NetworkPolicy')
  })
})
