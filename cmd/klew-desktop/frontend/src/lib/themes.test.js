import { describe, expect, it } from 'vitest'
import {
  DEFAULT_THEME,
  getTheme,
  normalizeThemePref,
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
