import { describe, expect, it } from 'vitest'
import {
  contextRecordingLabel,
  evidenceHistoryLine,
  formatBytes,
  formatGap,
  historyDepthLabel,
  historyGapLines,
  historySinceLine,
  normalizeHistorySettings,
  optionsWithValue,
  recordingTone,
  recordingTooltip,
  settingsContextRows,
  shouldShowContextRecording,
} from './eventHistory.js'

const tz = { timeZone: 'UTC' }

describe('History since line (inspector)', () => {
  it('formats the start of history', () => {
    expect(historySinceLine({ since: '2026-10-18T14:02:00Z' }, tz)).toBe('History since Oct 18, 14:02')
  })

  it('is empty without coverage', () => {
    expect(historySinceLine(null, tz)).toBe('')
    expect(historySinceLine({}, tz)).toBe('')
  })

  it('lists gaps as periods Klew was not recording', () => {
    const lines = historyGapLines({
      since: '2026-10-18T14:02:00Z',
      gaps: [
        { from: '2026-10-18T16:00:00Z', to: '2026-10-18T19:00:00Z' },
        { from: '2026-10-18T23:00:00Z', to: '2026-10-19T08:30:00Z' },
      ],
    }, tz)
    expect(lines).toEqual([
      'No history Oct 18 16:00–19:00 (Klew not recording)',
      'No history Oct 18 23:00–Oct 19 08:30 (Klew not recording)',
    ])
  })

  it('ignores malformed gaps', () => {
    expect(formatGap({ from: 'nope', to: '' }, tz)).toBe('')
    expect(historyGapLines({ gaps: [{}] }, tz)).toEqual([])
  })
})

describe('recording indicator', () => {
  it('describes active recording with history and gaps', () => {
    const s = { state: 'recording', since: '2026-10-18T14:02:00Z', gaps: [{}, {}] }
    expect(recordingTooltip(s, tz)).toBe('Recording events · history since Oct 18, 14:02 · 2 gaps')
    expect(recordingTone(s)).toBe('ok')
  })

  it('counts another window as recording', () => {
    expect(recordingTooltip({ state: 'other_window' }, tz)).toBe('Recording events (another Klew window)')
  })

  it('explains why it is not recording', () => {
    expect(recordingTooltip({ state: 'paused' }, tz)).toBe('Not recording — paused')
    expect(recordingTooltip({ state: 'off', reason: 'turned off in Settings' }, tz)).toBe('Not recording — turned off in Settings')
    expect(recordingTooltip({ state: 'not_recordable' }, tz)).toBe('Not recording — RBAC denied')
    expect(recordingTone({ state: 'paused' })).toBe('muted')
    expect(recordingTooltip(null, tz)).toBe('Not recording — event history unavailable')
  })

  it('flags high volume', () => {
    const s = { state: 'recording', highVolume: true }
    expect(recordingTooltip(s, tz)).toContain('recording Warnings only')
    expect(recordingTone(s)).toBe('warn')
  })

  it('shows history depth per context', () => {
    const now = new Date('2026-10-21T15:00:00Z')
    expect(historyDepthLabel('2026-10-18T14:02:00Z', now)).toBe('3d history')
    expect(historyDepthLabel('2026-10-21T10:00:00Z', now)).toBe('5h history')
    expect(historyDepthLabel('2026-10-21T14:30:00Z', now)).toBe('<1h history')
    expect(historyDepthLabel('', now)).toBe('')
    expect(contextRecordingLabel({ state: 'recording', since: '2026-10-18T14:02:00Z' }, now)).toBe('Recording · 3d history')
    expect(contextRecordingLabel({ state: 'off' }, now)).toBe('Not recording')
  })
})

describe('per-context label visibility', () => {
  it('shows only informative rows', () => {
    expect(shouldShowContextRecording({ state: 'recording' })).toBe(true)
    expect(shouldShowContextRecording({ state: 'off', favorite: true })).toBe(true)
    expect(shouldShowContextRecording({ state: 'off', since: '2026-10-18T14:02:00Z' })).toBe(true)
    expect(shouldShowContextRecording({ state: 'off' })).toBe(false)
    expect(shouldShowContextRecording(null)).toBe(false)
  })
})

describe('evidence history line', () => {
  it('is empty when no recorded event was used', () => {
    expect(evidenceHistoryLine({ since: '2026-10-21T09:00:00Z', recordedEvents: 0 })).toBe('')
    expect(evidenceHistoryLine(null)).toBe('')
  })

  it('summarises span and gaps', () => {
    const line = evidenceHistoryLine({
      since: '2026-10-21T09:00:00Z',
      recordedEvents: 3,
      gaps: [{ from: '2026-10-21T10:00:00Z', to: '2026-10-21T11:15:00Z' }],
    }, { now: new Date('2026-10-21T15:00:00Z'), ...tz })
    expect(line).toBe('Includes recorded events from the last 6h · gaps: Oct 21 10:00–11:15')
  })
})

describe('Settings → Event history controls', () => {
  it('defaults to on, 7 days, 500 MB', () => {
    expect(normalizeHistorySettings(undefined)).toEqual({
      available: false, enabled: true, retentionDays: 7, maxMB: 500, sizeBytes: 0, error: '',
    })
  })

  it('keeps saved values and clamps bad ones', () => {
    expect(normalizeHistorySettings({ available: true, enabled: false, retentionDays: 3, maxMB: 250, sizeBytes: 1024 }))
      .toMatchObject({ available: true, enabled: false, retentionDays: 3, maxMB: 250, sizeBytes: 1024 })
    expect(normalizeHistorySettings({ retentionDays: -1, maxMB: 'x' })).toMatchObject({ retentionDays: 7, maxMB: 500 })
    expect(normalizeHistorySettings({ retentionDays: 400 }).retentionDays).toBe(90)
  })

  it('formats current size', () => {
    expect(formatBytes(0)).toBe('0 B')
    expect(formatBytes(2048)).toBe('2 KB')
    expect(formatBytes(5 * 1024 * 1024)).toBe('5.0 MB')
  })

  it('keeps a custom saved value selectable', () => {
    expect(optionsWithValue([1, 7, 30], 10)).toEqual([1, 7, 10, 30])
    expect(optionsWithValue([1, 7, 30], 7)).toEqual([1, 7, 30])
  })

  it('lists history start per context', () => {
    const rows = settingsContextRows([
      { context: 'docker-desktop', state: 'recording', since: '2026-10-18T14:02:00Z', gaps: [{}] },
      { context: 'prod', state: 'not_recordable', reason: 'RBAC denies listing events' },
    ], tz)
    expect(rows).toEqual([
      { context: 'docker-desktop', text: 'Recording · since Oct 18, 14:02 · 1 gap', recording: true },
      { context: 'prod', text: 'Not recording — RBAC denies listing events · no history yet', recording: false },
    ])
  })
})
