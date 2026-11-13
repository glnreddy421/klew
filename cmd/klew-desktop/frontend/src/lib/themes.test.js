import { describe, expect, it } from 'vitest'
import {
  BRAND_ACCENT,
  BRAND_DARK_BG,
  BRAND_DARK_FG,
  BRAND_LIGHT,
  DEFAULT_THEME,
  getTheme,
  normalizeThemePref,
  onAccentForColor,
  resolveThemeId,
  THEME_OPTIONS,
  THEMES,
} from './themes'

describe('themes', () => {
  it('ships exactly one light and one dark palette', () => {
    expect(THEMES.map((t) => t.id).sort()).toEqual(['dark', 'light'])
    expect(THEME_OPTIONS.map((o) => o.id)).toEqual(['light', 'dark', 'system'])
    expect(DEFAULT_THEME).toBe('system')
  })

  it('uses cream light workspace and tour-video near-black dark', () => {
    const light = getTheme('light')
    const dark = getTheme('dark')
    expect(light.vars['--workspace-bg']).toBe(BRAND_LIGHT)
    expect(light.vars['--accent']).toBe(BRAND_DARK_BG)
    expect(dark.vars['--workspace-bg']).toBe(BRAND_DARK_BG)
    expect(BRAND_DARK_BG).toBe('#181b21')
    expect(dark.vars['--surface']).toBe('#1e2228')
    expect(dark.vars['--accent']).toBe(BRAND_DARK_FG)
    expect(onAccentForColor(BRAND_LIGHT)).toBe('#0a0a0a')
    expect(onAccentForColor(BRAND_DARK_FG)).toBe('#0a0a0a')
    expect(onAccentForColor(BRAND_ACCENT)).toBe('#ffffff')
  })

  it('migrates older palette ids to light or dark', () => {
    expect(normalizeThemePref('ocean')).toBe('dark')
    expect(normalizeThemePref('midnight')).toBe('dark')
    expect(normalizeThemePref('rose-light')).toBe('light')
    expect(normalizeThemePref('light')).toBe('light')
    expect(normalizeThemePref('system')).toBe('system')
    expect(normalizeThemePref('')).toBe('system')
  })

  it('resolves system to a real palette', () => {
    expect(['light', 'dark']).toContain(resolveThemeId('system'))
    expect(resolveThemeId('light')).toBe('light')
    expect(getTheme('system').mode).toMatch(/^(light|dark)$/)
  })
})
