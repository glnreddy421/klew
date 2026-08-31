/**
 * Desktop preferences — persisted in localStorage.
 * Investigation knobs are applied on the next StartInvestigation, or live when a session is running (refresh interval / auto-refresh).
 */

import { DEFAULT_WORKSPACE_LAYOUT, normalizeLayoutMode, saveLayoutMode } from './incidentLayout'
import { DEFAULT_TERMINAL_APPEARANCE, normalizeTerminalAppearance } from './terminalAppearance'

export const PREFS_STORAGE_KEY = 'klew.desktop.preferences'

/** Bump when defaults change so existing localStorage picks up migrations once. */
export const PREFS_VERSION = 5

/** Investigation window lengths supported by the engine (minutes). */
export const WINDOW_MIN_OPTIONS = [5, 15, 30, 60]

export const SETTINGS_SECTIONS = [
  { id: 'general', label: 'General', hint: 'App defaults & session' },
  { id: 'appearance', label: 'Appearance', hint: 'Theme & live tail' },
  { id: 'investigation', label: 'Investigation', hint: 'Windows & refresh' },
  { id: 'concurrency', label: 'Concurrency', hint: 'Log stream limits' },
  { id: 'kubernetes', label: 'Kubernetes', hint: 'Kubeconfig & metrics' },
  { id: 'help', label: 'Help', hint: 'Shortcuts & links' },
]

/** @typedef {ReturnType<typeof defaultPreferences>} Preferences */

export function defaultPreferences() {
  return {
    // General
    openStreamOnInvestigate: true,
    followLogsByDefault: true,
    rememberLastQuery: true,

    // Appearance / terminal (live tail)
    streamFontSize: 12,
    streamDense: false,
    streamWrapLines: false,
    workspaceLayout: DEFAULT_WORKSPACE_LAYOUT,

    // Investigation thresholds
    tailLines: 200,
    refreshSec: 10,
    windowMin: 15,
    autoRefresh: true,

    // Idle safety — on by default; stops investigation after no user input.
    idleAutoStop: true,
    idleAutoStopMin: 120,

    // Concurrency / streams
    maxLogRequests: 50,

    // Kubernetes
    kubeconfigPath: '', // empty = cluster default from backend
    allNamespaces: false,
    useMetricsServer: true,
    metricsApiGroup: 'metrics.k8s.io', // informational / future override

    // In-app terminal
    terminalShell: '', // empty/system = follow $SHELL
    terminalShellPrompted: false,
    terminalAppearance: DEFAULT_TERMINAL_APPEARANCE,

    prefsVersion: PREFS_VERSION,
  }
}

function migratePreferences(parsed) {
  const src = parsed && typeof parsed === 'object' ? parsed : {}
  const version = Number(src.prefsVersion) || 1
  if (version >= PREFS_VERSION) {
    return src
  }
  const next = { ...src, prefsVersion: PREFS_VERSION }
  // v2: idle auto-stop on by default (was off for early builds).
  if (version < 2) {
    next.idleAutoStop = true
    if (next.idleAutoStopMin == null) {
      next.idleAutoStopMin = 120
    }
  }
  // v3: workspace layout preference (migrate legacy layout localStorage key).
  if (version < 3) {
    try {
      const legacy = localStorage.getItem('klew.incident.layoutMode')
      next.workspaceLayout = normalizeLayoutMode(legacy || DEFAULT_WORKSPACE_LAYOUT)
      saveLayoutMode(next.workspaceLayout)
    } catch {
      next.workspaceLayout = DEFAULT_WORKSPACE_LAYOUT
    }
  }
  // v4: in-app terminal shell preference.
  if (version < 4) {
    next.terminalShell = ''
    next.terminalShellPrompted = false
  }
  // v5: terminal appearance preset.
  if (version < 5) {
    next.terminalAppearance = DEFAULT_TERMINAL_APPEARANCE
  }
  return next
}

