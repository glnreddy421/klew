import { describe, expect, it } from 'vitest'
import {
  appendLines,
  canStreamKind,
  MAX_STREAM_TARGETS,
  streamTargetsFor,
  describeSources,
  filterLines,
  formatLineTime,
  logSpecKey,
  logTabTitle,
  podColor,
  podTargets,
  POD_COLORS,
  sourceKey,
  streamSources,
} from './logStream'

const line = (seq, pod, text, severity = 'info', container = 'app') => ({
  seq, pod, container, namespace: 'shop', text, severity,
})

describe('appendLines', () => {
  it('keeps only the newest lines', () => {
    const prev = appendLines([], [line(1, 'a', 'one'), line(2, 'a', 'two')])
    const next = appendLines(prev, [line(3, 'a', 'three'), line(4, 'a', 'four')], 3)
    expect(next.map((l) => l.seq)).toEqual([2, 3, 4])
  })

  it('orders lines by log time across pods, ties by arrival', () => {
    const at = (seq, pod, time) => ({ ...line(seq, pod, 'x'), timestamp: `2026-11-06T10:00:${time}Z` })
    // Pod a replays its tail first, then pod b's older lines arrive.
    const prev = appendLines([], [at(1, 'a', '05'), at(2, 'a', '20')])
    const next = appendLines(prev, [at(4, 'b', '30'), at(3, 'b', '10'), at(5, 'b', '20')])
    expect(next.map((l) => l.seq)).toEqual([1, 3, 2, 5, 4])
    // Newer lines take the fast path and keep the existing array contents.
    const later = appendLines(next, [at(6, 'a', '40')])
    expect(later.map((l) => l.seq)).toEqual([1, 3, 2, 5, 4, 6])
  })

  it('returns the same array for an empty batch', () => {
    const prev = [line(1, 'a', 'one')]
    expect(appendLines(prev, [])).toBe(prev)
  })
})

describe('filterLines', () => {
  const lines = [
    line(1, 'api-1', 'GET /healthz 200'),
    line(2, 'api-1', 'connection refused', 'high'),
    line(3, 'api-2', 'panic: nil map', 'critical'),
    line(4, 'api-2', 'GET /orders 200', 'info', 'sidecar'),
  ]

  it('returns every line without filters', () => {
    expect(filterLines(lines)).toBe(lines)
  })

  it('includes and excludes by regex or text', () => {
    expect(filterLines(lines, { include: 'GET' }).map((l) => l.seq)).toEqual([1, 4])
    expect(filterLines(lines, { exclude: 'healthz|orders' }).map((l) => l.seq)).toEqual([2, 3])
    // An invalid regex falls back to plain text.
    expect(filterLines(lines, { include: 'nil map(' })).toEqual([])
  })

  it('keeps only high and critical lines for errors only', () => {
    expect(filterLines(lines, { errorsOnly: true }).map((l) => l.seq)).toEqual([2, 3])
  })

  it('hides muted pod/container sources', () => {
    const muted = new Set(['api-2/app'])
    expect(filterLines(lines, { muted }).map((l) => l.seq)).toEqual([1, 2, 4])
  })
})

describe('streamSources', () => {
  it('lists sources in arrival order with counts', () => {
    const got = streamSources([line(1, 'b', 'x'), line(2, 'a', 'y'), line(3, 'b', 'z')])
    expect(got.map((s) => [s.key, s.count])).toEqual([['b/app', 2], ['a/app', 1]])
  })
})

describe('podColor', () => {
  it('is stable and within the palette', () => {
    const key = sourceKey(line(1, 'api-7f9c', 'x'))
    expect(podColor(key)).toBe(podColor(key))
    for (const k of ['a/app', 'b/app', 'payment-api-0/app', '']) {
      expect(podColor(k)).toBeGreaterThanOrEqual(0)
      expect(podColor(k)).toBeLessThan(POD_COLORS)
    }
  })
})

describe('formatLineTime', () => {
  const now = new Date(2026, 10, 6, 18, 0, 0)

  it('formats today with milliseconds', () => {
    const d = new Date(2026, 10, 6, 9, 5, 7, 42)
    expect(formatLineTime(d.toISOString(), now)).toBe('09:05:07.042')
  })

  it('adds the date for earlier days', () => {
    const d = new Date(2026, 8, 8, 20, 35, 44)
    expect(formatLineTime(d.toISOString(), now)).toBe('Sep 8 20:35:44')
  })

  it('returns empty for missing or bad timestamps', () => {
    expect(formatLineTime('')).toBe('')
    expect(formatLineTime('not a time')).toBe('')
  })
})

