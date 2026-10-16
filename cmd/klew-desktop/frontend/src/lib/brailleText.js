/**
 * Braille-style dot lettering: text is drawn on a dot grid with a 5×7 pixel
 * font, then laid out in braille cells (2 dots wide × 4 tall, with a gap
 * between cells). Purely decorative — callers must expose the plain text to
 * assistive tech.
 */

const GLYPHS = {
  A: ['01110', '10001', '10001', '11111', '10001', '10001', '10001'],
  B: ['11110', '10001', '10001', '11110', '10001', '10001', '11110'],
  C: ['01110', '10001', '10000', '10000', '10000', '10001', '01110'],
  D: ['11110', '10001', '10001', '10001', '10001', '10001', '11110'],
  E: ['11111', '10000', '10000', '11110', '10000', '10000', '11111'],
  F: ['11111', '10000', '10000', '11110', '10000', '10000', '10000'],
  G: ['01110', '10001', '10000', '10111', '10001', '10001', '01111'],
  H: ['10001', '10001', '10001', '11111', '10001', '10001', '10001'],
  I: ['01110', '00100', '00100', '00100', '00100', '00100', '01110'],
  J: ['00111', '00010', '00010', '00010', '00010', '10010', '01100'],
  K: ['10001', '10010', '10100', '11000', '10100', '10010', '10001'],
  L: ['10000', '10000', '10000', '10000', '10000', '10000', '11111'],
  M: ['10001', '11011', '10101', '10101', '10001', '10001', '10001'],
  N: ['10001', '10001', '11001', '10101', '10011', '10001', '10001'],
  O: ['01110', '10001', '10001', '10001', '10001', '10001', '01110'],
  P: ['11110', '10001', '10001', '11110', '10000', '10000', '10000'],
  Q: ['01110', '10001', '10001', '10001', '10101', '10010', '01101'],
  R: ['11110', '10001', '10001', '11110', '10100', '10010', '10001'],
  S: ['01111', '10000', '10000', '01110', '00001', '00001', '11110'],
  T: ['11111', '00100', '00100', '00100', '00100', '00100', '00100'],
  U: ['10001', '10001', '10001', '10001', '10001', '10001', '01110'],
  V: ['10001', '10001', '10001', '10001', '10001', '01010', '00100'],
  W: ['10001', '10001', '10001', '10101', '10101', '10101', '01010'],
  X: ['10001', '10001', '01010', '00100', '01010', '10001', '10001'],
  Y: ['10001', '10001', '01010', '00100', '00100', '00100', '00100'],
  Z: ['11111', '00001', '00010', '00100', '01000', '10000', '11111'],
  ' ': ['00000', '00000', '00000', '00000', '00000', '00000', '00000'],
}

export const GLYPH_WIDTH = 5
export const GLYPH_HEIGHT = 7
const LETTER_GAP = 1

export const CELL_COLS = 2
export const CELL_ROWS = 4

/** Dot grid (array of rows of 0/1) for `text`, each font pixel drawn `scale`×`scale`. */
export function textToDots(text, { scale = 1 } = {}) {
  const chars = [...String(text || '').toUpperCase()].map((c) => GLYPHS[c] || GLYPHS[' '])
  if (!chars.length) return []
  const rows = []
  for (let gy = 0; gy < GLYPH_HEIGHT; gy++) {
    const row = []
    chars.forEach((glyph, i) => {
      if (i > 0) for (let g = 0; g < LETTER_GAP; g++) row.push(0)
      for (const bit of glyph[gy]) row.push(bit === '1' ? 1 : 0)
    })
    const scaled = row.flatMap((v) => Array(scale).fill(v))
    for (let s = 0; s < scale; s++) rows.push(scaled)
  }
  return rows
}

/**
 * Positions of raised dots for SVG drawing. Dots sit on a `pitch` grid with an
 * extra `cellGap` after every braille cell so the 2×4 cell rhythm shows.
 */
export function layoutBrailleDots(dots, { pitch = 6, cellGap = 3, radius = 2 } = {}) {
  const rows = dots.length
  const cols = rows ? Math.max(...dots.map((r) => r.length)) : 0
  const pos = (i, cell) => i * pitch + Math.floor(i / cell) * cellGap + radius
  const points = []
  dots.forEach((row, y) => {
    row.forEach((on, x) => {
      if (on) points.push({ x, y, cx: pos(x, CELL_COLS), cy: pos(y, CELL_ROWS) })
    })
  })
  const span = (n, cell) => (n ? pos(n - 1, cell) + radius : 0)
  return { points, width: span(cols, CELL_COLS), height: span(rows, CELL_ROWS), cols, rows }
}
