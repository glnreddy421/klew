import { SURFACE_META } from '../lib/constants'
import { investigationBreadcrumb } from '../lib/investigationContext'

export function WorkspaceChrome({
  tab,
  cluster,
  running,
  view,
  inspectKey,
  inspectRow,
  focusPinned,
  drillDown,
  onClearFocus,
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
    focusPinned,
    drillDown,
  })

  if (compact) {
    return (
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
          {focusPinned && drillDown?.active && (
            <div className="focus-chip focus-chip-compact">
              <span className="focus-chip-label">Focus · {drillDown.label}</span>
              <button type="button" className="focus-chip-clear" onClick={onClearFocus}>
                Clear
              </button>
            </div>
          )}
          {layoutSwitcher}
        </div>
      </header>
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
        {focusPinned && drillDown?.active && (
          <div className="focus-chip focus-chip-compact">
            <span className="focus-chip-label">Focus · {drillDown.label}</span>
            <button type="button" className="focus-chip-clear" onClick={onClearFocus}>
              Clear
            </button>
          </div>
        )}
        {layoutSwitcher}
      </div>
    </header>
  )
}
