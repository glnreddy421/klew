export const LAYOUT_STORAGE_KEY = 'klew.incident.layoutMode'
export const LIST_WIDTH_STORAGE_KEY = 'klew.incident.listWidth'

export const LAYOUT_MODES = [
  { id: 'current', letter: 'A', label: 'Current' },
  { id: 'master-detail', letter: 'B', label: 'Master–detail' },
  { id: 'signal-first', letter: 'C', label: 'Signal-first' },
  { id: 'detail-tabs', letter: 'D', label: 'Detail tabs' },
  { id: 'dense-list', letter: 'E', label: 'Dense list' },
  { id: 'unified-select', letter: 'F', label: 'Unified select' },
]

const VALID = new Set(LAYOUT_MODES.map((m) => m.id))

const DEFAULT_LIST_WIDTH = {
  current: 340,
  'master-detail': 250,
  'signal-first': 250,
  'detail-tabs': 250,
  'dense-list': 260,
  'unified-select': 300,
}

export function loadLayoutMode() {
  try {
    const v = localStorage.getItem(LAYOUT_STORAGE_KEY)
    if (v && VALID.has(v)) return v
  } catch {
    /* ignore */
  }
  return 'detail-tabs'
}

export function saveLayoutMode(id) {
  if (!VALID.has(id)) return
  try {
    localStorage.setItem(LAYOUT_STORAGE_KEY, id)
  } catch {
    /* ignore */
  }
}

export function defaultListWidth(layoutMode) {
  return DEFAULT_LIST_WIDTH[layoutMode] || 300
}

export function loadListWidth(layoutMode) {
  try {
    const raw = localStorage.getItem(LIST_WIDTH_STORAGE_KEY)
    if (raw) {
      const n = Number(raw)
      if (Number.isFinite(n) && n >= 200 && n <= 720) return n
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

/** List chrome flags derived from layout mode (match list, not focus-chain). */
export function listChromeForMode(layoutMode) {
  switch (layoutMode) {
    case 'master-detail':
    case 'signal-first':
    case 'detail-tabs':
    case 'unified-select':
      return {
        showFocusButton: false,
        dense: layoutMode !== 'unified-select',
        statusLed: layoutMode === 'signal-first',
        allowHoverInspect: false,
        focusChevron: false,
      }
    case 'dense-list':
      return {
        showFocusButton: false,
        dense: true,
        statusLed: true,
        allowHoverInspect: false,
        focusChevron: true,
      }
    case 'current':
    default:
      return {
        showFocusButton: true,
        dense: false,
        statusLed: false,
        allowHoverInspect: true,
        focusChevron: false,
      }
  }
}

/** Whether the detail panel should expose a Focus chain CTA in its header. */
export function inspectShowsFocusCta(layoutMode) {
  // A/E: list or legacy chrome owns Focus. F: inspecting strip owns the only CTA.
  return (
    layoutMode !== 'current'
    && layoutMode !== 'dense-list'
    && layoutMode !== 'unified-select'
  )
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
