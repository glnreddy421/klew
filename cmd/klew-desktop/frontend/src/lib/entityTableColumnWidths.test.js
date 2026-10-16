import { describe, expect, it } from 'vitest'
import {
  COLUMN_FALLBACK_WIDTH,
  COLUMN_MAX_WIDTH,
  COLUMN_MIN_WIDTH,
  clampColumnWidth,
  completeColumnWidths,
  hasColumnWidths,
  totalColumnWidth,
} from './entityTableColumnWidths.js'

describe('entityTableColumnWidths', () => {
  it('clamps and rounds widths', () => {
    expect(clampColumnWidth(10)).toBe(COLUMN_MIN_WIDTH)
    expect(clampColumnWidth(5000)).toBe(COLUMN_MAX_WIDTH)
    expect(clampColumnWidth(150.6)).toBe(151)
    expect(clampColumnWidth('nope')).toBe(COLUMN_MIN_WIDTH)
  })

  it('prefers saved, then measured, then fallback widths', () => {
    const out = completeColumnWidths({ name: 300 }, ['name', 'age', 'status'], { name: 200, age: 90 })
    expect(out).toEqual({ name: 300, age: 90, status: COLUMN_FALLBACK_WIDTH })
  })

  it('drops saved widths for columns no longer visible', () => {
    expect(completeColumnWidths({ gone: 200, name: 210 }, ['name'])).toEqual({ name: 210 })
  })

  it('sums widths for the table', () => {
    expect(totalColumnWidth({ a: 100, b: 50 }, ['a', 'b'])).toBe(150)
    expect(totalColumnWidth({ a: 100 }, ['a', 'b'])).toBe(100 + COLUMN_FALLBACK_WIDTH)
  })

  it('detects whether any width is set', () => {
    expect(hasColumnWidths({})).toBe(false)
    expect(hasColumnWidths(null)).toBe(false)
    expect(hasColumnWidths({ name: 100 })).toBe(true)
  })
})