export function loadPreferences() {
  const defaults = defaultPreferences()
  try {
    const raw = localStorage.getItem(PREFS_STORAGE_KEY)
    if (!raw) return defaults
    const before = JSON.parse(raw)
    const migrated = migratePreferences(before)
    const normalized = normalizePreferences({ ...defaults, ...migrated })
    if ((Number(before.prefsVersion) || 1) < PREFS_VERSION) {
      savePreferences(normalized)
    }
    return normalized
  } catch {
    return defaults
  }
}

export function savePreferences(prefs) {
  const next = normalizePreferences(prefs)
  try {
    localStorage.setItem(PREFS_STORAGE_KEY, JSON.stringify(next))
    saveLayoutMode(next.workspaceLayout)
  } catch {
    /* ignore quota */
  }
  return next
}

export function normalizePreferences(p) {
  const d = defaultPreferences()
  const src = p && typeof p === 'object' ? p : {}
  return {
    openStreamOnInvestigate: bool(src.openStreamOnInvestigate, d.openStreamOnInvestigate),
    followLogsByDefault: bool(src.followLogsByDefault, d.followLogsByDefault),
    rememberLastQuery: bool(src.rememberLastQuery, d.rememberLastQuery),

    streamFontSize: clampInt(src.streamFontSize, 10, 18, d.streamFontSize),
    streamDense: bool(src.streamDense, d.streamDense),
    streamWrapLines: bool(src.streamWrapLines, d.streamWrapLines),
    workspaceLayout: normalizeLayoutMode(src.workspaceLayout || d.workspaceLayout),

    tailLines: clampInt(src.tailLines, 0, 5000, d.tailLines),
    refreshSec: clampInt(src.refreshSec, 2, 300, d.refreshSec),
    windowMin: clampInt(src.windowMin, 1, 15, d.windowMin),
    autoRefresh: bool(src.autoRefresh, d.autoRefresh),

    idleAutoStop: bool(src.idleAutoStop, d.idleAutoStop),
    idleAutoStopMin: clampInt(src.idleAutoStopMin, 30, 480, d.idleAutoStopMin),

    maxLogRequests: clampInt(src.maxLogRequests, 1, 200, d.maxLogRequests),

    kubeconfigPath: String(src.kubeconfigPath ?? d.kubeconfigPath ?? '').trim(),
    allNamespaces: bool(src.allNamespaces, d.allNamespaces),
    useMetricsServer: bool(src.useMetricsServer, d.useMetricsServer),
    metricsApiGroup: String(src.metricsApiGroup || d.metricsApiGroup).trim() || d.metricsApiGroup,

    terminalShell: String(src.terminalShell ?? d.terminalShell ?? '').trim(),
    terminalShellPrompted: bool(src.terminalShellPrompted, d.terminalShellPrompted),
    terminalAppearance: normalizeTerminalAppearance(src.terminalAppearance ?? d.terminalAppearance),

    prefsVersion: clampInt(src.prefsVersion, 1, PREFS_VERSION, d.prefsVersion),
  }
}

/** Options passed to StartInvestigation from preferences. */
export function startOptionsFromPreferences(prefs, base = {}) {
  const p = normalizePreferences(prefs)
  return {
    query: base.query || '',
    namespace: base.namespace || '',
    context: base.context || '',
    kubeconfig: p.kubeconfigPath || base.kubeconfig || '',
    allNamespaces: Boolean(p.allNamespaces),
    tail: p.tailLines,
    refreshSec: p.refreshSec,
    windowSec: p.windowMin * 60,
    maxLogRequests: p.maxLogRequests,
    autoRefresh: p.autoRefresh,
    useMetricsServer: p.useMetricsServer,
  }
}

function bool(v, fallback) {
  return typeof v === 'boolean' ? v : fallback
}

function clampInt(v, min, max, fallback) {
  const n = Number(v)
  if (!Number.isFinite(n)) return fallback
  return Math.min(max, Math.max(min, Math.round(n)))
}
