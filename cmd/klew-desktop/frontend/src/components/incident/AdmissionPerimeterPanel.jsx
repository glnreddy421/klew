/**
 * Surfaces opt-in admission webhook perimeter checks inside existing investigation views.
 */
export function AdmissionPerimeterPanel({ summary, className = '' }) {
  if (!summary?.missingWorkload) return null

  const denied = summary.status === 'denied'
  const candidates = summary.candidates || []

  return (
    <section className={`card inv-card admission-perimeter ${className}`.trim()} aria-labelledby="admission-perimeter-title">
      <h3 id="admission-perimeter-title">Admission perimeter</h3>
      <div className="card-body">
        {denied ? (
          <p className="tone-warn admission-perimeter-denied">{summary.permissionNote}</p>
        ) : candidates.length === 0 ? (
          <p className="muted">
            Workloads expect pods but none were found. No namespace-scoped mutating or validating webhooks matched this namespace.
          </p>
        ) : (
          <ul className="admission-perimeter-list">
            {candidates.map((c) => (
              <li key={`${c.configKind}/${c.configName}/${c.webhookName}`}>
                <div className="admission-perimeter-head">
                  <strong>{c.webhookName}</strong>
                  <span className="muted mono">{c.configKind}/{c.configName}</span>
                </div>
                <div className="muted admission-perimeter-scope">
                  Namespace scope: {c.namespaceScopeLabel || 'All namespaces'}
                </div>
                {(c.failurePolicy || c.timeoutSeconds > 0) && (
                  <div className="muted">
                    {c.failurePolicy && `Failure policy: ${c.failurePolicy}`}
                    {c.timeoutSeconds > 0 && ` · Timeout: ${c.timeoutSeconds}s`}
                    {c.serviceRef && ` · ${c.serviceRef}`}
                  </div>
                )}
                {c.matchSummary && <div className="muted">{c.matchSummary}</div>}
                {(c.recentSignals || []).length > 0 && (
                  <ul className="admission-perimeter-signals">
                    {c.recentSignals.map((s, i) => (
                      <li key={i}>
                        <span className="mono muted">{s.reason || s.outcome}</span>
                        {' '}
                        {s.message}
                      </li>
                    ))}
                  </ul>
                )}
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  )
}
