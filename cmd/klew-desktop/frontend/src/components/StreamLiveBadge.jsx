/** Live status dot — grey when idle, green when streaming. */
export function StreamLiveBadge({
  running = false,
  logTailEngaged = false,
  logTailPaused = false,
  className = '',
}) {
  const streaming = running && logTailEngaged && !logTailPaused
  const tone = streaming ? 'is-live' : 'is-off'
  const label = streaming ? 'Live logs streaming' : 'Live logs idle'

  return (
    <span
      className={`console-live-dot ${tone} ${className}`.trim()}
      role="img"
      aria-label={label}
      title={label}
    />
  )
}
