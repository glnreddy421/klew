export function RefreshIcon({ spinning = false, className = '' }) {
  return (
    <svg
      className={['icon-refresh', spinning ? 'is-spinning' : '', className].filter(Boolean).join(' ')}
      viewBox="0 0 16 16"
      width="16"
      height="16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      aria-hidden="true"
    >
      <path d="M13 3v3H10" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M3 13V10H6" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M4.2 6.1A4.5 4.5 0 0 1 12 5.5L13 3M3 13l1-2.5A4.5 4.5 0 0 1 11.8 9.9" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

export function RefreshButton({
  onClick,
  spinning = false,
  disabled = false,
  title = 'Refresh',
  className = 'icon-refresh-btn',
}) {
  return (
    <button
      type="button"
      className={className}
      onClick={onClick}
      disabled={disabled}
      title={title}
      aria-label={title}
    >
      <RefreshIcon spinning={spinning} />
    </button>
  )
}
