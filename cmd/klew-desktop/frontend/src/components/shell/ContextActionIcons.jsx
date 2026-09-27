/** Icons for context list actions (favorite, launch default). */

const VB = 24
const STROKE = 1.65

function svgProps({ size, className, extraClass }) {
  const cls = ['context-action-icon', extraClass, className].filter(Boolean).join(' ')
  return { className: cls, viewBox: `0 0 ${VB} ${VB}`, width: size, height: size, 'aria-hidden': true }
}

/** Balanced 5-point star — Lucide-aligned proportions. */
const STAR =
  'M12 2.4l2.85 5.77 6.38.93-4.62 4.5 1.09 6.35L12 17.9l-5.7 2.99 1.09-6.35-4.62-4.5 6.38-.93L12 2.4z'

export function FavoriteStarIcon({ filled = false, size = 16, className = '' }) {
  const props = svgProps({
    size,
    className,
    extraClass: ['context-action-star', filled ? 'is-filled' : 'is-outline'].join(' '),
  })

  if (filled) {
    return (
      <svg {...props}>
        <path d={STAR} fill="currentColor" stroke="none" />
      </svg>
    )
  }

  return (
    <svg {...props} fill="none">
      <path
        d={STAR}
        stroke="currentColor"
        strokeWidth={STROKE}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  )
}

const HOME_SHELL = 'M4 10.2 12 3.6l8 6.6V20a1.6 1.6 0 0 1-1.6 1.6H5.6A1.6 1.6 0 0 1 4 20V10.2z'
const HOME_DOOR = 'M9.8 21.6V13.2h4.4v8.4'

export function LaunchDefaultIcon({ active = false, size = 16, className = '' }) {
  const props = svgProps({
    size,
    className,
    extraClass: ['context-action-home', active ? 'is-active' : 'is-outline'].join(' '),
  })

  return (
    <svg {...props} fill="none">
      <path
        d={HOME_SHELL}
        fill={active ? 'currentColor' : 'none'}
        fillOpacity={active ? 0.22 : 0}
        stroke="currentColor"
        strokeWidth={STROKE}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d={HOME_DOOR}
        stroke="currentColor"
        strokeWidth={STROKE}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  )
}
