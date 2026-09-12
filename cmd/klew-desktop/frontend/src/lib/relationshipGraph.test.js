import { describe, expect, it } from 'vitest'
import { buildRelationshipGraph } from './relationshipGraph.js'

describe('buildRelationshipGraph service backends', () => {
  it('links backend rows to target pods not nodes', () => {
    const graph = buildRelationshipGraph({
      center: { kind: 'Service', name: 'payment-api', key: 'Service/klew-lab/payment-api', namespace: 'klew-lab' },
      items: [],
      sections: [{
        id: 'backendAddresses',
        title: 'Backend Addresses',
        group: 'relationships',
        table: {
          columns: ['Addresses', 'Ready', 'Node', 'Zone', 'Target'],
          rows: [
            ['10.244.1.147', 'True', 'desktop-worker', '', 'Pod/payment-api-abc'],
            ['10.244.1.148', 'True', 'desktop-worker', '', 'Pod/payment-api-def'],
          ],
        },
      }],
      inspectNamespace: 'klew-lab',
    })

    const satellites = graph.nodes.filter((n) => !n.isCenter)
    expect(satellites).toHaveLength(2)
    expect(satellites.every((n) => n.kind === 'Pod')).toBe(true)
    expect(graph.edges.some((e) => e.role === 'Backend')).toBe(true)
    expect(graph.nodes.some((n) => n.kind === 'Node')).toBe(false)
  })

  it('links job owner references and spawned pods', () => {
    const graph = buildRelationshipGraph({
      center: { kind: 'Job', name: 'nightly-123', key: 'Job/dtool/nightly-123', namespace: 'dtool' },
      items: [],
      sections: [
        {
          id: 'ownerRefs',
          title: 'Owner References',
          group: 'relationships',
          table: {
            columns: ['Kind', 'Name', 'Namespace', 'UID'],
            rows: [['CronJob', 'nightly', 'dtool', 'uid-1']],
          },
        },
        {
          id: 'pods',
          title: 'Pods',
          group: 'relationships',
          table: {
            columns: ['Name', 'Phase', 'Ready'],
            rows: [['nightly-123-abc', 'Succeeded', 'True']],
          },
        },
      ],
      inspectNamespace: 'dtool',
    })

    expect(graph.nodes.some((n) => n.kind === 'CronJob' && n.name === 'nightly')).toBe(true)
    expect(graph.nodes.some((n) => n.kind === 'Pod' && n.name === 'nightly-123-abc')).toBe(true)
    expect(graph.edges.some((e) => e.role === 'Owner')).toBe(true)
    expect(graph.edges.some((e) => e.role === 'Pod')).toBe(true)
  })

  it('includes endpoint slice nodes with EndpointSlice role', () => {
    const graph = buildRelationshipGraph({
      center: { kind: 'Service', name: 'payment-api', key: 'Service/klew-lab/payment-api', namespace: 'klew-lab' },
      items: [],
      sections: [{
        id: 'endpointSlices',
        title: 'EndpointSlices',
        group: 'relationships',
        table: {
          columns: ['Name', 'Address Type', 'Ready', 'Namespace'],
          rows: [['payment-api-vkxqh', 'IPv4', '3/3', 'klew-lab']],
        },
      }],
      inspectNamespace: 'klew-lab',
    })

    const slice = graph.nodes.find((n) => n.name === 'payment-api-vkxqh')
    expect(slice?.kind).toBe('EndpointSlice')
    expect(graph.edges.find((e) => e.to === slice?.id)?.role).toBe('EndpointSlice')
  })
})
