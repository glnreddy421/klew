import { k8sIconUrl, normalizeKind, streamIconUrl } from '../lib/k8sIcons'

export { normalizeKind } from '../lib/k8sIcons'

export function KindIcon({ kind, size = 22, className = '', title }) {
  const normalized = normalizeKind(kind)
  const label = title || normalized
  const src = k8sIconUrl(kind)
  const px = typeof size === 'number' ? `${size}px` : size

  return (
    <span
      className={`kind-icon ${className}`.trim()}
      title={label}
      aria-label={label}
      role="img"
    >
      <img src={src} alt="" width={px} height={px} draggable={false} />
    </span>
  )
}

/** Icons for live stream source types (LOG, EVENT, …). */
export function StreamSourceIcon({ source, size = 18, className = '' }) {
  const label = source || 'Stream'
  const src = streamIconUrl(source)
  const px = typeof size === 'number' ? `${size}px` : size

  return (
    <span
      className={`kind-icon kind-icon--stream ${className}`.trim()}
      title={label}
      aria-label={label}
      role="img"
    >
      <img src={src} alt="" width={px} height={px} draggable={false} />
    </span>
  )
}
