import { describe, expect, it } from 'vitest'
import { isRoutine, rankFindings, reasonOf } from './findingRanking.js'

const f = (title, severity, extra = {}) => ({ title, severity, reason: reasonOf(title), ...extra })

describe('Key findings ranking', () => {
  it('reads the reason from event templates and labels', () => {
    expect(reasonOf('[Pulled] Container image "busybox" already present')).toBe('Pulled')
    expect(reasonOf('FailedCreate')).toBe('FailedCreate')
    expect(reasonOf('')).toBe('')
  })

  it('drops routine lifecycle events even when they are the most frequent', () => {
    const out = rankFindings([
      f('[Pulled] Container image already present', 'warning', { count: 37 }),
      f('[Scheduled] Successfully assigned klew-lab/<POD>', 'warning', { count: 37 }),
      f('[Started] Container started', 'warning', { count: 37 }),
      f('SuccessfulCreate', 'warning', { count: 4 }),
      f('Completed', 'warning', { count: 3 }),
      f('[FailedCreate] Error creating: admission webhook denied', 'high', { count: 2 }),
    ])
    expect(out.map((x) => x.reason)).toEqual(['FailedCreate'])
  })

  it('orders by severity, then recency, then count', () => {
    const out = rankFindings([
      f('[BackOff] Back-off restarting', 'high', { count: 50, lastSeen: 1000 }),
      f('[OOMKilling] Memory cgroup out of memory', 'critical', { count: 1, lastSeen: 500 }),
      f('[Unhealthy] Readiness probe failed', 'high', { count: 2, lastSeen: 2000 }),
      f('[FailedMount] MountVolume failed', 'critical', { count: 9, lastSeen: 500 }),
    ])
    expect(out.map((x) => x.reason)).toEqual(['FailedMount', 'OOMKilling', 'Unhealthy', 'BackOff'])
  })

  it('keeps Killing only when it points at a failure', () => {
    expect(isRoutine('Killing', '[Killing] Stopping container app')).toBe(true)
    expect(isRoutine('Killing', '[Killing] Container app failed liveness probe, will be restarted')).toBe(false)
  })

  it('returns nothing when only routine or info items remain', () => {
    expect(rankFindings([f('[Pulled] image present', 'warning'), f('Something informational', 'info')])).toEqual([])
  })
})
