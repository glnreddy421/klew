import { useMemo } from 'react'
import { layoutBrailleDots, textToDots } from '../lib/brailleText.js'

/**
 * Text drawn as braille-style dot lettering (SVG, no font dependency).
 * Announced as the plain text; the dots are hidden from assistive tech.
 */
export function BrailleWordmark({
  text,
  scale = 2,
  pitch = 7.5,
  cellGap = 3.5,
  radius = 2.8,
  className = '',
}) {
  const layout = useMemo(
    () => layoutBrailleDots(textToDots(text, { scale }), { pitch, cellGap, radius }),
    [text, scale, pitch, cellGap, radius],
  )
  return (
    <svg
      className={['braille-wordmark', className].filter(Boolean).join(' ')}
      viewBox={`0 0 ${layout.width} ${layout.height}`}
      width={layout.width}
      height={layout.height}
      role="img"
      aria-label={text}
    >
      {layout.points.map((p) => (
        <circle
          key={`${p.x}-${p.y}`}
          className="braille-wordmark-dot"
          cx={p.cx}
          cy={p.cy}
          r={radius}
          style={{ '--col': p.x }}
        />
      ))}
    </svg>
  )
}
