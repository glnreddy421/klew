/** Telescope: marks things Klew can investigate. */
export function InvestigateIcon({ size = 13, className = '' }) {
  return (
    <svg
      className={`investigate-icon ${className}`.trim()}
      width={size}
      height={size}
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.4"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M2.2 9.4l8.6-5 1.2 2.1-8.6 5z" />
      <path d="M10.8 3.6l2.4-1.4 1.6 2.8-2.4 1.4z" />
      <path d="M7.6 9.6l-2.4 4.8M7.6 9.6l2.4 4.8" />
    </svg>
  )
}
