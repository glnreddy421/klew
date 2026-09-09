import { describe, expect, it } from 'vitest'
import { buildWorkloadMetrics } from './workloadMetrics.js'

function mockTree(counts) {
  return {
    categories: [{
      id: 'workloads',
      label: 'Workloads',
      kinds: Object.entries(counts).map(([kind, count]) => ({
        kind,
        label: `${kind}s`,
        count,
        countState: { state: 'loaded', value: count },
        accessState: 'allowed',
        resourceId: `id/${kind.toLowerCase()}`,
      })),
    }],
  }
}

describe('buildWorkloadMetrics', () => {
  it('builds kind tiles from catalog tree counts', () => {
    const metrics = buildWorkloadMetrics({
      tree: mockTree({
        Pod: 12,
        Deployment: 4,
        StatefulSet: 2,
        DaemonSet: 2,
        Job: 1,
        CronJob: 3,
      }),
      view: {},
      clusterStatus: { nodes: { total: 3, ready: 3, notReady: 0 } },
    })
    expect(metrics.kindTiles.map((t) => [t.kind, t.count])).toEqual([
      ['Pod', 12],
      ['Deployment', 4],
      ['StatefulSet', 2],
      ['DaemonSet', 2],
      ['Job', 1],
      ['CronJob', 3],
    ])
    expect(metrics.totalWorkloadCount).toBe(24)
  })

  it('summarizes investigation pod and replica health', () => {
    const metrics = buildWorkloadMetrics({
      tree: mockTree({ Pod: 2, Deployment: 1 }),
      view: {
        state: {
          snapshot: {
            pods: [
              { name: 'a', ready: true, phase: 'Running' },
              { name: 'b', ready: false, phase: 'Pending' },
            ],
            workloads: [{ kind: 'Deployment', ready: 2, replicas: 3 }],
            metrics: {
              available: true,
              cpuUsageMillicores: 500,
              cpuRequestMillicores: 1000,
              memUsageMi: 256,
              memRequestMi: 512,
            },
          },
        },
      },
      clusterStatus: { nodes: { total: 2, ready: 2, notReady: 0 } },
    })
    expect(metrics.pods.total).toBe(2)
    expect(metrics.pods.healthy).toBe(1)
    expect(metrics.replicas).toEqual({ ready: 2, desired: 3 })
    expect(metrics.resources.available).toBe(true)
    expect(metrics.healthTone).toBe('warn')
  })
})
