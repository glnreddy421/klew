import { formatDataAge, isDataStale } from '../lib/dataAge.js'

export function DataAgeLabel({
  updatedAt,
  live = false,
  staleAfterMs = 60 * 1000,
  className = '',
}) {
  if (!updatedAt) return null
  const label = live ? 'Live' : formatDataAge(updatedAt)
  if (!label) return null
  const stale = !live && isDataStale(updatedAt, staleAfterMs)
  return (
    <span
      className={[
        'data-age-label',
        live ? 'is-live' : '',
        stale ? 'is-stale' : '',
        className,
      ].filter(Boolean).join(' ')}
      title={live ? 'Streaming changes from the cluster' : label}
    >
      {live && <span className="data-age-live-dot" aria-hidden="true" />}
      {label}
    </span>
  )
}
