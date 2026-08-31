import { KindIcon } from '../KindIcon.jsx'

export function AffectedResourcePath({
  path,
  onNavigate,
  onResourceSelect,
  compact = false,
}) {
  if (!path?.length) return null

  return (
    <section className={`inv-visual-section inv-path-section ${compact ? 'inv-path-compact' : ''}`}>
      <div className="inv-section-head">
        <h2 className="inv-section-title">Affected path</h2>
        <button type="button" className="text-link-btn" onClick={() => onNavigate?.('graph')}>
          Open Graph →
        </button>
      </div>

      <div className="inv-resource-path">
        {path.map((step, i) => (
          <div key={`${step.kind}-${step.name}-${i}`} className="inv-path-step">
            {i > 0 && step.viaLabel && (
              <span className="inv-path-via muted">{step.viaLabel}</span>
            )}
            <button
              type="button"
              className="inv-path-node"
              onClick={() => onResourceSelect?.({ kind: step.kind, name: step.name, key: `${step.kind}/${step.name}` })}
            >
              <KindIcon kind={step.kind} size={14} />
              <span className="inv-path-kind muted">{step.kind}</span>
              <span className="inv-path-name mono">{step.name}</span>
              {step.badges?.map((b) => (
                <span key={b.text} className={`inv-path-badge tone-${b.tone}`}>
                  {b.tone === 'failure' && '▲ '}
                  {b.text}
                </span>
              ))}
            </button>
          </div>
        ))}
      </div>
    </section>
  )
}

export function AffectedResourcesList({
  resources,
  onNavigate,
  onResourceSelect,
}) {
  if (!resources?.length) return null

  return (
    <section className="inv-visual-section inv-affected-section">
      <div className="inv-section-head">
        <h2 className="inv-section-title">Affected resources</h2>
        <button type="button" className="text-link-btn" onClick={() => onNavigate?.('resources')}>
          Open catalog →
        </button>
      </div>
      <ul className="inv-affected-list">
        {resources.map((res) => (
          <li key={res.key}>
            <button
              type="button"
              className="inv-affected-row"
              onClick={() => onResourceSelect?.(res)}
            >
              <KindIcon kind={res.kind} size={14} />
              <span className="inv-affected-name">{res.kind}/{res.name}</span>
              <span className={`inv-affected-detail ${res.status === 'critical' ? 'tone-crit' : res.status === 'degraded' ? 'tone-warn' : 'muted'}`}>
                {res.signalCount > 0 ? `${res.signalCount} signal${res.signalCount === 1 ? '' : 's'}` : res.detail}
              </span>
            </button>
          </li>
        ))}
      </ul>
    </section>
  )
}
