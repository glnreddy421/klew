import { describe, expect, it } from 'vitest'
import { chainRecovered, chainWindow, formatClockRange, incidentRows } from './incidents.js'

const tz = { timeZone: 'UTC' }

describe('incidents', () => {
  it('formats rows newest first with status', () => {
    const rows = incidentRows([
      { start: '2026-10-21T20:15:00Z', end: '2026-10-21T20:33:00Z', leadingReason: 'FailedCreate', recovered: true, count: 4 },
      { start: '2026-10-21T21:05:00Z', end: '2026-10-21T21:05:00Z', leadingReason: 'BackOff', recovered: false, count: 1 },
    ], tz)
    expect(rows.map((r) => [r.range, r.reason, r.status])).toEqual([
      ['21:05', 'BackOff', 'ongoing'],
      ['20:15–20:33', 'FailedCreate', 'recovered'],
    ])
  })

  it('labels the chain window', () => {
    const chain = { nodes: [{ timestamp: '2026-10-21T20:15:00Z' }, { timestamp: '2026-10-21T20:33:00Z' }, {}] }
    expect(chainWindow(chain, tz)).toBe('20:15–20:33')
    expect(formatClockRange('2026-10-21T20:15:10Z', '2026-10-21T20:15:50Z', tz)).toBe('20:15')
  })

  it('marks a chain recovered only when all incidents recovered and no step is newer', () => {
    const chain = { nodes: [{ timestamp: '2026-10-21T20:15:00Z' }, { timestamp: '2026-10-21T20:33:00Z' }] }
    const done = [{ end: '2026-10-21T20:33:00Z', recovered: true }]
    expect(chainRecovered(chain, done)).toBe(true)
    expect(chainRecovered(chain, [{ end: '2026-10-21T20:33:00Z', recovered: false }])).toBe(false)
    const newer = { nodes: [...chain.nodes, { timestamp: '2026-10-21T21:05:00Z' }] }
    expect(chainRecovered(newer, done)).toBe(false)
    expect(chainRecovered(chain, [])).toBe(false)
  })
})
