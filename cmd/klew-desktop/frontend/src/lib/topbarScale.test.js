import { describe, expect, it } from 'vitest'
import {
  bumpTopbarScale,
  normalizeTopbarScale,
  TOPBAR_SCALE_DEFAULT,
  TOPBAR_SCALE_MAX,
  TOPBAR_SCALE_MIN,
} from './topbarScale.js'

describe('topbarScale', () => {
  it('defaults to 100', () => {
    expect(normalizeTopbarScale(undefined)).toBe(TOPBAR_SCALE_DEFAULT)
  })

  it('clamps and steps scale values', () => {
    expect(normalizeTopbarScale(102)).toBe(100)
    expect(normalizeTopbarScale(108)).toBe(110)
    expect(normalizeTopbarScale(TOPBAR_SCALE_MIN - 1)).toBe(TOPBAR_SCALE_MIN)
    expect(normalizeTopbarScale(TOPBAR_SCALE_MAX + 10)).toBe(TOPBAR_SCALE_MAX)
  })

  it('bumps in 5% steps', () => {
    expect(bumpTopbarScale(100, 5)).toBe(105)
    expect(bumpTopbarScale(100, -5)).toBe(95)
  })
})
