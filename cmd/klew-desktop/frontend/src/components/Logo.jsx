import klewMark from '../assets/klew-mark.png'

export function LogoMark({ className = '' }) {
  return (
    <img
      src={klewMark}
      alt=""
      aria-hidden="true"
      className={`brand-mark ${className}`.trim()}
      draggable={false}
    />
  )
}

/** Vertical KL / EW stack for compact chrome (top bar). */
export function BrandWordmark({ className = '', variant = 'topbar' }) {
  return (
    <span
      className={['brand-wordmark', `brand-wordmark-${variant}`, className].filter(Boolean).join(' ')}
      aria-label="KLEW"
    >
      <span className="brand-wordmark-stack">
        <span className="brand-wordmark-kl">KL</span>
        <span className="brand-wordmark-ew">EW</span>
      </span>
    </span>
  )
}
