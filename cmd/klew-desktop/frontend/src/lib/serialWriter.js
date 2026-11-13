/**
 * Sends terminal input one write at a time, in order.
 *
 * Each WriteTerminal call is a separate Wails request and the Go side does not
 * run them in the order they were sent, so firing one per key press lets fast
 * typing or a paste reach the shell scrambled. Input that arrives while a write
 * is in flight is buffered and sent with the next write.
 *
 * write(data) must return a promise; failures are dropped like before.
 */
export function createSerialWriter(write) {
  let buf = ''
  let busy = false

  async function pump() {
    if (busy) return
    busy = true
    try {
      while (buf) {
        const data = buf
        buf = ''
        try {
          await write(data)
        } catch {
          /* dropped, as an unbuffered write would be */
        }
      }
    } finally {
      busy = false
    }
  }

  return {
    push(data) {
      if (!data) return
      buf += data
      pump()
    },
    /** Drops input that has not been sent yet (session closed or restarted). */
    clear() {
      buf = ''
    },
  }
}
