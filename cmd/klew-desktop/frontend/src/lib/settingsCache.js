/**
 * Unified local settings cache — IndexedDB mirror of all user preferences.
 * Syncs on every change; refreshes from cache every hour (multi-window safe).
 */

import { idbGet, idbPut } from './localDb.js'
import { THEME_STORAGE_KEY } from './themes.js'
import { ENTITY_TABLE_COLUMNS_KEY } from './entityTableColumns.js'
import { LAYOUT_STORAGE_KEY, LIST_WIDTH_STORAGE_KEY } from './incidentLayout.js'
import { SHELL_LAYOUT_KEY } from './shellLayout.js'

const PREFS_STORAGE_KEY = 'klew.desktop.preferences'

export const SETTINGS_CACHE_TTL_MS = 60 * 60 * 1000
export const SETTINGS_CACHE_VERSION = 1

const DB_NAME = 'klew-settings-cache'
const DB_VERSION = 1
const STORE = 'snapshot'

const SIDEBAR_COLLAPSED_KEY = 'klew-desktop-sidebar-collapsed'
const WORKLOAD_METRICS_COLLAPSED_KEY = 'klew.workloadMetrics.collapsed'

/** @typedef {{
 *   version: number,
 *   updatedAt: number,
 *   prefs: object | null,
 *   themeId: string | null,
 *   layoutMode: string | null,
 *   listWidth: string | null,
 *   shellLayout: object | null,
 *   sidebarCollapsed: boolean,
 *   workloadMetricsCollapsed: boolean,
 *   entityTableColumns: Record<string, string[]>,
 * }} SettingsSnapshot */

/** @type {Set<(snapshot: SettingsSnapshot) => void>} */
const refreshHandlers = new Set()

function readJson(key) {
  try {
    const raw = localStorage.getItem(key)
    if (!raw) return null
    return JSON.parse(raw)
  } catch {
    return null
  }
}

function collectEntityTableColumns() {
  /** @type {Record<string, string[]>} */
  const out = {}
  try {
    const prefix = `${ENTITY_TABLE_COLUMNS_KEY}.`
    for (let i = 0; i < localStorage.length; i += 1) {
      const key = localStorage.key(i)
      if (!key?.startsWith(prefix)) continue
      const kind = key.slice(prefix.length)
      const parsed = readJson(key)
      if (Array.isArray(parsed) && parsed.length) {
        out[kind] = parsed.filter(Boolean)
      }
    }
  } catch {
    // ignore
  }
  return out
}

/** Gather all persisted settings from localStorage. */
export function collectSettingsSnapshot() {
  /** @type {SettingsSnapshot} */
  const snapshot = {
    version: SETTINGS_CACHE_VERSION,
    updatedAt: Date.now(),
    prefs: readJson(PREFS_STORAGE_KEY),
    themeId: localStorage.getItem(THEME_STORAGE_KEY),
    layoutMode: localStorage.getItem(LAYOUT_STORAGE_KEY),
    listWidth: localStorage.getItem(LIST_WIDTH_STORAGE_KEY),
    shellLayout: readJson(SHELL_LAYOUT_KEY),
    sidebarCollapsed: localStorage.getItem(SIDEBAR_COLLAPSED_KEY) === '1',
    workloadMetricsCollapsed: localStorage.getItem(WORKLOAD_METRICS_COLLAPSED_KEY) === '1',
    entityTableColumns: collectEntityTableColumns(),
  }
  return snapshot
}

/** Write snapshot fields back to localStorage. */
export function applySettingsSnapshot(snapshot) {
  if (!snapshot) return

  if (snapshot.prefs) {
    localStorage.setItem(PREFS_STORAGE_KEY, JSON.stringify(snapshot.prefs))
  }
  if (snapshot.themeId) {
    localStorage.setItem(THEME_STORAGE_KEY, snapshot.themeId)
  }
  if (snapshot.layoutMode) {
    localStorage.setItem(LAYOUT_STORAGE_KEY, snapshot.layoutMode)
  }
  if (snapshot.listWidth != null) {
    localStorage.setItem(LIST_WIDTH_STORAGE_KEY, String(snapshot.listWidth))
  }
  if (snapshot.shellLayout) {
    localStorage.setItem(SHELL_LAYOUT_KEY, JSON.stringify(snapshot.shellLayout))
  }
  localStorage.setItem(SIDEBAR_COLLAPSED_KEY, snapshot.sidebarCollapsed ? '1' : '0')
  localStorage.setItem(WORKLOAD_METRICS_COLLAPSED_KEY, snapshot.workloadMetricsCollapsed ? '1' : '0')

  const prefix = `${ENTITY_TABLE_COLUMNS_KEY}.`
  const nextKinds = new Set(Object.keys(snapshot.entityTableColumns || {}))
  for (let i = localStorage.length - 1; i >= 0; i -= 1) {
    const key = localStorage.key(i)
    if (key?.startsWith(prefix)) {
      const kind = key.slice(prefix.length)
      if (!nextKinds.has(kind)) {
        localStorage.removeItem(key)
      }
    }
  }
  for (const [kind, cols] of Object.entries(snapshot.entityTableColumns || {})) {
    if (cols?.length) {
      localStorage.setItem(`${ENTITY_TABLE_COLUMNS_KEY}.${kind}`, JSON.stringify(cols))
    }
  }
}

