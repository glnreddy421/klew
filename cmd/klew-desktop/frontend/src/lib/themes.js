export const THEME_STORAGE_KEY = 'klew-desktop-theme'
export const DEFAULT_THEME = 'ocean'

const DARK_HOVER = {
  '--surface-hover': 'rgba(255, 255, 255, 0.04)',
  '--surface-hover-strong': 'rgba(255, 255, 255, 0.06)',
}

const LIGHT_HOVER = {
  '--surface-hover': 'rgba(0, 0, 0, 0.04)',
  '--surface-hover-strong': 'rgba(0, 0, 0, 0.07)',
}

/** Dark UI palettes — accent + shell tints. */
const DARK_THEMES = [
  {
    id: 'ocean',
    name: 'Ocean',
    description: 'Docker-style blue — default',
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
      ...DARK_HOVER,
    },
  },
  {
    id: 'violet',
    name: 'Violet',
    description: 'Indigo shell with purple accent',
    mode: 'dark',
    swatch: ['#1a1824', '#8b5cf6'],
    vars: {
      '--sidebar-bg': '#1a1824',
      '--sidebar-hover': '#252033',
      '--sidebar-active': '#2e2640',
      '--workspace-bg': '#0f0d14',
      '--surface': '#16141f',
      '--surface-2': '#1e1a28',
      '--border': '#2f2a3d',
      '--border-light': '#3f3854',
      '--text': '#eceaf4',
      '--text-secondary': '#a59bb8',
      '--accent': '#8b5cf6',
      '--accent-hover': '#9d6fff',
      '--accent-dim': 'rgba(139, 92, 246, 0.16)',
      ...DARK_HOVER,
    },
  },
  {
    id: 'emerald',
    name: 'Emerald',
    description: 'Deep green workspace, teal accent',
    mode: 'dark',
    swatch: ['#141c1a', '#10b981'],
    vars: {
      '--sidebar-bg': '#141c1a',
      '--sidebar-hover': '#1c2623',
      '--sidebar-active': '#24312c',
      '--workspace-bg': '#0c1110',
      '--surface': '#131a18',
      '--surface-2': '#1a2421',
      '--border': '#2a3834',
      '--border-light': '#3a4d47',
      '--text': '#e6f0ec',
      '--text-secondary': '#93a89f',
      '--accent': '#10b981',
      '--accent-hover': '#22c993',
      '--accent-dim': 'rgba(16, 185, 129, 0.15)',
      ...DARK_HOVER,
    },
  },
  {
    id: 'amber',
    name: 'Amber',
    description: 'Warm charcoal with gold accent',
    mode: 'dark',
    swatch: ['#1c1914', '#f59e0b'],
    vars: {
      '--sidebar-bg': '#1c1914',
      '--sidebar-hover': '#262219',
      '--sidebar-active': '#322c22',
      '--workspace-bg': '#110f0c',
      '--surface': '#1a1712',
      '--surface-2': '#242019',
      '--border': '#3a342a',
      '--border-light': '#4d4538',
      '--text': '#f2ece3',
      '--text-secondary': '#b0a593',
      '--accent': '#f59e0b',
      '--accent-hover': '#fbbf24',
      '--accent-dim': 'rgba(245, 158, 11, 0.16)',
      ...DARK_HOVER,
    },
  },
  {
    id: 'rose',
    name: 'Rose',
    description: 'Soft dark rose with coral accent',
    mode: 'dark',
    swatch: ['#1c1518', '#f43f5e'],
    vars: {
      '--sidebar-bg': '#1c1518',
      '--sidebar-hover': '#271e22',
      '--sidebar-active': '#33262c',
      '--workspace-bg': '#110d0f',
      '--surface': '#1a1417',
      '--surface-2': '#241c20',
      '--border': '#3a2e34',
      '--border-light': '#4d3d45',
      '--text': '#f5ecef',
      '--text-secondary': '#b09aa3',
      '--accent': '#f43f5e',
      '--accent-hover': '#fb7185',
      '--accent-dim': 'rgba(244, 63, 94, 0.15)',
      ...DARK_HOVER,
    },
  },
]

