import { describe, expect, it } from 'vitest'
import {
  canStepInspectHistory,
  EMPTY_INSPECT_HISTORY,
  stepInspectHistory,
  visitInspectKey,
} from './inspectHistory.js'

function visitAll(keys) {
  return keys.reduce(visitInspectKey, EMPTY_INSPECT_HISTORY)
}

describe('inspectHistory', () => {
  it('records visits and ignores repeats of the current object', () => {
    const h = visitAll(['pod', 'pod', 'rs', 'deploy'])
    expect(h.entries).toEqual(['pod', 'rs', 'deploy'])
    expect(h.index).toBe(2)
  })

  it('steps back and forward', () => {
    const h = visitAll(['pod', 'rs', 'deploy'])
    const back = stepInspectHistory(h, -1)
    expect(back.key).toBe('rs')
    const forward = stepInspectHistory(back.history, 1)
    expect(forward.key).toBe('deploy')
    expect(stepInspectHistory(forward.history, 1)).toBeNull()
    expect(canStepInspectHistory(EMPTY_INSPECT_HISTORY, -1)).toBe(false)
  })

  it('drops forward entries when a new object is opened after going back', () => {
    const h = visitAll(['pod', 'rs', 'deploy'])
    const { history } = stepInspectHistory(h, -2)
    const next = visitInspectKey(history, 'node')
    expect(next.entries).toEqual(['pod', 'node'])
    expect(canStepInspectHistory(next, 1)).toBe(false)
  })

  it('keeps the newest 50 entries', () => {
    const h = visitAll(Array.from({ length: 60 }, (_, i) => `obj-${i}`))
    expect(h.entries).toHaveLength(50)
    expect(h.entries[0]).toBe('obj-10')
    expect(h.index).toBe(49)
  })
})
