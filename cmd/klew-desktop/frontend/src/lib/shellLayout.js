/**
 * Shell UI preference persistence (not investigation state).
 */

export const SHELL_LAYOUT_KEY = 'klew.shell.layout'

/**
 * Activity rail sizing. `auto` shows icons only while an explorer pane is beside
 * it (labels would duplicate the explorer) and icon + label elsewhere.
 */
export const RAIL_MODE = {
  AUTO: 'auto',
  EXPANDED: 'expanded',
  COLLAPSED: 'collapsed',
}

/** Must match --rail-w / --rail-w-collapsed in style.css. */
export const RAIL_WIDTH = { expanded: 156, collapsed: 52 }

/** Below this the resource table stops being usable; the explorer yields first. */
export const WORKSPACE_MIN_WIDTH = 480

/** Pane gutters on the inspector row (explorer handle, inspector handle, outer edge). */
export const SHELL_GUTTERS = 24

const DEFAULT = {
  explorerWidth: 260,
  inspectorWidth: 440,
  /** User dragged the explorer edge — label auto-fit stays off until a double-click refit. */
  explorerWidthPinned: false,
  /** User sized the inspector — it reopens at that size instead of the default. */
  inspectorWidthPinned: false,
  inspectorPlacement: 'right',
  explorerCollapsed: false,
  inspectorCollapsed: true,
  /** Activity rail sizing — see RAIL_MODE */
  railMode: RAIL_MODE.AUTO,
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
    const { railCollapsed: legacyRailCollapsed, ...rest } = parsed
    return {
      ...DEFAULT,
      ...rest,
      railMode: normalizeRailMode(parsed.railMode, legacyRailCollapsed),
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
  import('./settingsCache.js').then((m) => m.queueSettingsCacheSync()).catch(() => {})
}

export function explorerDefaultCollapsed(tab) {
  const layout = loadShellLayout()
  return layout.explorerCollapsedByTab?.[tab] ?? false
}

function normalizeRailMode(mode, legacyRailCollapsed) {
  if (Object.values(RAIL_MODE).includes(mode)) return mode
  // Pre-auto layouts stored a boolean; keep an explicit icon-only choice.
  return legacyRailCollapsed === true ? RAIL_MODE.COLLAPSED : RAIL_MODE.AUTO
}

/** Whether the rail renders icon-only for this mode and context. */
export function resolveRailCollapsed(railMode, hasExplorer) {
  if (railMode === RAIL_MODE.COLLAPSED) return true
  if (railMode === RAIL_MODE.EXPANDED) return false
  return !!hasExplorer
}

/**
 * Rail toggle: flip what the user sees. When the flipped state is what auto
 * would choose anyway, return to auto so the rail keeps adapting.
 */
export function nextRailMode(railMode, hasExplorer) {
  const collapse = !resolveRailCollapsed(railMode, hasExplorer)
  if (collapse === !!hasExplorer) return RAIL_MODE.AUTO
  return collapse ? RAIL_MODE.COLLAPSED : RAIL_MODE.EXPANDED
}

/** True when an open right-side inspector would squeeze the workspace below its minimum. */
export function shouldAutoCollapseExplorer({
  viewportWidth,
  railWidth,
  explorerWidth,
  inspectorWidth,
  gutters = SHELL_GUTTERS,
  minWorkspace = WORKSPACE_MIN_WIDTH,
}) {
  return viewportWidth - railWidth - explorerWidth - inspectorWidth - gutters < minWorkspace
}
