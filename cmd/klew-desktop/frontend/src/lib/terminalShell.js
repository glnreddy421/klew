/** Preference value for terminal shell — empty/system means follow $SHELL. */
export function resolveTerminalShellPref(prefValue) {
  const v = String(prefValue || '').trim()
  if (!v || v === 'system') return ''
  return v
}
