import { describe, expect, it } from 'vitest'
import { buildOverviewBrief, causalSteps, failingPods, podFailureReason, podHeatmap, podWorkload, warningPatterns } from './overviewBrief.js'

const healthyPod = { name: 'api-1', namespace: 'shop', phase: 'Running', ready: true, restartCount: 0, containers: [{ name: 'api', ready: true, state: 'running' }] }
const crashPod = {
  name: 'payment-api-1', namespace: 'shop', phase: 'Running', ready: false, restartCount: 41,
  containers: [{ name: 'api', ready: false, state: 'waiting', reason: 'CrashLoopBackOff', lastReason: 'OOMKilled', lastExitCode: 137 }],
}
const pendingPod = { name: 'billing-1', namespace: 'shop', phase: 'Pending', ready: false, restartCount: 0, unschedulable: 'Insufficient memory', containers: [] }
const doneJobPod = { name: 'hello-1', namespace: 'shop', phase: 'Succeeded', ready: false, restartCount: 0, containers: [{ name: 'c', state: 'terminated', reason: 'Completed' }] }

function view(state) {
  return { state }
}

describe('podFailureReason', () => {
  it('reports the waiting reason with the last crash and exit code', () => {
    expect(podFailureReason(crashPod)).toMatchObject({ reason: 'CrashLoopBackOff', detail: 'last: OOMKilled, exit 137' })
  })

  it('reports why a pod cannot be scheduled', () => {
    expect(podFailureReason(pendingPod)).toMatchObject({ reason: 'Pending', detail: 'Insufficient memory' })
  })
})

describe('failingPods', () => {
  it('lists only unhealthy pods, worst first, never completed Job pods', () => {
    const rows = failingPods([healthyPod, pendingPod, doneJobPod, crashPod])
    expect(rows.map((r) => r.name)).toEqual(['payment-api-1', 'billing-1'])
    expect(rows[0]).toMatchObject({ severity: 'critical', restarts: 41, key: 'Pod/shop/payment-api-1' })
  })
})

describe('warningPatterns', () => {
  it('drops info patterns and ranks by severity then count', () => {
    const rows = warningPatterns({
      eventTemplates: [
        { id: 'e1', template: 'Scheduled pod', count: 90, severity: 'info', pods: [] },
        { id: 'e3', template: '[Pulled] Container image already present', count: 32, severity: 'warning', pods: [] },
        { id: 'e2', template: 'Back-off restarting', count: 41, severity: 'warning', pods: ['a'] },
      ],
      templates: [{ id: 'l1', template: 'panic: out of memory', count: 3, severity: 'critical', pods: ['a', 'b'] }],
    })
    expect(rows.map((r) => r.text)).toEqual(['panic: out of memory', 'Back-off restarting'])
    expect(rows[0]).toMatchObject({ source: 'log', pods: 2 })
  })
})

describe('buildOverviewBrief', () => {
  it('leads with the failing pod and the likely trigger', () => {
    const brief = buildOverviewBrief(view({
      verdict: { likelyTrigger: 'memory limit lowered', confidence: 0.9 },
      causalChain: ['limit lowered', 'OOMKilled', 'CrashLoopBackOff'],
      snapshot: { pods: [healthyPod, crashPod] },
    }))
    expect(brief.status).toBe('failing')
    expect(brief.headline).toBe('payment-api-1 · CrashLoopBackOff')
    expect(brief.trigger).toBe('memory limit lowered')
    expect(brief.confidenceText).toBe('High')
    expect(brief.chain).toHaveLength(3)
  })

  it('says healthy in one line, with recovered incidents counted', () => {
    const brief = buildOverviewBrief(view({
      verdict: { likelyTrigger: 'old trigger', confidence: 0.9 },
      causalChain: ['a', 'b'],
      nextChecks: ['Inspect the failing pod api-1'],
      snapshot: { pods: [healthyPod, doneJobPod] },
      incidents: [{ start: '2026-11-11T07:00:00Z', end: '2026-11-11T07:45:00Z', leadingReason: 'Unhealthy', count: 227, recovered: true }],
    }))
    expect(brief.status).toBe('recovered')
    expect(brief.headline).toBe('Healthy now · 1/1 pods ready · 1 recovered incident')
    expect(brief.trigger).toBe('')
    expect(brief.chain).toEqual([])
    expect(brief.earlier).toHaveLength(1)
    expect(brief.nextChecks).toEqual([])
  })

  it('uses verdict next checks when the state has none', () => {
    const brief = buildOverviewBrief(view({
      verdict: { recommendedNextChecks: ['kubectl -n shop describe pod payment-api-1'] },
      snapshot: { pods: [crashPod] },
    }))
    expect(brief.nextChecks).toEqual(['kubectl -n shop describe pod payment-api-1'])
  })
})

describe('causalSteps', () => {
  it('drops routine lifecycle steps from the chain', () => {
    expect(causalSteps(['CreateContainerConfigError +2h35m', 'Completed +2h35m', 'CrashLoopBackOff +2h36m']))
      .toEqual(['CreateContainerConfigError +2h35m', 'CrashLoopBackOff +2h36m'])
  })
})

describe('podHeatmap', () => {
  const rsOwned = (name, rs, hash, extra = {}) => ({
    ...healthyPod,
    name,
    labels: { 'pod-template-hash': hash },
    ownerRefs: [{ kind: 'ReplicaSet', name: rs }],
    ...extra,
  })

  it('maps a ReplicaSet pod to its Deployment', () => {
    expect(podWorkload(rsOwned('api-7f-x', 'api-7f', '7f'))).toEqual({ kind: 'Deployment', name: 'api' })
    expect(podWorkload({ ownerRefs: [{ kind: 'Job', name: 'hello-1' }] })).toEqual({ kind: 'Job', name: 'hello-1' })
    expect(podWorkload({})).toEqual({ kind: 'Pod', name: 'Standalone pods' })
  })

  it('groups pods by workload, worst group first, and counts impact', () => {
    const map = podHeatmap([
      rsOwned('web-1', 'web-5c', '5c'),
      rsOwned('pay-1', 'pay-9d', '9d', { ...crashPod, name: 'pay-1', labels: { 'pod-template-hash': '9d' }, ownerRefs: [{ kind: 'ReplicaSet', name: 'pay-9d' }] }),
      rsOwned('pay-2', 'pay-9d', '9d'),
      { ...doneJobPod, ownerRefs: [{ kind: 'Job', name: 'hello-1' }] },
    ])
    expect(map.groups.map((g) => g.name)).toEqual(['pay', 'web', 'hello-1'])
    expect(map.groups[0].cells.map((c) => c.state)).toEqual(['failing', 'healthy'])
    expect(map.counts).toEqual({ failing: 1, degraded: 0, healthy: 2, done: 1 })
    expect(map.impactedPct).toBe(33)
  })
})
