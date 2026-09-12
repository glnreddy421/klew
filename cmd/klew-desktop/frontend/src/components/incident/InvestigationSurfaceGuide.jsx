import { getInvestigationSurface } from '../../lib/investigationSurfaceGuide.js'
import {
  InvestigateScopeIcon,
  SurfaceGuideCheckIcon,
  SurfaceGuideIcon,
  SurfaceGuidePanelIcon,
} from './surfaceGuideIcons.jsx'

const SURFACE_ICON_ID = {
  overview: 'incident',
  patterns: 'patterns',
  failures: 'failures',
  evidence: 'evidence',
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
          {surface.steps.map((step, index) => (
            <li key={step} className="surface-guide-step-item">
              <span className="surface-guide-step-num">{index + 1}</span>
              <span className="surface-guide-step-text">{step}</span>
            </li>
          ))}
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

        <header className="surface-guide-hero">
          <span className="surface-guide-icon-badge surface-guide-icon-badge-lg" aria-hidden="true">
            <SurfaceGuideIcon id={iconId} size={22} />
          </span>
          <div className="surface-guide-hero-copy">
            <span className="surface-guide-badge">Investigation surface</span>
            <p className="surface-guide-kicker">{surface.title}</p>
            <h2 className="surface-guide-headline">{surface.headline}</h2>
            <p className="surface-guide-desc">{surface.description}</p>
          </div>
        </header>

        <div className="surface-guide-investigate-callout">
          <span className="surface-guide-callout-icon" aria-hidden="true">
            <InvestigateScopeIcon />
          </span>
          <div className="surface-guide-callout-copy">
            <strong>Start from the top bar</strong>
            <p>Set <span className="surface-guide-inline-chip">Investigate scope</span> to one namespace, then click <span className="surface-guide-inline-chip accent">Investigate</span>.</p>
          </div>
        </div>

        {!compact && (
          <div className="surface-guide-panels">
            <section className="surface-guide-panel">
              <h3 className="surface-guide-panel-title">
                <span className="surface-guide-panel-icon" aria-hidden="true">
                  <SurfaceGuidePanelIcon kind="see" />
                </span>
                What you will see
              </h3>
              <ul className="surface-guide-feature-list">
                {surface.provides.map((item) => (
                  <li key={item} className="surface-guide-feature-item">
                    <span className="surface-guide-feature-check" aria-hidden="true">
                      <SurfaceGuideCheckIcon />
                    </span>
                    <span>{item}</span>
                  </li>
                ))}
              </ul>
            </section>

            <section className="surface-guide-panel">
              <h3 className="surface-guide-panel-title">
                <span className="surface-guide-panel-icon" aria-hidden="true">
                  <SurfaceGuidePanelIcon kind="do" />
                </span>
                How to get data here
              </h3>
              <ol className="surface-guide-step-list">
                {surface.steps.map((step, index) => (
                  <li key={step} className="surface-guide-step-item">
                    <span className="surface-guide-step-num">{index + 1}</span>
                    <span className="surface-guide-step-text">{step}</span>
                  </li>
                ))}
              </ol>
            </section>
          </div>
        )}

        <footer className="surface-guide-foot">
          {surface.whenLive && (
            <p className="surface-guide-live-hint">
              <span className="surface-guide-live-dot" aria-hidden="true" />
              {surface.whenLive}
            </p>
          )}

          {onNavigate && (
            <div className="surface-guide-actions">
              <button type="button" className="btn btn-outline btn-sm" onClick={() => onNavigate('resources')}>
                Browse Resources
              </button>
            </div>
          )}

          {surface.browseNote && (
            <p className="surface-guide-alt">{surface.browseNote}</p>
          )}
        </footer>
      </div>
    </div>
  )
}
