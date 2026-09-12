const STROKE = 1.5

function GuideIcon({ size = 20, children }) {
  return (
    <svg
      viewBox="0 0 24 24"
      width={size}
      height={size}
      fill="none"
      stroke="currentColor"
      strokeWidth={STROKE}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {children}
    </svg>
  )
}

export function SurfaceGuideIcon({ id, size = 20 }) {
  switch (id) {
    case 'patterns':
      return (
        <GuideIcon size={size}>
          <path d="M3.5 12h2.2l1.6-3.8 2.2 7.6 1.8-5.2 1.4 1.1H20.5" />
        </GuideIcon>
      )
    case 'failures':
      return (
        <GuideIcon size={size}>
          <path d="M12 8.25v4.25M12 15.5h.01" />
          <path d="M10.35 4.45 3.95 17.2a1.35 1.35 0 001.17 2h13.76a1.35 1.35 0 001.17-2L13.65 4.45a1.35 1.35 0 00-2.3 0z" />
        </GuideIcon>
      )
    case 'evidence':
      return (
        <GuideIcon size={size}>
          <path d="M14.5 3.5H8a1.5 1.5 0 00-1.5 1.5v14A1.5 1.5 0 008 20.5h8a1.5 1.5 0 001.5-1.5V8z" />
          <path d="M14.5 3.5V8h4.5M9 13h6M9 16.5h4" />
        </GuideIcon>
      )
    case 'incident':
    default:
      return (
        <GuideIcon size={size}>
          <circle cx="12" cy="12" r="6.25" />
          <path d="M12 8v3.5l2 1.5" />
        </GuideIcon>
      )
  }
}

export function InvestigateScopeIcon({ size = 18 }) {
  return (
    <GuideIcon size={size}>
      <circle cx="12" cy="12" r="6" />
      <path d="M12 9v6M9 12h6" />
    </GuideIcon>
  )
}

export function SurfaceGuidePanelIcon({ kind, size = 13 }) {
  if (kind === 'do') {
    return (
      <svg viewBox="0 0 16 16" width={size} height={size} fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
        <path d="M3.5 8.25 6.5 11l6-6.5" />
      </svg>
    )
  }
  return (
    <svg viewBox="0 0 16 16" width={size} height={size} fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <circle cx="8" cy="8" r="4.75" />
      <path d="M8 5.25V8l1.75 1.25" />
    </svg>
  )
}

export function SurfaceGuideCheckIcon({ size = 11 }) {
  return (
    <svg viewBox="0 0 12 12" width={size} height={size} fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M2.75 6.1 5 8.35 9.25 3.75" />
    </svg>
  )
}
