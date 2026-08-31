import { describe, expect, it } from 'vitest'
import {
  BUILTIN_PRESENTATION,
  presentationKey,
  isBuiltinPresentationKey,
  builtinCategoryForKey,
  defaultNamespaced,
} from './resourcePresentation.js'
import {
  buildCatalogScopeTree,
  formatKindCount,
} from './resourceCatalog.js'

function mockDescriptor(overrides) {
  return {
    id: 'apps/v1/deployments',
    group: 'apps',
    version: 'v1',
    apiVersion: 'apps/v1',
    resource: 'deployments',
    kind: 'Deployment',
    namespaced: true,
    source: 'builtin',
    discovered: true,
    accessState: 'allowed',
    count: { state: 'loaded', count: 4 },
    ...overrides,
  }
}

function mockCatalog(descriptors) {
  return {
    generatedAt: '2026-01-01T00:00:00Z',
    resources: descriptors,
  }
}

describe('resourcePresentation', () => {
  it('uses group/resource presentation keys', () => {
    expect(presentationKey('', 'pods')).toBe('/pods')
    expect(presentationKey('apps', 'deployments')).toBe('apps/deployments')
    expect(presentationKey('networking.k8s.io', 'ingresses')).toBe('networking.k8s.io/ingresses')
  })

  it('maps persistent volumes to storage not cluster', () => {
    expect(builtinCategoryForKey('/persistentvolumes')).toBe('storage')
    expect(builtinCategoryForKey('rbac.authorization.k8s.io/clusterroles')).toBe('security')
    expect(builtinCategoryForKey('/nodes')).toBe('cluster')
  })

  it('recognizes builtin keys', () => {
    expect(isBuiltinPresentationKey('/pods')).toBe(true)
    expect(isBuiltinPresentationKey('argoproj.io/applications')).toBe(false)
  })

  it('defaults namespaced scope for built-ins', () => {
    expect(defaultNamespaced({ resource: 'pods' })).toBe(true)
    expect(defaultNamespaced({ resource: 'nodes' })).toBe(false)
  })

  it('groups HPA and webhooks under Config', () => {
    expect(builtinCategoryForKey('autoscaling/horizontalpodautoscalers')).toBe('config')
    expect(builtinCategoryForKey('policy/poddisruptionbudgets')).toBe('config')
    expect(builtinCategoryForKey('admissionregistration.k8s.io/mutatingwebhookconfigurations')).toBe('config')
    expect(builtinCategoryForKey('scheduling.k8s.io/priorityclasses')).toBe('config')
  })

  it('maps VPA to config when present', () => {
    expect(builtinCategoryForKey('autoscaling.k8s.io/verticalpodautoscalers')).toBe('config')
  })

  it('maps ingress classes to network', () => {
    expect(builtinCategoryForKey('networking.k8s.io/ingressclasses')).toBe('network')
    expect(defaultNamespaced({ resource: 'ingressclasses' })).toBe(false)
  })
})

describe('buildCatalogScopeTree presentation merge', () => {
  it('places deployments under Workloads with count', () => {
    const tree = buildCatalogScopeTree(
      mockCatalog([mockDescriptor({})]),
      [{ key: 'Deployment/a', kind: 'Deployment', name: 'a', resourceId: 'apps/v1/deployments' }],
      [],
    )
    const workloads = tree.categories.find((c) => c.id === 'workloads')
    const dep = workloads.kinds.find((k) => k.kind === 'Deployment')
    expect(dep.label).toBe('Deployments')
    expect(dep.matchCount).toBe(1)
    expect(formatKindCount(dep)).toBe('1')
  })

  it('places HPA under Config with short label', () => {
    const tree = buildCatalogScopeTree(
      mockCatalog([mockDescriptor({
        id: 'autoscaling/v2/horizontalpodautoscalers',
        group: 'autoscaling',
        version: 'v2',
        apiVersion: 'autoscaling/v2',
        resource: 'horizontalpodautoscalers',
        kind: 'HorizontalPodAutoscaler',
      })]),
      [],
      [],
    )
    const config = tree.categories.find((c) => c.id === 'config')
    const hpa = config.kinds.find((k) => k.kind === 'HorizontalPodAutoscaler')
    expect(hpa.label).toBe('HPA')
  })

  it('omits VPA when not discovered', () => {
    const tree = buildCatalogScopeTree(mockCatalog([]), [], [])
    const config = tree.categories.find((c) => c.id === 'config')
    expect(config.kinds.some((k) => k.kind === 'VerticalPodAutoscaler')).toBe(false)
  })

  it('shows VPA under Config when discovered', () => {
    const tree = buildCatalogScopeTree(
      mockCatalog([mockDescriptor({
        id: 'autoscaling.k8s.io/v1/verticalpodautoscalers',
        group: 'autoscaling.k8s.io',
        version: 'v1',
        apiVersion: 'autoscaling.k8s.io/v1',
        resource: 'verticalpodautoscalers',
        kind: 'VerticalPodAutoscaler',
        source: 'extension',
      })]),
      [],
      [],
    )
    const config = tree.categories.find((c) => c.id === 'config')
    const vpa = config.kinds.find((k) => k.kind === 'VerticalPodAutoscaler')
    expect(vpa?.label).toBe('VPA')
  })

  it('shows lock for forbidden built-in', () => {
    const tree = buildCatalogScopeTree(
      mockCatalog([mockDescriptor({
        id: 'apps/v1/daemonsets',
        resource: 'daemonsets',
        kind: 'DaemonSet',
        accessState: 'forbidden',
        count: { state: 'forbidden' },
      })]),
      [],
      [],
    )
    const workloads = tree.categories.find((c) => c.id === 'workloads')
    const ds = workloads.kinds.find((k) => k.kind === 'DaemonSet')
    expect(formatKindCount(ds)).toBe('🔒')
  })

  it('uses catalog count when investigation scope has zero matches', () => {
    const tree = buildCatalogScopeTree(
      mockCatalog([mockDescriptor({
        id: 'v1/nodes',
        group: '',
        version: 'v1',
        apiVersion: 'v1',
        resource: 'nodes',
        kind: 'Node',
        namespaced: false,
        count: { state: 'loaded', count: 2 },
      })]),
      [],
      [],
    )
    const cluster = tree.categories.find((c) => c.id === 'cluster')
    const nodes = cluster.kinds.find((k) => k.kind === 'Node')
    expect(formatKindCount(nodes)).toBe('2')
  })

  it('places CRDs under Custom Resources', () => {
    const tree = buildCatalogScopeTree(
      mockCatalog([mockDescriptor({
        id: 'argoproj.io/v1alpha1/applications',
        group: 'argoproj.io',
        version: 'v1alpha1',
        apiVersion: 'argoproj.io/v1alpha1',
        resource: 'applications',
        kind: 'Application',
        source: 'extension',
      })]),
      [],
      [],
    )
    const custom = tree.categories.find((c) => c.id === 'custom')
    expect(custom.kinds.some((k) => k.kind === 'Application')).toBe(true)
  })
})