export function isSettingsCacheStale(updatedAt) {
  if (!updatedAt) return true
  return Date.now() - updatedAt >= SETTINGS_CACHE_TTL_MS
}

function snapshotSignature(snapshot) {
  if (!snapshot) return ''
  return JSON.stringify({
    prefs: snapshot.prefs,
    themeId: snapshot.themeId,
    layoutMode: snapshot.layoutMode,
    listWidth: snapshot.listWidth,
    shellLayout: snapshot.shellLayout,
    sidebarCollapsed: snapshot.sidebarCollapsed,
    workloadMetricsCollapsed: snapshot.workloadMetricsCollapsed,
    entityTableColumns: snapshot.entityTableColumns,
  })
}

/** @returns {Promise<SettingsSnapshot | null>} */
async function readSettingsCache() {
  try {
    const row = await idbGet(DB_NAME, DB_VERSION, STORE, 'active')
    if (!row?.snapshot) return null
    return row.snapshot
  } catch {
    return readSettingsCacheFallback()
  }
}

/** @returns {Promise<void>} */
async function writeSettingsCache(snapshot) {
  const row = { id: 'active', snapshot }
  try {
    await idbPut(DB_NAME, DB_VERSION, STORE, row)
    writeSettingsCacheFallback(snapshot)
  } catch {
    writeSettingsCacheFallback(snapshot)
  }
}

function readSettingsCacheFallback() {
  try {
    const raw = localStorage.getItem('klew.settings-cache.fallback')
    if (!raw) return null
    const parsed = JSON.parse(raw)
    return parsed?.snapshot || null
  } catch {
    return null
  }
}

function writeSettingsCacheFallback(snapshot) {
  try {
    localStorage.setItem('klew.settings-cache.fallback', JSON.stringify({
      snapshot,
      updatedAt: snapshot.updatedAt,
    }))
  } catch {
    // quota
  }
}

function notifyRefreshHandlers(snapshot) {
  for (const handler of refreshHandlers) {
    try {
      handler(snapshot)
    } catch {
      // ignore listener errors
    }
  }
}

/** Persist current localStorage settings to IndexedDB. */
export async function syncSettingsCache() {
  const snapshot = collectSettingsSnapshot()
  await writeSettingsCache(snapshot)
  return snapshot
}

/** Boot: restore from cache when local prefs missing; otherwise sync local → cache. */
export async function initSettingsCache() {
  const local = collectSettingsSnapshot()
  const cached = await readSettingsCache()
  const hasLocalPrefs = Boolean(localStorage.getItem(PREFS_STORAGE_KEY))

  if (!hasLocalPrefs && cached?.prefs) {
    applySettingsSnapshot(cached)
    return cached
  }

  const snapshot = collectSettingsSnapshot()
  await writeSettingsCache(snapshot)
  return snapshot
}

/**
 * Hourly refresh — pull newer cache from other windows, checkpoint local, re-apply runtime settings.
 * @returns {Promise<SettingsSnapshot>}
 */
export async function refreshSettingsFromCache() {
  const cached = await readSettingsCache()
  const local = collectSettingsSnapshot()

  if (
    cached
    && (cached.updatedAt || 0) > (local.updatedAt || 0)
    && snapshotSignature(cached) !== snapshotSignature(local)
  ) {
    applySettingsSnapshot(cached)
  }

  const effective = collectSettingsSnapshot()
  effective.updatedAt = Date.now()
  await writeSettingsCache(effective)
  notifyRefreshHandlers(effective)
  return effective
}

export function registerSettingsRefreshHandler(handler) {
  refreshHandlers.add(handler)
  return () => refreshHandlers.delete(handler)
}

export function scheduleSettingsCacheRefresh() {
  const tick = () => {
    refreshSettingsFromCache().catch(() => {})
  }
  const timer = setInterval(tick, SETTINGS_CACHE_TTL_MS)

  const onStorage = (event) => {
    if (!event.key) return
    if (
      event.key === PREFS_STORAGE_KEY
      || event.key === THEME_STORAGE_KEY
      || event.key?.startsWith(`${ENTITY_TABLE_COLUMNS_KEY}.`)
      || event.key === SHELL_LAYOUT_KEY
      || event.key === LAYOUT_STORAGE_KEY
      || event.key === LIST_WIDTH_STORAGE_KEY
    ) {
      refreshSettingsFromCache().catch(() => {})
    }
  }
  window.addEventListener('storage', onStorage)

  return () => {
    clearInterval(timer)
    window.removeEventListener('storage', onStorage)
  }
}

/** Fire-and-forget sync after a single setting changes. */
export function queueSettingsCacheSync() {
  syncSettingsCache().catch(() => {})
}
