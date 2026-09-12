import {
  downloadFontCss,
  FONT_CACHE_TTL_MS,
  isFontCacheStale,
  readFontCache,
  writeFontCache,
} from './fontCache.js'

export const DEFAULT_UI_FONT = 'jakarta'

/** @typedef {{ id: string, name: string, description: string, sans: string, mono: string, googleFonts?: string | null, sample?: string }} UiFontPreset */

/** @type {UiFontPreset[]} */
export const UI_FONT_PRESETS = [
  {
    id: 'jakarta',
    name: 'Jakarta',
    description: 'Modern & friendly — default',
    sans: "'Plus Jakarta Sans', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
    mono: "'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, monospace",
    googleFonts:
      'https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;500;600&family=Plus+Jakarta+Sans:wght@400;500;600;700&display=swap',
    sample: 'Overview 142m',
  },
  {
    id: 'inter',
    name: 'Inter',
    description: 'Neutral, widely used UI sans',
    sans: "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
    mono: "'IBM Plex Mono', ui-monospace, Menlo, Monaco, Consolas, monospace",
    googleFonts:
      'https://fonts.googleapis.com/css2?family=IBM+Plex+Mono:wght@400;500;600&family=Inter:wght@400;500;600;700&display=swap',
    sample: 'Overview 142m',
  },
  {
    id: 'system',
    name: 'System',
    description: 'Native OS fonts — no web download',
    sans: "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif",
    mono: "ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Monaco, Consolas, monospace",
    googleFonts: null,
    sample: 'Overview 142m',
  },
  {
    id: 'ibm-plex',
    name: 'IBM Plex',
    description: 'Technical & precise',
    sans: "'IBM Plex Sans', -apple-system, BlinkMacSystemFont, sans-serif",
    mono: "'IBM Plex Mono', ui-monospace, Menlo, Monaco, Consolas, monospace",
    googleFonts:
      'https://fonts.googleapis.com/css2?family=IBM+Plex+Mono:wght@400;500;600&family=IBM+Plex+Sans:wght@400;500;600;700&display=swap',
    sample: 'Overview 142m',
  },
  {
    id: 'source',
    name: 'Source',
    description: 'Adobe open source — highly readable',
    sans: "'Source Sans 3', -apple-system, BlinkMacSystemFont, sans-serif",
    mono: "'Source Code Pro', ui-monospace, Menlo, Monaco, Consolas, monospace",
    googleFonts:
      'https://fonts.googleapis.com/css2?family=Source+Code+Pro:wght@400;500;600&family=Source+Sans+3:wght@400;500;600;700&display=swap',
    sample: 'Overview 142m',
  },
]

const refreshInFlight = new Map()

export function getUiFont(id) {
  return UI_FONT_PRESETS.find((f) => f.id === id) || UI_FONT_PRESETS[0]
}

export function normalizeUiFont(id) {
  const raw = String(id || '').trim()
  return UI_FONT_PRESETS.some((f) => f.id === raw) ? raw : DEFAULT_UI_FONT
}

function ensureFontStylesheet(url) {
  if (!url) return
  if (document.querySelector(`link[data-klew-ui-font-link="${url}"]`)) return
  const link = document.createElement('link')
  link.rel = 'stylesheet'
  link.href = url
  link.dataset.klewUiFontLink = url
  document.head.appendChild(link)
}

function clearInjectedFontStyles(activeId) {
  document.querySelectorAll('style[data-klew-ui-font]').forEach((node) => {
    if (node.dataset.klewUiFont !== activeId) node.remove()
  })
  document.querySelectorAll('link[data-klew-ui-font-link]').forEach((node) => node.remove())
}

function injectFontStyle(presetId, css) {
  clearInjectedFontStyles(presetId)
  let el = document.getElementById(`klew-font-style-${presetId}`)
  if (!el) {
    el = document.createElement('style')
    el.id = `klew-font-style-${presetId}`
    document.head.appendChild(el)
  }
  el.dataset.klewUiFont = presetId
  el.textContent = css
}

/** Apply CSS variables immediately (sync). */
export function applyUiFontShell(id) {
  const preset = getUiFont(normalizeUiFont(id))
  const root = document.documentElement
  root.setAttribute('data-ui-font', preset.id)
  root.style.setProperty('--sans', preset.sans)
  root.style.setProperty('--mono', preset.mono)
  return preset.id
}

async function fetchAndCachePreset(preset) {
  if (!preset.googleFonts) return null
  const css = await downloadFontCss(preset.googleFonts)
  await writeFontCache(preset.id, css)
  return css
}

/** Refresh one preset from network and update cache. */
export async function refreshFontCache(presetOrId) {
  const preset = typeof presetOrId === 'string' ? getUiFont(presetOrId) : presetOrId
  if (!preset?.googleFonts) return null

  if (refreshInFlight.has(preset.id)) {
    return refreshInFlight.get(preset.id)
  }

  const job = fetchAndCachePreset(preset)
    .then((css) => {
      if (css && document.documentElement.getAttribute('data-ui-font') === preset.id) {
        injectFontStyle(preset.id, css)
      }
      return css
    })
    .finally(() => {
      refreshInFlight.delete(preset.id)
    })

  refreshInFlight.set(preset.id, job)
  return job
}

/**
 * Load font from local cache; refresh from network when stale (>1 hour).
 * Preference id is persisted in localStorage via preferences.js.
 */
export async function applyUiFont(id, { forceRefresh = false } = {}) {
  const preset = getUiFont(normalizeUiFont(id))
  applyUiFontShell(preset.id)

  if (!preset.googleFonts) {
    clearInjectedFontStyles('')
    return preset.id
  }

  const cached = forceRefresh ? null : await readFontCache(preset.id)

  if (cached?.css) {
    injectFontStyle(preset.id, cached.css)
    if (isFontCacheStale(cached.updatedAt)) {
      refreshFontCache(preset).catch(() => {})
    }
    return preset.id
  }

  try {
    const css = await fetchAndCachePreset(preset)
    if (css) injectFontStyle(preset.id, css)
  } catch {
    ensureFontStylesheet(preset.googleFonts)
  }

  return preset.id
}

/** Preload preview fonts in settings picker (network links only). */
export function preloadUiFontPreviews() {
  for (const preset of UI_FONT_PRESETS) {
    ensureFontStylesheet(preset.googleFonts)
  }
}

/** @deprecated use preloadUiFontPreviews */
export function preloadUiFonts() {
  preloadUiFontPreviews()
}

export async function initUiFont() {
  let id = DEFAULT_UI_FONT
  try {
    const raw = localStorage.getItem('klew.desktop.preferences')
    if (raw) {
      const parsed = JSON.parse(raw)
      id = normalizeUiFont(parsed.uiFont)
    }
  } catch {
    // ignore
  }
  applyUiFontShell(id)
  await applyUiFont(id)
  return id
}

/** Re-fetch active font every hour while the app is open. */
export function scheduleFontCacheRefresh(getFontId) {
  const tick = () => {
    const id = normalizeUiFont(getFontId?.())
    const preset = getUiFont(id)
    if (preset.googleFonts) {
      refreshFontCache(preset).catch(() => {})
    }
  }
  const timer = setInterval(tick, FONT_CACHE_TTL_MS)
  return () => clearInterval(timer)
}
