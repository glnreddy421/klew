import { normalizeKind } from '../lib/k8sIcons'
import { GENERIC_GLYPH, GLYPH_STROKE, KIND_GLYPHS, STREAM_GLYPHS, glyphForKind } from './icons/kindGlyphs.jsx'

export { normalizeKind } from '../lib/k8sIcons'

const GLYPH_BY_LOWER = Object.fromEntries(
  Object.entries(KIND_GLYPHS).map(([kind, glyph]) => [kind.toLowerCase(), glyph]),
)

/** Glyph for any kind spelling; custom and unknown kinds get the generic glyph. */
export function kindGlyph(kind) {
  const normalized = normalizeKind(kind)
  return KIND_GLYPHS[normalized] || GLYPH_BY_LOWER[normalized.toLowerCase()] || glyphForKind(normalized)
}

function Glyph({ glyph, size, className, label }) {
  const px = typeof size === 'number' ? `${size}px` : size
  return (
    <span className={`kind-icon ${className}`.trim()} title={label} aria-label={label} role="img">
      <svg viewBox="0 0 24 24" width={px} height={px} aria-hidden="true" focusable="false" {...GLYPH_STROKE}>
        {glyph}
      </svg>
    </span>
  )
}

export function KindIcon({ kind, size = 22, className = '', title }) {
  const label = title || normalizeKind(kind)
  return <Glyph glyph={kindGlyph(kind)} size={size} className={className} label={label} />
}

/** Icons for live stream source types (LOG, EVENT, …). */
export function StreamSourceIcon({ source, size = 18, className = '' }) {
  const glyph = STREAM_GLYPHS[String(source || '').toUpperCase()] || GENERIC_GLYPH
  return (
    <Glyph
      glyph={glyph}
      size={size}
      className={`kind-icon--stream ${className}`.trim()}
      label={source || 'Stream'}
    />
  )
}
