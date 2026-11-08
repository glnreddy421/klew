import { SURFACE_META } from '../lib/constants'
import { investigationBreadcrumb } from '../lib/investigationContext'
import { HeaderSlot } from '../context/HeaderSlotContext.jsx'

export function WorkspaceChrome({
  tab,
  cluster,
  running,
  view,
  inspectKey,
  inspectRow,
  layoutSwitcher,
  timeWindowLabel,
  live,
  compact = false,
}) {
  const meta = SURFACE_META[tab] || SURFACE_META.incident
  const crumb = investigationBreadcrumb({
    cluster,
    inspectKey,
    inspectRow,
  })

  if (compact) {
    // Shares the Watch / Investigate bar's row when that bar is shown.
    return (
      <HeaderSlot name="context">
      <header className="workspace-chrome workspace-chrome-compact">
        <div className="workspace-chrome-main">
          {crumb && (
            <div className="workspace-context-crumb" title={crumb.full}>
              <span className="crumb-ns mono">{crumb.namespace}</span>
              {crumb.segments.map((seg) => (
                <span key={seg} className="crumb-segment">
                  <span className="crumb-sep" aria-hidden="true">›</span>
                  <span>{seg}</span>
                </span>
              ))}
            </div>
          )}
          {!crumb && meta.subtitle && (
            <p className="workspace-surface-sub muted">{meta.subtitle}</p>
          )}
        </div>
        <div className="workspace-chrome-actions">
          {layoutSwitcher}
        </div>
      </header>
      </HeaderSlot>
    )
  }

  return (
    <header className="workspace-chrome">
      <div className="workspace-chrome-main">
        <div className="workspace-chrome-title">
          <h2 className="workspace-surface-title">{meta.title}</h2>
          {meta.subtitle && (
            <p className="workspace-surface-sub muted">{meta.subtitle}</p>
          )}
        </div>
        {crumb && (
          <div className="workspace-context-crumb" title={crumb.full}>
            <span className="crumb-ns mono">{crumb.namespace}</span>
            {crumb.segments.map((seg) => (
              <span key={seg} className="crumb-segment">
                <span className="crumb-sep" aria-hidden="true">›</span>
                <span>{seg}</span>
              </span>
            ))}
          </div>
        )}
      </div>
      <div className="workspace-chrome-actions">
        {running && (
          <div className="workspace-time-context" title="Investigation time window">
            <span className="workspace-time-label">{timeWindowLabel || 'Last 15m'}</span>
            {live && <span className="workspace-live-badge">Live</span>}
          </div>
        )}
        {layoutSwitcher}
      </div>
    </header>
  )
}
