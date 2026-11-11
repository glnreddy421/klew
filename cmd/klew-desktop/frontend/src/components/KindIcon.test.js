import { describe, expect, it } from 'vitest'
import { createElement as h } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { KindIcon, kindGlyph } from './KindIcon.jsx'
import { GENERIC_GLYPH, KIND_GLYPHS } from './icons/kindGlyphs.jsx'

const BUILT_IN_KINDS = [
  'Pod', 'Deployment', 'StatefulSet', 'DaemonSet', 'ReplicaSet', 'Job', 'CronJob',
  'HorizontalPodAutoscaler', 'PodDisruptionBudget', 'PriorityClass',
  'Service', 'Ingress', 'IngressClass', 'Endpoints', 'EndpointSlice', 'NetworkPolicy',
  'Gateway', 'HTTPRoute',
  'ConfigMap', 'Secret',
  'PersistentVolume', 'PersistentVolumeClaim', 'StorageClass', 'Volume', 'VolumeAttachment', 'CSIDriver',
  'ServiceAccount', 'Role', 'ClusterRole', 'RoleBinding', 'ClusterRoleBinding', 'User', 'Group',
  'Namespace', 'Node', 'ResourceQuota', 'LimitRange', 'Lease', 'Event', 'CustomResourceDefinition',
  'ValidatingWebhookConfiguration', 'MutatingWebhookConfiguration', 'HelmRelease',
]

describe('KindIcon', () => {
  it('has an own glyph for every built-in kind', () => {
    const missing = BUILT_IN_KINDS.filter((k) => !KIND_GLYPHS[k])
    expect(missing).toEqual([])
  })

  it('resolves short names, plurals and case to the same glyph', () => {
    expect(kindGlyph('deploy')).toBe(KIND_GLYPHS.Deployment)
    expect(kindGlyph('pods')).toBe(KIND_GLYPHS.Pod)
    expect(kindGlyph('statefulset')).toBe(KIND_GLYPHS.StatefulSet)
    expect(kindGlyph('helmrelease')).toBe(KIND_GLYPHS.HelmRelease)
  })

  it('falls back to the generic glyph for custom kinds', () => {
    expect(kindGlyph('Certificate')).toBe(GENERIC_GLYPH)
    expect(kindGlyph('')).toBe(GENERIC_GLYPH)
  })

  it('renders an inline svg that inherits the text colour', () => {
    const html = renderToStaticMarkup(h(KindIcon, { kind: 'Deployment', size: 14 }))
    expect(html).toContain('<svg')
    expect(html).toContain('stroke="currentColor"')
    expect(html).toContain('width="14px"')
    expect(html).toContain('aria-label="Deployment"')
    expect(html).not.toContain('<img')
  })
})
