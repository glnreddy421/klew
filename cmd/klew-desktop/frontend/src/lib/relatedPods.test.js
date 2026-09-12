import { describe, expect, it } from 'vitest'
import { collectRelatedPods } from './relatedPods.js'

describe('collectRelatedPods', () => {
  it('merges snapshot pods and live detail pod tables', () => {
    const pods = collectRelatedPods({
      kind: 'Deployment',
      namespace: 'klew-lab',
      relatedPods: [{
        key: 'Pod/klew-lab/redis-old',
        name: 'redis-old',
        namespace: 'klew-lab',
        phase: 'Running',
        ready: 1,
        total: 1,
        status: 'healthy',
      }],
      sections: [{
        id: 'pods',
        title: 'Pods',
        table: {
          columns: ['Name', 'Phase', 'Ready', 'Node'],
          rows: [['redis-new', 'Running', 'True', 'node-a']],
        },
      }],
    })

    expect(pods.map((p) => p.name).sort()).toEqual(['redis-new', 'redis-old'])
    expect(pods[0].key).toContain('klew-lab')
  })

  it('returns empty for Pod inspect target', () => {
    expect(collectRelatedPods({ kind: 'Pod', name: 'redis-abc' })).toEqual([])
  })
})