describe('describeSources', () => {
  it('describes one or several sources', () => {
    expect(describeSources([])).toBe('')
    expect(describeSources([{ namespace: 'shop', label: 'Deployment/api', follows: true }]))
      .toBe('Deployment/api in shop (follows new pods)')
    expect(describeSources([
      { namespace: 'shop', label: 'Pod/a' },
      { namespace: 'prod', label: 'Pod/b' },
    ])).toBe('Pod/a in shop and 1 more')
  })
})

describe('podTargets', () => {
  it('builds one target per namespace, without duplicates', () => {
    expect(podTargets('shop', ['a', 'b', 'a', ''])).toEqual([{ namespace: 'shop', pods: ['a', 'b'] }])
    expect(podTargets('', ['a'])).toEqual([])
    expect(podTargets('shop', [])).toEqual([])
  })
})

describe('streamTargetsFor', () => {
  it('groups pods by namespace and keeps workloads separate', () => {
    const { targets, skipped } = streamTargetsFor([
      { kind: 'Pod', name: 'a', namespace: 'shop' },
      { kind: 'Pod', name: 'b', namespace: 'shop' },
      { kind: 'Pod', name: 'a', namespace: 'shop' },
      { kind: 'Pod', name: 'c', namespace: 'prod' },
      { kind: 'Deployment', name: 'api', namespace: 'shop' },
    ])
    expect(targets).toEqual([
      { namespace: 'shop', pods: ['a', 'b'] },
      { namespace: 'prod', pods: ['c'] },
      { namespace: 'shop', kind: 'Deployment', name: 'api' },
    ])
    expect(skipped).toBe(0)
  })

  it('skips kinds without pods and resources without a namespace', () => {
    const { targets, skipped } = streamTargetsFor([
      { kind: 'ConfigMap', name: 'settings', namespace: 'shop' },
      { kind: 'Pod', name: 'a', namespace: '' },
      { kind: 'Service', name: 'web', namespace: 'shop' },
    ])
    expect(targets).toEqual([{ namespace: 'shop', kind: 'Service', name: 'web' }])
    expect(skipped).toBe(2)
  })

  it('stops at the source limit and counts the rest as skipped', () => {
    const many = Array.from({ length: MAX_STREAM_TARGETS + 3 }, (_, i) => ({ kind: 'Deployment', name: `d${i}`, namespace: 'shop' }))
    const { targets, skipped } = streamTargetsFor(many)
    expect(targets).toHaveLength(MAX_STREAM_TARGETS)
    expect(skipped).toBe(3)
  })

  it('knows which kinds stream', () => {
    expect(canStreamKind('Pod')).toBe(true)
    expect(canStreamKind('StatefulSet')).toBe(true)
    expect(canStreamKind('CronJob')).toBe(false)
  })
})

describe('logSpecKey and logTabTitle', () => {
  it('produces identical keys regardless of pod or target order', () => {
    const k1 = logSpecKey({
      targets: [
        { namespace: 'shop', pods: ['b', 'a'] },
        { namespace: 'shop', kind: 'Deployment', name: 'checkout' },
      ],
    })
    const k2 = logSpecKey({
      targets: [
        { namespace: 'shop', kind: 'Deployment', name: 'checkout' },
        { namespace: 'shop', pods: ['a', 'b', 'a'] },
      ],
    })
    expect(k1).toBe(k2)
    expect(logSpecKey({ targets: [{ namespace: 'shop', pods: ['a'] }], container: 'app' }))
      .not.toBe(logSpecKey({ targets: [{ namespace: 'shop', pods: ['a'] }], container: 'sidecar' }))
  })

  it('names tabs for one pod, one workload, or multiple sources', () => {
    expect(logTabTitle({ targets: [{ namespace: 'shop', pods: ['api-7f9c-x2'] }] }))
      .toBe('api-7f9c-x2')
    expect(logTabTitle({ targets: [{ namespace: 'shop', kind: 'Deployment', name: 'checkout' }] }))
      .toBe('Deployment/checkout')
    expect(logTabTitle({ targets: [{ namespace: 'shop', pods: ['a', 'b', 'c'] }] }))
      .toBe('3 sources')
    expect(logTabTitle({
      targets: [
        { namespace: 'shop', pods: ['a'] },
        { namespace: 'shop', kind: 'Service', name: 'gateway' },
      ],
    })).toBe('2 sources')
  })
})
