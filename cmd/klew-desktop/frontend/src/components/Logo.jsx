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
