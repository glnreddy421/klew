import { getInvestigationSurface } from '../../lib/investigationSurfaceGuide.js'
import { SurfaceGuideIcon } from './surfaceGuideIcons.jsx'

const SURFACE_ICON_ID = {
  overview: 'incident',
  patterns: 'patterns',
  failures: 'failures',
  evidence: 'evidence',
}

/** Ghost preview of the surface that will replace this empty state. */
function SurfacePreview({ surfaceId }) {
  if (surfaceId === 'patterns') {
    return (
      <div className="surface-guide-preview is-patterns" aria-hidden="true">
        {[72, 48, 61, 34].map((w, i) => (
          <span key={i} className="surface-guide-preview-bar">
            <span className="surface-guide-preview-bar-fill" style={{ width: `${w}%` }} />
            <span className="surface-guide-preview-bar-label" />
          </span>
        ))}
      </div>
    )
  }
  if (surfaceId === 'failures') {
    return (
      <div className="surface-guide-preview is-failures" aria-hidden="true">
        {[1, 2, 3].map((row) => (
          <span key={row} className={`surface-guide-preview-row tone-${row === 1 ? 'crit' : row === 2 ? 'warn' : 'ok'}`}>
            <span className="surface-guide-preview-dot" />
            <span className="surface-guide-preview-lines">
              <span />
              <span />
            </span>
          </span>
        ))}
      </div>
    )
  }
  if (surfaceId === 'evidence') {
    return (
      <div className="surface-guide-preview is-evidence" aria-hidden="true">
        <span className="surface-guide-preview-card" />
        <span className="surface-guide-preview-card is-tall" />
        <span className="surface-guide-preview-card" />
      </div>
    )
  }
  return (
    <div className="surface-guide-preview is-overview" aria-hidden="true">
      <div className="surface-guide-preview-verdict">
        <span className="surface-guide-preview-status" />
        <span className="surface-guide-preview-headline" />
        <span className="surface-guide-preview-meta" />
      </div>
      <div className="surface-guide-preview-findings">
        <span />
        <span />
        <span />
      </div>
    </div>
  )
}

function Journey({ steps }) {
  if (!steps?.length) return null
  return (
    <ol className="surface-guide-journey" aria-label="How to start">
      {steps.map((step, index) => (
        <li key={step.label} className="surface-guide-journey-step">
          {index > 0 && <span className="surface-guide-journey-rail" aria-hidden="true" />}
          <span className="surface-guide-journey-num">{index + 1}</span>
          <span className="surface-guide-journey-copy">
            <strong>{step.label}</strong>
            <span>{step.detail}</span>
          </span>
        </li>
      ))}
    </ol>
  )
}

export function InvestigationSurfaceGuide({
  surfaceId,
  variant = 'page',
  onNavigate,
  className = '',
  compact = false,
}) {
  const surface = getInvestigationSurface(surfaceId)
  if (!surface) return null

  const tone = surface.tone || surfaceId
  const iconId = SURFACE_ICON_ID[surfaceId] || surfaceId

  if (variant === 'explorer') {
    return (
      <div className={`surface-guide surface-guide-explorer tone-${tone} ${className}`.trim()}>
        <div className="surface-guide-explorer-head">
          <span className="surface-guide-icon-badge" aria-hidden="true">
            <SurfaceGuideIcon id={iconId} size={18} />
          </span>
          <div className="surface-guide-explorer-copy">
            <p className="surface-guide-kicker">{surface.title}</p>
            <p className="surface-guide-headline">{surface.headline}</p>
          </div>
        </div>
        <p className="surface-guide-desc">{surface.description}</p>
        <ol className="surface-guide-step-list surface-guide-step-list-compact">
          {(surface.journey || surface.steps).map((step, index) => {
            const text = typeof step === 'string' ? step : `${step.label} — ${step.detail}`
            return (
              <li key={text} className="surface-guide-step-item">
                <span className="surface-guide-step-num">{index + 1}</span>
                <span className="surface-guide-step-text">{text}</span>
              </li>
            )
          })}
        </ol>
        {surface.whenLive && (
          <p className="surface-guide-live-hint">
            <span className="surface-guide-live-dot" aria-hidden="true" />
            {surface.whenLive}
          </p>
        )}
      </div>
    )
  }

  const shellClass = [
    'surface-guide',
    `tone-${tone}`,
    compact ? 'is-compact' : 'surface-guide-page',
    className,
  ].filter(Boolean).join(' ')

  return (
    <div className={shellClass}>
      <div className="surface-guide-shell">
        <div className="surface-guide-glow" aria-hidden="true" />

        {!compact && (
          <div className="surface-guide-preview-wrap">
            <SurfacePreview surfaceId={surfaceId} />
            <span className="surface-guide-preview-fade" aria-hidden="true" />
          </div>
        )}

        <header className="surface-guide-hero">
          <span className="surface-guide-icon-badge surface-guide-icon-badge-lg" aria-hidden="true">
            <SurfaceGuideIcon id={iconId} size={22} />
          </span>
          <div className="surface-guide-hero-copy">
            <p className="surface-guide-kicker">{surface.title}</p>
            <h2 className="surface-guide-headline">{surface.headline}</h2>
            <p className="surface-guide-desc">{surface.description}</p>
          </div>
        </header>

        {!compact && <Journey steps={surface.journey} />}

        {!compact && surface.outcomes?.length > 0 && (
          <ul className="surface-guide-outcomes" aria-label="What you will see">
            {surface.outcomes.map((item) => (
              <li key={item.label} className="surface-guide-outcome">
                <strong>{item.label}</strong>
                <span>{item.hint}</span>
              </li>
            ))}
          </ul>
        )}

        <div className="surface-guide-cta">
          <div className="surface-guide-cta-cue">
            <span className="surface-guide-cta-arrow" aria-hidden="true">
              <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">
                <path d="M8 12.5V3.5M4.5 7 8 3.5 11.5 7" />
              </svg>
            </span>
            <div className="surface-guide-cta-copy">
              <strong>{surface.cue || 'Click Investigate in the bar above'}</strong>
              <p>
                Scope is already in the top bar — pick a namespace, then press{' '}
                <span className="surface-guide-inline-chip accent">Investigate</span>.
              </p>
            </div>
          </div>

          <div className="surface-guide-cta-actions">
            {onNavigate && (
              <button type="button" className="btn btn-outline btn-sm" onClick={() => onNavigate('resources')}>
                Browse Resources
              </button>
            )}
            {surface.browseNote && (
              <p className="surface-guide-alt">{surface.browseNote}</p>
            )}
          </div>
        </div>

        {surface.whenLive && (
          <p className="surface-guide-live-hint surface-guide-live-hint-foot">
            <span className="surface-guide-live-dot" aria-hidden="true" />
            {surface.whenLive}
          </p>
        )}
      </div>
    </div>
  )
}
