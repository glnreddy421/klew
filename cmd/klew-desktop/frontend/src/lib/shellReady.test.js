import { describe, expect, it } from 'vitest'
import { waitForShellQuiet } from './shellReady.js'

/** A fake clock: sleep advances time instead of waiting. */
function clock(outputs = []) {
  let t = 0
  return {
    now: () => t,
    sleep: async (ms) => { t += ms },
    // Last output time the shell has produced by "now".
    lastOutputAt: () => outputs.filter((o) => o <= t).at(-1) ?? 0,
    elapsed: () => t,
  }
}

describe('waitForShellQuiet', () => {
  it('waits for output to stop for quietMs after the prompt', async () => {
    const c = clock([10, 200, 450]) // rc files still printing at 450ms
    const ok = await waitForShellQuiet(c.lastOutputAt, { quietMs: 300, now: c.now, sleep: c.sleep })
    expect(ok).toBe(true)
    expect(c.elapsed()).toBeGreaterThanOrEqual(750)
  })

  it('does not treat a silent shell as ready before any output', async () => {
    const c = clock([])
    const ok = await waitForShellQuiet(c.lastOutputAt, { quietMs: 300, maxMs: 1000, now: c.now, sleep: c.sleep })
    expect(ok).toBe(false)
    expect(c.elapsed()).toBeGreaterThanOrEqual(1000)
  })

  it('gives up after maxMs when output never stops', async () => {
    const c = clock(Array.from({ length: 200 }, (_, i) => i * 40))
    const ok = await waitForShellQuiet(c.lastOutputAt, { quietMs: 300, maxMs: 2000, now: c.now, sleep: c.sleep })
    expect(ok).toBe(false)
    expect(c.elapsed()).toBeLessThan(2100)
  })

  it('returns at once when the prompt is already old', async () => {
    const c = clock([0.5])
    c.sleep(400)
    const ok = await waitForShellQuiet(c.lastOutputAt, { quietMs: 300, now: c.now, sleep: c.sleep })
    expect(ok).toBe(true)
  })
})
