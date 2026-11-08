/** Shared dock glyphs: terminal (top-bar toggle, shell tabs) and live log streams. */
export function TerminalGlyph({ size = 14 }) {
  return (
    <svg viewBox="0 0 16 16" width={size} height={size} fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <rect x="1.75" y="2.75" width="12.5" height="10.5" rx="2" />
      <path d="M4.5 6.25L6.75 8 4.5 9.75" />
      <path d="M8.5 10h3" />
    </svg>
  )
}

/** Live log stream (multi-source) — use only next to pod-scoped Stream logs actions. */
export function StreamLogsGlyph({ size = 14 }) {
  return (
    <svg viewBox="0 0 16 16" width={size} height={size} fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" aria-hidden="true">
      <path d="M2.5 4h7" />
      <path d="M4.5 7h9" opacity="0.8" />
      <path d="M2.5 10h5.5" />
      <path d="M4.5 13h6.5" opacity="0.6" />
      <circle cx="12.75" cy="4" r="1.2" fill="currentColor" stroke="none" />
    </svg>
  )
}
