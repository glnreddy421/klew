import { describe, expect, it } from 'vitest'
import {
  GLYPH_HEIGHT,
  GLYPH_WIDTH,
  layoutBrailleDots,
  textToDots,
} from './brailleText.js'

describe('brailleText', () => {
  it('lays glyphs out with a one-dot gap and scales them', () => {
    const dots = textToDots('KL')
    expect(dots).toHaveLength(GLYPH_HEIGHT)
    expect(dots[0]).toHaveLength(GLYPH_WIDTH * 2 + 1)

    const scaled = textToDots('KL', { scale: 2 })
    expect(scaled).toHaveLength(GLYPH_HEIGHT * 2)
    expect(scaled[0]).toHaveLength((GLYPH_WIDTH * 2 + 1) * 2)
  })

  it('draws the L glyph as a column plus a bottom bar', () => {
    const dots = textToDots('L')
    expect(dots.map((r) => r[0])).toEqual([1, 1, 1, 1, 1, 1, 1])
    expect(dots[GLYPH_HEIGHT - 1]).toEqual([1, 1, 1, 1, 1])
  })

  it('is case-insensitive and blanks unknown characters', () => {
    expect(textToDots('klew')).toEqual(textToDots('KLEW'))
    expect(textToDots('?')).toEqual(textToDots(' '))
  })

  it('places only raised dots, adding a gap between braille cells', () => {
    const { points, width, height } = layoutBrailleDots(
      [[1, 0, 1], [0, 0, 0], [0, 0, 0], [0, 0, 0], [1, 0, 0]],
      { pitch: 6, cellGap: 3, radius: 2 },
    )
    expect(points.map((p) => [p.cx, p.cy])).toEqual([
      [2, 2], // cell (0,0)
      [2 + 12 + 3, 2], // third column starts the next cell → +gap
      [2, 2 + 24 + 3], // fifth row starts the next cell row → +gap
    ])
    expect(width).toBe(17 + 2)
    expect(height).toBe(29 + 2)
  })

  it('handles empty input', () => {
    expect(layoutBrailleDots([])).toEqual({ points: [], width: 0, height: 0, cols: 0, rows: 0 })
  })
})
