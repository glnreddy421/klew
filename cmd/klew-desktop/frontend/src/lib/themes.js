export const THEME_STORAGE_KEY = 'klew-desktop-theme'

/** Appearance preference: a fixed theme, or follow the OS. */
export const THEME_SYSTEM = 'system'
export const DEFAULT_THEME = THEME_SYSTEM

function parseHexColor(hex) {
  const raw = String(hex || '').trim().replace('#', '')
  if (raw.length === 3) {
    return raw.split('').map((c) => parseInt(c + c, 16))
  }
  if (raw.length !== 6) return null
  return [
    parseInt(raw.slice(0, 2), 16),
    parseInt(raw.slice(2, 4), 16),
    parseInt(raw.slice(4, 6), 16),
  ]
}

/** Pick readable label color for accent-filled buttons (Investigate, primary CTAs). */
export function onAccentForColor(accent) {
  const rgb = parseHexColor(accent)
  if (!rgb || rgb.some((n) => Number.isNaN(n))) return '#ffffff'
  const channelLum = (c) => {
    const s = c / 255
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4
  }
  const [r, g, b] = rgb.map(channelLum)
  const l = 0.2126 * r + 0.7152 * g + 0.0722 * b
  return l > 0.55 ? '#0a0a0a' : '#ffffff'
}

/** The two palettes. Everything else in the app derives from these tokens. */
export const THEMES = [
  {
    id: 'light',
    name: 'Light',
    description: 'Bright workspace',
    mode: 'light',
    swatch: ['#eef1f5', '#1d8fe1'],
    vars: {
      '--sidebar-bg': '#eef1f5',
      '--sidebar-hover': '#e3e8ee',
      '--sidebar-active': '#d8e0e8',
      '--workspace-bg': '#f8f9fb',
      '--surface': '#ffffff',
      '--surface-2': '#f0f3f6',
      '--border': '#d8dee6',
      '--border-light': '#c4ccd6',
      '--text': '#1a1d21',
      '--text-secondary': '#5f6b7a',
      '--accent': '#1d8fe1',
      '--accent-hover': '#1578c2',
      '--accent-dim': 'rgba(29, 143, 225, 0.12)',
      '--surface-hover': 'rgba(0, 0, 0, 0.04)',
      '--surface-hover-strong': 'rgba(0, 0, 0, 0.07)',
    },
  },
  {
    id: 'dark',
    name: 'Dark',
    description: 'Dim workspace',
    mode: 'dark',
    swatch: ['#1b1f26', '#1d8fe1'],
    vars: {
      '--sidebar-bg': '#1b1f26',
      '--sidebar-hover': '#252a33',
      '--sidebar-active': '#2a3140',
      '--workspace-bg': '#101418',
      '--surface': '#171b21',
      '--surface-2': '#1e242c',
      '--border': '#2d3540',
      '--border-light': '#3d4654',
      '--text': '#e8eaed',
      '--text-secondary': '#9aa3af',
      '--accent': '#1d8fe1',
      '--accent-hover': '#2496ed',
      '--accent-dim': 'rgba(29, 143, 225, 0.15)',
      '--surface-hover': 'rgba(255, 255, 255, 0.04)',
      '--surface-hover-strong': 'rgba(255, 255, 255, 0.06)',
    },
  },
]

/** Options shown in Settings → Appearance. */
export const THEME_OPTIONS = [
  { id: 'light', name: 'Light', description: 'Always bright' },
  { id: 'dark', name: 'Dark', description: 'Always dim' },
  { id: THEME_SYSTEM, name: 'System', description: 'Follow macOS appearance' },
]

function systemPrefersDark() {
  try {
    return window.matchMedia('(prefers-color-scheme: dark)').matches
  } catch {
    return true
  }
}

/** Normalize a stored or incoming value to a preference, migrating older theme ids. */
export function normalizeThemePref(value) {
  const id = String(value || '').trim()
  if (id === THEME_SYSTEM) return THEME_SYSTEM
  if (THEMES.some((t) => t.id === id)) return id
  if (!id) return DEFAULT_THEME
  // Pre-2.x palettes (ocean, violet, midnight, ocean-light, …) collapse to light/dark.
  return id.endsWith('-light') ? 'light' : 'dark'
}

/** The palette actually in effect for a preference. */
export function resolveThemeId(pref) {
  const normalized = normalizeThemePref(pref)
  if (normalized !== THEME_SYSTEM) return normalized
  return systemPrefersDark() ? 'dark' : 'light'
}

export function getTheme(id) {
  const resolved = resolveThemeId(id)
  return THEMES.find((t) => t.id === resolved) || THEMES[1]
}

const listeners = new Set()

/** Notified with (resolvedId, preference) whenever the effective palette changes. */
export function subscribeTheme(cb) {
  listeners.add(cb)
  return () => listeners.delete(cb)
}

export function applyTheme(pref) {
  const preference = normalizeThemePref(pref)
  const theme = getTheme(preference)
  const root = document.documentElement
  root.setAttribute('data-theme', theme.id)
  root.setAttribute('data-theme-mode', theme.mode)
  root.setAttribute('data-theme-pref', preference)
  root.style.colorScheme = theme.mode
  for (const [key, value] of Object.entries(theme.vars)) {
    root.style.setProperty(key, value)
  }
  root.style.setProperty('--on-accent', onAccentForColor(theme.vars['--accent']))
  // Investigation shell uses dedicated tokens — keep them aligned with appearance accent.
  root.style.setProperty('--investigation-accent', theme.vars['--accent'])
  root.style.setProperty('--investigation-accent-dim', theme.vars['--accent-dim'])
  try {
    localStorage.setItem(THEME_STORAGE_KEY, preference)
  } catch {
    // ignore storage errors
  }
  for (const cb of listeners) cb(theme.id, preference)
  import('./settingsCache.js').then((m) => m.queueSettingsCacheSync()).catch(() => {})
  return preference
}

export function readStoredTheme() {
  try {
    return normalizeThemePref(localStorage.getItem(THEME_STORAGE_KEY))
  } catch {
    return DEFAULT_THEME
  }
}

export function initTheme() {
  const pref = readStoredTheme()
  applyTheme(pref)
  try {
    const mq = window.matchMedia('(prefers-color-scheme: dark)')
    const onChange = () => {
      if (readStoredTheme() === THEME_SYSTEM) applyTheme(THEME_SYSTEM)
    }
    if (mq.addEventListener) mq.addEventListener('change', onChange)
    else mq.addListener(onChange)
  } catch {
    // matchMedia unavailable — fixed themes still work
  }
  return pref
}
