import { DEFAULT_THEME, getTheme } from './themes'

// The terminal always follows the app theme (light or dark), like an IDE's integrated terminal.

// xterm's default ANSI palette assumes a dark background; white/yellow vanish on light shells.
const LIGHT_ANSI = {
  red: '#c62828',
  green: '#2e7d32',
  yellow: '#a66a00',
  blue: '#1565c0',
  magenta: '#8e24aa',
  cyan: '#00838f',
  white: '#6b7280',
  brightRed: '#d32f2f',
  brightGreen: '#388e3c',
  brightYellow: '#b7791f',
  brightBlue: '#1976d2',
  brightMagenta: '#9c27b0',
  brightCyan: '#0097a7',
  brightWhite: '#4b5563',
}

function parseHex(hex) {
  const raw = String(hex || '').trim().replace('#', '')
  const full = raw.length === 3 ? raw.split('').map((c) => c + c).join('') : raw
  if (!/^[0-9a-fA-F]{6}$/.test(full)) return null
  return [0, 2, 4].map((i) => parseInt(full.slice(i, i + 2), 16))
}

/** Mix two #rrggbb colors; weight is the share of `a` (0–1). xterm decorations need plain hex. */
export function mixHex(a, b, weight) {
  const ca = parseHex(a)
  const cb = parseHex(b)
  if (!ca || !cb) return a
  const w = Math.min(1, Math.max(0, weight))
  return `#${ca
    .map((v, i) => Math.round(v * w + cb[i] * (1 - w)).toString(16).padStart(2, '0'))
    .join('')}`
}

function hexAlpha(hex, alpha) {
  const c = parseHex(hex)
  if (!c) return hex
  return `rgba(${c[0]}, ${c[1]}, ${c[2]}, ${alpha})`
}

/** Terminal colors for an app theme: CSS vars for chrome, xterm theme, find highlights. */
export function terminalPalette(themeId) {
  const theme = getTheme(themeId || DEFAULT_THEME)
  const v = theme.vars
  const light = theme.mode === 'light'
  const bg = v['--surface']
  const fg = v['--text']
  const accent = v['--accent']
  const accentStrong = v['--accent-hover'] || accent
  const selection = hexAlpha(accent, light ? 0.22 : 0.32)
  return {
    css: {
      '--terminal-bg': bg,
      '--terminal-fg': fg,
      '--terminal-fg-muted': v['--text-secondary'],
      '--terminal-border': v['--border'],
      '--terminal-cursor': accent,
      '--terminal-selection': selection,
    },
    xterm: {
      background: bg,
      foreground: fg,
      cursor: accent,
      cursorAccent: bg,
      selectionBackground: selection,
      black: light ? fg : bg,
      brightBlack: v['--text-secondary'],
      ...(light ? LIGHT_ANSI : null),
    },
    find: {
      matchBackground: mixHex(accent, bg, light ? 0.18 : 0.25),
      matchBorder: accent,
      matchOverviewRuler: accent,
      activeMatchBackground: mixHex(accent, bg, light ? 0.34 : 0.45),
      activeMatchBorder: accentStrong,
      activeMatchColorOverviewRuler: accentStrong,
    },
  }
}

export function terminalAppearanceStyle(themeId) {
  return terminalPalette(themeId).css
}

export function terminalXtermTheme(themeId) {
  return terminalPalette(themeId).xterm
}

export function terminalFindDecorations(themeId) {
  return terminalPalette(themeId).find
}
