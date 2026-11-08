import { describe, expect, it } from 'vitest'
import {
  ALL_KINDS,
  kindsFromGroups,
  objectsOfKind,
  queryForTargets,
  reconcileTargets,
  targetLabel,
  targetsLabel,
  kindHeading,
  searchTargets,
} from './investigateTarget'

const groups = [
  { kind: 'Deployment', collector: 'workloads', names: ['api', 'web'] },
  { kind: 'PersistentVolumeClaim', collector: 'persistentvolumeclaims', names: ['data'] },
  { kind: 'Certificate', collector: 'certificates', custom: true, names: ['web-tls'] },
  { kind: 'Empty', names: [] },
  null,
]

describe('investigateTarget', () => {
  it('lists the engine kinds in its order, marking custom ones', () => {
    expect(kindsFromGroups(groups)).toEqual([
      { kind: 'Deployment', count: 2, custom: false },
      { kind: 'PersistentVolumeClaim', count: 1, custom: false },
      { kind: 'Certificate', count: 1, custom: true },
    ])
    expect(kindsFromGroups(null)).toEqual([])
  })

  it('narrows objects by kind', () => {
    expect(objectsOfKind(groups, 'Deployment').map((o) => o.key)).toEqual(['Deployment/api', 'Deployment/web'])
    expect(objectsOfKind(groups, ALL_KINDS)).toHaveLength(4)
  })

  it('targets exactly the chosen resources, any kind the engine lists', () => {
    expect(queryForTargets([], groups)).toBe('')
    expect(queryForTargets(['Deployment/api'], groups)).toBe('Deployment/api')
    expect(queryForTargets(['Deployment/api', 'Certificate/web-tls', 'Deployment/gone'], groups)).toBe('Deployment/api,Certificate/web-tls')
  })

  it('drops targets that are no longer listed', () => {
    expect(reconcileTargets(['Deployment/api', 'Deployment/gone'], groups)).toEqual(['Deployment/api'])
    expect(reconcileTargets(null, groups)).toEqual([])
  })

  it('labels the picker', () => {
    expect(targetsLabel([])).toBe('All resources')
    expect(targetsLabel(['Deployment/api'])).toBe('api')
    expect(targetsLabel(['Deployment/api', 'Service/web', 'ConfigMap/c'])).toBe('api +2')
  })

  it('fuzzy-searches targets by name and kind, best first', () => {
    const many = [
      { kind: 'Deployment', names: ['billing', 'mobile-lb', 'ledger'] },
      { kind: 'Service', names: ['gateway'] },
      { kind: 'ServiceAccount', names: ['a'] },
      { kind: 'PersistentVolumeClaim', names: ['data'] },
    ]
    expect(searchTargets(many, 'bil').map((o) => o.key)).toEqual(['Deployment/billing', 'Deployment/mobile-lb'])
    expect(searchTargets(many, 'svc').map((o) => o.key).slice(0, 2)).toEqual(['Service/gateway', 'ServiceAccount/a'])
    expect(searchTargets(many, 'pvc')[0].key).toBe('PersistentVolumeClaim/data')
    expect(searchTargets(many, 'deploy/led').map((o) => o.key)).toEqual(['Deployment/ledger'])
    expect(searchTargets(many, 'svc gw').map((o) => o.key)).toEqual(['Service/gateway'])
    expect(searchTargets(many, 'zzz')).toEqual([])
    const [hit] = searchTargets(many, 'blng')
    expect([...hit.nameHits].sort((a, b) => a - b)).toEqual([0, 2, 5, 6])
  })

  it('turns kinds into readable headings', () => {
    expect(kindHeading('Deployment')).toBe('Deployments')
    expect(kindHeading('PersistentVolumeClaim')).toBe('Persistent volume claims')
    expect(kindHeading('Ingress')).toBe('Ingresses')
    expect(kindHeading('NetworkPolicy')).toBe('Network policies')
    expect(kindHeading('HTTPRoute')).toBe('HTTPRoutes')
    expect(kindHeading('')).toBe('')
  })

  it('labels the target', () => {
    expect(targetLabel('shop', [])).toBe('namespace shop')
    expect(targetLabel('shop', ['Deployment/api'])).toBe('Deployment/api in shop')
    expect(targetLabel('shop', ['a', 'b'])).toBe('2 resources in shop')
  })
})
