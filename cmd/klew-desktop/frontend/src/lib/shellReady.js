/**
 * Resolves once a new shell has gone quiet: it has printed something (its
 * prompt) and then nothing for `quietMs`. Typing into a shell that is still
 * loading its rc files makes zsh's line editor draw the line garbled.
 *
 * Returns true when the shell went quiet, false when `maxMs` passed first;
 * callers write either way.
 */
export async function waitForShellQuiet(lastOutputAt, {
  quietMs = 300,
  maxMs = 5000,
  now = Date.now,
  sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms)),
} = {}) {
  const start = now()
  for (;;) {
    const t = now()
    const last = lastOutputAt()
    if (last > 0 && t - last >= quietMs) return true
    if (t - start >= maxMs) return false
    await sleep(Math.min(50, quietMs))
  }
}
