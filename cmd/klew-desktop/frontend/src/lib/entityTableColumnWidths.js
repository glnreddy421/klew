/**
 * Manual column widths for the entity table, persisted per kind.
 *
 * Until the user resizes a column the table uses automatic layout. The first
 * resize snapshots every column's rendered width and switches the table to fixed
 * layout, so dragging one edge moves only that column.
 */

// Distinct from ENTITY_TABLE_COLUMNS_KEY: settings sync treats every key under
// that prefix as a kind's visible-column list.
export const ENTITY_TABLE_WIDTHS_KEY = 'klew.entityTable.widths'

export const COLUMN_MIN_WIDTH = 48
export const COLUMN_MAX_WIDTH = 800
export const COLUMN_FALLBACK_WIDTH = 120
export const COLUMN_KEYBOARD_STEP = 16

export function clampColumnWidth(width) {
  const n = Math.round(Number(width) || 0)
  return Math.min(COLUMN_MAX_WIDTH, Math.max(COLUMN_MIN_WIDTH, n))
}

export function hasColumnWidths(widths) {
  return !!widths && Object.keys(widths).length > 0
}

/** Fill widths for `ids` from saved values, then measured values, then a fallback. */
export function completeColumnWidths(saved = {}, ids = [], measured = {}) {
  const out = {}
  for (const id of ids) {
    const w = saved[id] ?? measured[id] ?? COLUMN_FALLBACK_WIDTH
    out[id] = clampColumnWidth(w)
  }
  return out
}

export function totalColumnWidth(widths = {}, ids = []) {
  return ids.reduce((sum, id) => sum + (widths[id] ?? COLUMN_FALLBACK_WIDTH), 0)
}

export function loadColumnWidths(kind) {
  if (!kind) return {}
  try {
    const raw = localStorage.getItem(`${ENTITY_TABLE_WIDTHS_KEY}.${kind}`)
    const parsed = raw ? JSON.parse(raw) : null
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {}
    const out = {}
    for (const [id, w] of Object.entries(parsed)) {
      if (Number.isFinite(w)) out[id] = clampColumnWidth(w)
    }
    return out
  } catch {
    return {}
  }
}

export function saveColumnWidths(kind, widths) {
  if (!kind) return
  try {
    const key = `${ENTITY_TABLE_WIDTHS_KEY}.${kind}`
    if (hasColumnWidths(widths)) localStorage.setItem(key, JSON.stringify(widths))
    else localStorage.removeItem(key)
  } catch {
    // ignore quota / unavailable storage
  }
}

/** Rendered header widths keyed by `data-col-id`. */
export function measureHeaderWidths(table) {
  const out = {}
  for (const th of table?.querySelectorAll('thead th[data-col-id]') || []) {
    out[th.dataset.colId] = Math.ceil(th.getBoundingClientRect().width)
  }
  return out
}

/**
 * Content-fit widths: briefly lay the table out automatically at max-content,
 * read each header cell, then restore. Runs synchronously, so nothing paints.
 */
export function measureNaturalColumnWidths(table) {
  if (!table) return {}
  const cols = [...table.querySelectorAll('colgroup col')]
  const savedCols = cols.map((c) => c.style.width)
  const { tableLayout, width, minWidth } = table.style
  cols.forEach((c) => { c.style.width = '' })
  table.classList.add('is-measuring')
  table.style.tableLayout = 'auto'
  table.style.width = 'max-content'
  table.style.minWidth = '0'
  try {
    return measureHeaderWidths(table)
  } finally {
    cols.forEach((c, i) => { c.style.width = savedCols[i] })
    table.classList.remove('is-measuring')
    table.style.tableLayout = tableLayout
    table.style.width = width
    table.style.minWidth = minWidth
  }
}
