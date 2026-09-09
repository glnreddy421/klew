/**
 * Shell UI preference persistence (not investigation state).
 */

export const SHELL_LAYOUT_KEY = 'klew.shell.layout'

const DEFAULT = {
  explorerWidth: 230,
  inspectorWidth: 400,
  inspectorPlacement: 'right',
  explorerCollapsed: false,
  inspectorCollapsed: true,
  /** Activity rail — false shows icon + label, true shows icons only */
  railCollapsed: false,
  /** Per-activity default collapse — Overview hides the explorer entirely */
  explorerCollapsedByTab: {
    incident: true,
    graph: false,
    evidence: false,
    patterns: false,
    failures: false,
    resources: false,
    terminal: true,
  },
}

export function loadShellLayout() {
  try {
    const raw = localStorage.getItem(SHELL_LAYOUT_KEY)
    if (!raw) return { ...DEFAULT }
    const parsed = JSON.parse(raw)
    return {
      ...DEFAULT,
      ...parsed,
      // Inspector open/close is per-session — always start collapsed.
      inspectorCollapsed: DEFAULT.inspectorCollapsed,
      explorerCollapsedByTab: {
        ...DEFAULT.explorerCollapsedByTab,
        ...(parsed.explorerCollapsedByTab || {}),
      },
    }
  } catch {
    return { ...DEFAULT }
  }
}

export function saveShellLayout(patch) {
  try {
    const prev = loadShellLayout()
    const { inspectorCollapsed: _collapsed, ...persistedPatch } = patch
    localStorage.setItem(SHELL_LAYOUT_KEY, JSON.stringify({ ...prev, ...persistedPatch }))
  } catch {
    // ignore
  }
}

export function explorerDefaultCollapsed(tab) {
  const layout = loadShellLayout()
  return layout.explorerCollapsedByTab?.[tab] ?? false
}
