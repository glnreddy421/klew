/**
 * Desktop preferences — persisted in localStorage.
 * Investigation knobs are applied on the next StartInvestigation.
 */

export const PREFS_STORAGE_KEY = 'klew.desktop.preferences'

export const SETTINGS_SECTIONS = [
  { id: 'general', label: 'General', hint: 'App defaults' },
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

    // Investigation thresholds
    tailLines: 200,
    refreshSec: 10,
    windowMin: 15,
    autoRefresh: true,

    // Concurrency / streams
    maxLogRequests: 50,

    // Kubernetes
    kubeconfigPath: '', // empty = cluster default from backend
    allNamespaces: false,
    useMetricsServer: true,
    metricsApiGroup: 'metrics.k8s.io', // informational / future override
  }
}

export function loadPreferences() {
  const defaults = defaultPreferences()
  try {
    const raw = localStorage.getItem(PREFS_STORAGE_KEY)
    if (!raw) return defaults
    const parsed = JSON.parse(raw)
    return normalizePreferences({ ...defaults, ...parsed })
  } catch {
    return defaults
  }
}

export function savePreferences(prefs) {
  const next = normalizePreferences(prefs)
  try {
    localStorage.setItem(PREFS_STORAGE_KEY, JSON.stringify(next))
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

    tailLines: clampInt(src.tailLines, 0, 5000, d.tailLines),
    refreshSec: clampInt(src.refreshSec, 2, 300, d.refreshSec),
    windowMin: clampInt(src.windowMin, 1, 15, d.windowMin),
    autoRefresh: bool(src.autoRefresh, d.autoRefresh),

    maxLogRequests: clampInt(src.maxLogRequests, 1, 200, d.maxLogRequests),

    kubeconfigPath: String(src.kubeconfigPath ?? d.kubeconfigPath ?? '').trim(),
    allNamespaces: bool(src.allNamespaces, d.allNamespaces),
    useMetricsServer: bool(src.useMetricsServer, d.useMetricsServer),
    metricsApiGroup: String(src.metricsApiGroup || d.metricsApiGroup).trim() || d.metricsApiGroup,
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