/** Light UI palettes — same accents, bright shells. */
const LIGHT_THEMES = [
  {
    id: 'ocean-light',
    name: 'Ocean',
    description: 'Clean white workspace, blue accent',
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
      ...LIGHT_HOVER,
    },
  },
  {
    id: 'violet-light',
    name: 'Violet',
    description: 'Soft lavender shell, purple accent',
    mode: 'light',
    swatch: ['#f0edf8', '#7c3aed'],
    vars: {
      '--sidebar-bg': '#f0edf8',
      '--sidebar-hover': '#e6e0f2',
      '--sidebar-active': '#dcd4eb',
      '--workspace-bg': '#faf8fd',
      '--surface': '#ffffff',
      '--surface-2': '#f3f0fa',
      '--border': '#ddd6ea',
      '--border-light': '#c9bfd8',
      '--text': '#1f1830',
      '--text-secondary': '#6b6080',
      '--accent': '#7c3aed',
      '--accent-hover': '#6d28d9',
      '--accent-dim': 'rgba(124, 58, 237, 0.12)',
      ...LIGHT_HOVER,
    },
  },
  {
    id: 'emerald-light',
    name: 'Emerald',
    description: 'Mint workspace, green accent',
    mode: 'light',
    swatch: ['#ecf5f1', '#059669'],
    vars: {
      '--sidebar-bg': '#ecf5f1',
      '--sidebar-hover': '#dfece6',
      '--sidebar-active': '#d0e4db',
      '--workspace-bg': '#f7fbf9',
      '--surface': '#ffffff',
      '--surface-2': '#eef6f2',
      '--border': '#cfe0d8',
      '--border-light': '#b8cfc4',
      '--text': '#142820',
      '--text-secondary': '#5a7568',
      '--accent': '#059669',
      '--accent-hover': '#047857',
      '--accent-dim': 'rgba(5, 150, 105, 0.12)',
      ...LIGHT_HOVER,
    },
  },
  {
    id: 'amber-light',
    name: 'Amber',
    description: 'Warm paper tone, gold accent',
    mode: 'light',
    swatch: ['#f7f3eb', '#d97706'],
    vars: {
      '--sidebar-bg': '#f7f3eb',
      '--sidebar-hover': '#efe8dc',
      '--sidebar-active': '#e5dcc8',
      '--workspace-bg': '#fcfaf6',
      '--surface': '#ffffff',
      '--surface-2': '#f5f0e6',
      '--border': '#e0d5c4',
      '--border-light': '#cfc0a8',
      '--text': '#2a2218',
      '--text-secondary': '#7a6e5c',
      '--accent': '#d97706',
      '--accent-hover': '#b45309',
      '--accent-dim': 'rgba(217, 119, 6, 0.14)',
      ...LIGHT_HOVER,
    },
  },
  {
    id: 'rose-light',
    name: 'Rose',
    description: 'Blush shell, coral accent',
    mode: 'light',
    swatch: ['#faf0f2', '#e11d48'],
    vars: {
      '--sidebar-bg': '#faf0f2',
      '--sidebar-hover': '#f3e4e8',
      '--sidebar-active': '#ebd6dc',
      '--workspace-bg': '#fdf8f9',
      '--surface': '#ffffff',
      '--surface-2': '#f8eef0',
      '--border': '#e8d4da',
      '--border-light': '#d8bcc4',
      '--text': '#2a141c',
      '--text-secondary': '#8a6570',
      '--accent': '#e11d48',
      '--accent-hover': '#be123c',
      '--accent-dim': 'rgba(225, 29, 72, 0.12)',
      ...LIGHT_HOVER,
    },
  },
]

export const THEMES = [...DARK_THEMES, ...LIGHT_THEMES]

export const THEME_GROUPS = [
  { id: 'dark', label: 'Dark', themes: DARK_THEMES },
  { id: 'light', label: 'Light', themes: LIGHT_THEMES },
]

export function getTheme(id) {
  return THEMES.find((t) => t.id === id) || THEMES[0]
}

export function applyTheme(id) {
  const theme = getTheme(id)
  const root = document.documentElement
  root.setAttribute('data-theme', theme.id)
  root.setAttribute('data-theme-mode', theme.mode)
  root.style.colorScheme = theme.mode
  for (const [key, value] of Object.entries(theme.vars)) {
    root.style.setProperty(key, value)
  }
  try {
    localStorage.setItem(THEME_STORAGE_KEY, theme.id)
  } catch {
    // ignore storage errors
  }
  return theme.id
}

export function readStoredTheme() {
  try {
    const stored = localStorage.getItem(THEME_STORAGE_KEY)
    if (stored && THEMES.some((t) => t.id === stored)) {
      return stored
    }
  } catch {
    // ignore
  }
  return DEFAULT_THEME
}

export function initTheme() {
  applyTheme(readStoredTheme())
}
