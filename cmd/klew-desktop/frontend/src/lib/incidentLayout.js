export const LAYOUT_STORAGE_KEY = 'klew.incident.layoutMode'
export const LIST_WIDTH_STORAGE_KEY = 'klew.incident.listWidth'

/** Default workspace layout — balanced table + signals sidebar. */
export const DEFAULT_WORKSPACE_LAYOUT = 'clean-professional'

export const WORKSPACE_LAYOUTS = [
  {
    id: 'command-center',
    letter: '1',
    label: 'Command Center',
    shortLabel: 'Center',
    hint: 'Analyst focused',
    description: 'Resource tree, entity table, and signals sidebar.',
    entityView: 'table',
    tableDensity: 'standard',
    inspectMode: 'stacked',
    defaultListWidth: 560,
    showFocusButton: false,
    showEmptyToggle: false,
    sortBySignal: false,
  },
  {
    id: 'clean-professional',
    letter: '2',
    label: 'Clean Professional',
    shortLabel: 'Clean',
    hint: 'Balanced',
    description: 'Spacious table layout with empty-resource controls.',
    entityView: 'table',
    tableDensity: 'standard',
    inspectMode: 'stacked',
    defaultListWidth: 580,
    showFocusButton: false,
    showEmptyToggle: true,
    sortBySignal: false,
  },
  {
    id: 'data-dense',
    letter: '3',
    label: 'Data Dense',
    shortLabel: 'Dense',
    hint: 'Power users',
    description: 'Extra columns for restarts, CPU, and memory.',
    entityView: 'table',
    tableDensity: 'dense',
    inspectMode: 'stacked',
    defaultListWidth: 660,
    showFocusButton: false,
    showEmptyToggle: false,
    sortBySignal: false,
  },
  {
    id: 'investigation-flow',
    letter: '4',
    label: 'Investigation Flow',
    shortLabel: 'Flow',
    hint: 'Contextual',
    description: 'Compact entity list with tabbed deep-dive panel.',
    entityView: 'list',
    tableDensity: 'standard',
    inspectMode: 'detail-tabs',
    defaultListWidth: 400,
    showFocusButton: true,
    showEmptyToggle: false,
    sortBySignal: true,
  },
]

/** @deprecated use WORKSPACE_LAYOUTS */
export const LAYOUT_MODES = WORKSPACE_LAYOUTS

const LEGACY_LAYOUT_MAP = {
  current: 'command-center',
  'master-detail': 'command-center',
  'signal-first': 'command-center',
  'detail-tabs': 'investigation-flow',
  'dense-list': 'data-dense',
  'unified-select': 'clean-professional',
}

const VALID = new Set(WORKSPACE_LAYOUTS.map((m) => m.id))

const DEFAULT_LIST_WIDTH = Object.fromEntries(
  WORKSPACE_LAYOUTS.map((m) => [m.id, m.defaultListWidth]),
)

export function layoutConfig(id) {
  return WORKSPACE_LAYOUTS.find((m) => m.id === id)
    || WORKSPACE_LAYOUTS.find((m) => m.id === DEFAULT_WORKSPACE_LAYOUT)
}

export function normalizeLayoutMode(id) {
  if (id && VALID.has(id)) return id
  if (id && LEGACY_LAYOUT_MAP[id]) return LEGACY_LAYOUT_MAP[id]
  return DEFAULT_WORKSPACE_LAYOUT
}

export function loadLayoutMode(prefs) {
  const fromPrefs = prefs?.workspaceLayout
  if (fromPrefs) {
    const normalized = normalizeLayoutMode(fromPrefs)
    if (VALID.has(normalized)) return normalized
  }
  try {
    const v = localStorage.getItem(LAYOUT_STORAGE_KEY)
    if (v) return normalizeLayoutMode(v)
  } catch {
    /* ignore */
  }
  return DEFAULT_WORKSPACE_LAYOUT
}

export function saveLayoutMode(id) {
  const next = normalizeLayoutMode(id)
  if (!VALID.has(next)) return
  try {
    localStorage.setItem(LAYOUT_STORAGE_KEY, next)
  } catch {
    /* ignore */
  }
  return next
}

export function defaultListWidth(layoutMode) {
  return DEFAULT_LIST_WIDTH[normalizeLayoutMode(layoutMode)] || 520
}

export function loadListWidth(layoutMode) {
  try {
    const raw = localStorage.getItem(LIST_WIDTH_STORAGE_KEY)
    if (raw) {
      const n = Number(raw)
      if (Number.isFinite(n) && n >= 200 && n <= 900) return n
    }
  } catch {
    /* ignore */
  }
  return defaultListWidth(layoutMode)
}

export function saveListWidth(px) {
  try {
    localStorage.setItem(LIST_WIDTH_STORAGE_KEY, String(Math.round(px)))
  } catch {
    /* ignore */
  }
}

/** List chrome flags derived from workspace layout. */
export function listChromeForMode(layoutMode) {
  const cfg = layoutConfig(layoutMode)
  return {
    showFocusButton: cfg.showFocusButton,
    dense: cfg.entityView === 'list',
    statusLed: cfg.entityView === 'list',
    allowHoverInspect: cfg.entityView === 'list',
    focusChevron: false,
    entityView: cfg.entityView,
    tableDensity: cfg.tableDensity,
    showEmptyToggle: cfg.showEmptyToggle,
  }
}

export function inspectPanelMode(layoutMode) {
  return layoutConfig(layoutMode).inspectMode
}

/** Whether the detail panel should expose a Focus chain CTA in its header. */
export function inspectShowsFocusCta(layoutMode) {
  return layoutConfig(layoutMode).id === 'investigation-flow'
}

/** Pull up to 3 compact stats from existing inspect model (no invented telemetry). */
export function deriveSignalStats(inspect) {
  if (!inspect) return []
  const stats = []

  const restart = (inspect.status?.fields || []).find((f) => /restart/i.test(f.k))
  if (restart && restart.v != null && restart.v !== '—' && restart.v !== '0') {
    stats.push({ label: 'Restarts', value: String(restart.v) })
  }

  for (const bar of inspect.resourceBars || []) {
    if (stats.length >= 3 || bar.empty) continue
    const label = bar.label || ''
    if (/mem/i.test(label) && bar.detail) {
      const lim = String(bar.detail).match(/limit\s+([^\s|/]+)/i)
        || String(bar.detail).match(/\/\s*([^\s]+)\s*$/)
      stats.push({
        label: lim ? 'Mem limit' : label,
        value: lim ? lim[1] : String(bar.detail).split(/\s+/).slice(-1)[0] || bar.detail,
      })
    } else if (/cpu/i.test(label) && bar.detail) {
      const lim = String(bar.detail).match(/limit\s+([^\s|/]+)/i)
      stats.push({
        label: lim ? 'CPU limit' : label,
        value: lim ? lim[1] : String(bar.detail).split(/\s+/).slice(-1)[0] || bar.detail,
      })
    }
  }

  const ready = (inspect.status?.fields || []).find((f) => /^ready$/i.test(f.k) || /ready/i.test(f.k))
  if (stats.length < 3 && ready?.v) {
    stats.push({ label: 'Ready', value: String(ready.v) })
  }

  const phase = (inspect.status?.fields || []).find((f) => /phase/i.test(f.k))
  if (stats.length < 3 && phase?.v) {
    stats.push({ label: 'Phase', value: String(phase.v) })
  }

  return stats.slice(0, 3)
}

export function hasAnomalyIssues(inspect) {
  return (inspect?.anomalies || []).some((a) => a.level === 'crit' || a.level === 'warn')
}
