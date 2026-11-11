import { HoverTip } from '../HoverTip.jsx'
import { displayContextName } from '../../lib/clusterContexts.js'

/**
 * Chrome-style context tabs in the top bar. The active tab carries the
 * connection status dot; switching restores where you were in that context.
 */
export function ContextTabs({
  tabs = [],
  active = '',
  statusTone = 'muted',
  statusTitle = '',
  locked = false,
  contextAliases = {},
  onSelect,
  onClose,
  children,
}) {
  if (!tabs.length) return children || null
  return (
    <div className="context-tabs" role="tablist" aria-label="Open contexts">
      {tabs.map((name) => {
        const isActive = name === active
        const blocked = locked && !isActive
        const label = displayContextName(contextAliases, name)
        const fullLabel = displayContextName(contextAliases, name, { truncate: false })
        const tipLabel = blocked
          ? 'Context locked'
          : isActive
            ? fullLabel
            : `Switch to ${fullLabel}`
        const tipDetail = blocked
          ? 'Stop the investigation to switch context.'
          : isActive
            ? [
              statusTitle ? `Active · ${statusTitle}` : 'Active context — click again to open Resources.',
              fullLabel !== name ? name : '',
            ].filter(Boolean).join(' · ')
            : [
              'Restores where you left off in this cluster.',
              fullLabel !== name ? name : '',
            ].filter(Boolean).join(' · ')
        return (
          <div
            key={name}
            className={['context-tab', isActive ? 'is-active' : ''].filter(Boolean).join(' ')}
            role="presentation"
          >
            <HoverTip label={tipLabel} detail={tipDetail} side="bottom">
              <button
                type="button"
                role="tab"
                aria-selected={isActive}
                aria-label={fullLabel}
                title={fullLabel !== name ? `${fullLabel} (${name})` : fullLabel}
                className="context-tab-main"
                onClick={() => !blocked && onSelect?.(name)}
                disabled={blocked}
              >
                {isActive && <span className={`topbar-status-dot tone-${statusTone}`} aria-hidden="true" />}
                <span className="context-tab-name mono">{label}</span>
              </button>
            </HoverTip>
            <HoverTip label={`Close ${fullLabel}`} detail="Removes this tab. Recording keeps running if still enabled.">
              <button
                type="button"
                className="context-tab-close"
                onClick={() => onClose?.(name)}
                disabled={locked && isActive}
                aria-label={`Close ${fullLabel}`}
              >
                <svg viewBox="0 0 16 16" width="10" height="10" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" aria-hidden="true">
                  <path d="M4 4l8 8M12 4l-8 8" />
                </svg>
              </button>
            </HoverTip>
          </div>
        )
      })}
      {children}
    </div>
  )
}
