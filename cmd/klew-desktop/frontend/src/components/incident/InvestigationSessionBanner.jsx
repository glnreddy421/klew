/** Compact banner while an investigation session is active on Resources. */
export function InvestigationSessionBanner({
  namespaceLabel,
  query = '',
  starting = false,
}) {
  if (!namespaceLabel && !query && !starting) return null

  return (
    <div className="investigation-session-banner" role="status" aria-live="polite">
      <span className="investigation-session-badge">Investigating</span>
      {namespaceLabel && (
        <span className="investigation-session-ns mono">{namespaceLabel}</span>
      )}
      {query ? (
        <>
          <span className="investigation-session-sep muted" aria-hidden="true">·</span>
          <span className="investigation-session-query mono">{query}</span>
        </>
      ) : starting ? (
        <span className="muted">Starting…</span>
      ) : null}
      <span className="investigation-session-hint muted">
        Resources locked to investigation scope
      </span>
    </div>
  )
}
