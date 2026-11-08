import { describe, expect, it } from 'vitest'
import {
  mixHex,
  terminalAppearanceStyle,
  terminalFindDecorations,
  terminalXtermTheme,
} from './terminalAppearance'
import { buildTerminalFindOptions } from './terminalFind'
import { getTheme, THEMES } from './themes'

describe('terminal appearance', () => {
  it('uses the app theme surface as the terminal background', () => {
    for (const theme of THEMES) {
      expect(terminalAppearanceStyle(theme.id)['--terminal-bg']).toBe(theme.vars['--surface'])
      expect(terminalXtermTheme(theme.id).background).toBe(theme.vars['--surface'])
      expect(terminalXtermTheme(theme.id).foreground).toBe(theme.vars['--text'])
    }
  })

  it('uses a light-safe ANSI palette only on light themes', () => {
    expect(terminalXtermTheme('light').white).not.toBe('#ffffff')
    expect(terminalXtermTheme('dark').white).toBeUndefined()
  })

  it('resolves unknown and legacy ids through the theme map', () => {
    expect(terminalXtermTheme('nope').background).toBe(getTheme('nope').vars['--surface'])
    expect(terminalXtermTheme('ocean-light').background).toBe(getTheme('light').vars['--surface'])
  })

  it('builds hex find decorations from the accent', () => {
    const opts = buildTerminalFindOptions('dark')
    expect(opts.decorations.matchBorder).toBe(getTheme('dark').vars['--accent'])
    expect(opts.decorations.matchBackground).toMatch(/^#[0-9a-f]{6}$/)
    expect(terminalFindDecorations('dark')).toEqual(opts.decorations)
  })

  it('mixes hex colors', () => {
    expect(mixHex('#ffffff', '#000000', 0.5)).toBe('#808080')
    expect(mixHex('#ff0000', '#0000ff', 1)).toBe('#ff0000')
  })
})
