/** @typedef {'midnight' | 'black' | 'silver'} TerminalAppearanceId */

export const DEFAULT_TERMINAL_APPEARANCE = 'midnight'

export const TERMINAL_APPEARANCES = [
  {
    id: 'midnight',
    label: 'Midnight',
    description: 'Grey-black — easy on the eyes',
    swatch: ['#0d1117', '#e6edf3'],
  },
  {
    id: 'black',
    label: 'Black',
    description: 'True black — high contrast',
    swatch: ['#000000', '#f5f5f5'],
  },
  {
    id: 'silver',
    label: 'Black silver',
    description: 'Deep black with cool silver text',
    swatch: ['#050506', '#c5cdd8'],
  },
]

const PRESETS = {
  midnight: {
    css: {
      '--terminal-bg': '#0d1117',
      '--terminal-fg': '#e6edf3',
      '--terminal-fg-muted': '#8b949e',
      '--terminal-border': '#30363d',
      '--terminal-cursor': '#58a6ff',
      '--terminal-selection': 'rgba(56, 139, 253, 0.35)',
    },
    xterm: {
      background: '#0d1117',
      foreground: '#e6edf3',
      cursor: '#58a6ff',
      selectionBackground: 'rgba(56, 139, 253, 0.35)',
      brightBlack: '#6e7681',
      black: '#0d1117',
    },
  },
  black: {
    css: {
      '--terminal-bg': '#000000',
      '--terminal-fg': '#f5f5f5',
      '--terminal-fg-muted': '#a3a3a3',
      '--terminal-border': '#2a2a2a',
      '--terminal-cursor': '#ffffff',
      '--terminal-selection': 'rgba(255, 255, 255, 0.22)',
    },
    xterm: {
      background: '#000000',
      foreground: '#f5f5f5',
      cursor: '#ffffff',
      selectionBackground: 'rgba(255, 255, 255, 0.22)',
      brightBlack: '#737373',
      black: '#000000',
    },
  },
  silver: {
    css: {
      '--terminal-bg': '#050506',
      '--terminal-fg': '#c5cdd8',
      '--terminal-fg-muted': '#8b939e',
      '--terminal-border': '#1e2128',
      '--terminal-cursor': '#d4dae3',
      '--terminal-selection': 'rgba(196, 205, 216, 0.18)',
    },
    xterm: {
      background: '#050506',
      foreground: '#c5cdd8',
      cursor: '#d4dae3',
      selectionBackground: 'rgba(196, 205, 216, 0.18)',
      brightBlack: '#6b7280',
      black: '#050506',
    },
  },
}

export function normalizeTerminalAppearance(value) {
  const id = String(value || '').trim()
  if (PRESETS[id]) return id
  return DEFAULT_TERMINAL_APPEARANCE
}

export function terminalAppearancePreset(id) {
  return PRESETS[normalizeTerminalAppearance(id)]
}

export function terminalAppearanceStyle(id) {
  return terminalAppearancePreset(id).css
}

export function terminalXtermTheme(id) {
  return terminalAppearancePreset(id).xterm
}
