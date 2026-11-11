/** Inspector back / forward: the objects opened in the inspector, oldest first. */

const MAX_ENTRIES = 50

export const EMPTY_INSPECT_HISTORY = { entries: [], index: -1 }

/** Record that key is now shown. Opening something new drops the forward entries. */
export function visitInspectKey(history, key) {
  if (!key) return history
  const { entries, index } = history
  if (entries[index] === key) return history
  const next = [...entries.slice(0, index + 1), key].slice(-MAX_ENTRIES)
  return { entries: next, index: next.length - 1 }
}

/** Step delta entries (−1 back, +1 forward); null when there is nowhere to go. */
export function stepInspectHistory(history, delta) {
  const index = history.index + delta
  if (index < 0 || index >= history.entries.length) return null
  return { history: { ...history, index }, key: history.entries[index] }
}

export function canStepInspectHistory(history, delta) {
  const index = history.index + delta
  return index >= 0 && index < history.entries.length
}
