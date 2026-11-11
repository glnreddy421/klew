import { describe, expect, it } from 'vitest'
import { groupWarnings } from './recentWarnings.js'
import { buildInspectKey } from './matches.js'

function warning(reason, name, lastSeen, extra = {}) {
  return {
    reason,
    message: `${reason} on ${name}`,
    count: 1,
    lastSeen,
    involvedObject: { kind: 'Pod', name, namespace: 'shop' },
    ...extra,
  }
}

describe('groupWarnings', () => {
  it('groups by reason, newest group first, with summed counts', () => {
    const groups = groupWarnings([
      warning('BackOff', 'api-1', '2026-11-11T16:00:00Z', { count: 4 }),
      warning('FailedScheduling', 'worker-1', '2026-11-11T16:05:00Z', { count: 2 }),
      warning('FailedScheduling', 'worker-2', '2026-11-11T16:03:00Z', { count: 3 }),
    ])
    expect(groups.map((g) => [g.reason, g.count, g.objectCount])).toEqual([
      ['FailedScheduling', 5, 2],
      ['BackOff', 4, 1],
    ])
    expect(groups[0].message).toBe('FailedScheduling on worker-1')
    expect(groups[0].objects[0].key).toBe(buildInspectKey('Pod', 'worker-1', 'shop'))
  })

  it('caps objects per group and groups overall', () => {
    const events = []
    for (let i = 0; i < 5; i++) {
      events.push(warning('FailedScheduling', `w-${i}`, `2026-11-11T16:0${i}:00Z`))
    }
    for (let i = 0; i < 8; i++) {
      events.push(warning(`Reason${i}`, `p-${i}`, '2026-11-11T15:00:00Z'))
    }
    const groups = groupWarnings(events, { maxGroups: 3, maxObjects: 2 })
    expect(groups).toHaveLength(3)
    expect(groups[0].objects.map((o) => o.name)).toEqual(['w-4', 'w-3'])
    expect(groups[0].moreObjects).toBe(3)
  })
})
