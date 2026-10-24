/**
 * One-shot request to open a specific inspector tab for an object, e.g. the
 * Overview pin → Deployment "Changes". Matched on kind + name because inspect
 * keys come in short (Kind/name) and long (Kind/ns/name) forms.
 */
let pending = null

export function requestInspectorTab(kind, name, tab) {
  pending = kind && name && tab ? { kind, name, tab } : null
}

/** Returns the requested tab for this object once, then clears it. */
export function consumeInspectorTab(kind, name) {
  if (!pending || pending.kind !== kind || pending.name !== name) return null
  const { tab } = pending
  pending = null
  return tab
}
