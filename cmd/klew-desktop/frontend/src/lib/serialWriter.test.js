import { describe, expect, it } from 'vitest'
import { createSerialWriter } from './serialWriter.js'

/** A write whose calls finish in reverse order, like concurrent Wails requests can. */
function reorderingWrite() {
  const received = []
  let inFlight = 0
  const write = (data) => {
    inFlight += 1
    const delay = 30 - Math.min(inFlight, 25)
    return new Promise((resolve) => setTimeout(() => {
      received.push(data)
      inFlight -= 1
      resolve()
    }, delay))
  }
  return { write, received }
}

const settle = () => new Promise((r) => setTimeout(r, 200))

describe('createSerialWriter', () => {
  it('keeps key presses in order even when writes finish out of order', async () => {
    const { write, received } = reorderingWrite()
    const w = createSerialWriter(write)
    for (const ch of 'clear; echo $(stty size)') w.push(ch)
    await settle()
    expect(received.join('')).toBe('clear; echo $(stty size)')
  })

  it('sends at most one write at a time and batches what arrives meanwhile', async () => {
    let active = 0
    let maxActive = 0
    const calls = []
    const w = createSerialWriter(async (data) => {
      active += 1
      maxActive = Math.max(maxActive, active)
      calls.push(data)
      await new Promise((r) => setTimeout(r, 10))
      active -= 1
    })
    w.push('a')
    w.push('b')
    w.push('c')
    await settle()
    expect(maxActive).toBe(1)
    expect(calls).toEqual(['a', 'bc'])
  })

  it('keeps going after a failed write', async () => {
    const calls = []
    let first = true
    const w = createSerialWriter(async (data) => {
      calls.push(data)
      if (first) {
        first = false
        throw new Error('gone')
      }
    })
    w.push('x')
    await settle()
    w.push('y')
    await settle()
    expect(calls).toEqual(['x', 'y'])
  })

  it('clear drops input not yet sent', async () => {
    const calls = []
    const w = createSerialWriter(async (data) => {
      calls.push(data)
      await new Promise((r) => setTimeout(r, 10))
    })
    w.push('a')
    w.push('b')
    w.clear()
    await settle()
    expect(calls).toEqual(['a'])
  })
})
