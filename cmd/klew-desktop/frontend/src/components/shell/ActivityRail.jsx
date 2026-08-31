import { NAV_ITEMS, NAV_ITEMS_SECONDARY } from '../../lib/constants.js'

export function ActivityRail({ active, onSelect, collapsed = false, onToggleCollapse }) {
  return (
    <aside
      className={`activity-rail ${collapsed ? 'is-collapsed' : ''}`}
      aria-label="Investigation surfaces"
    >
      <div className="activity-rail-inner">
        <nav className="activity-rail-nav" aria-label="Primary surfaces">
          {NAV_ITEMS.map((item) => (
            <ActivityRailItem
              key={item.id}
              item={item}
              active={active === item.id}
              collapsed={collapsed}
              onClick={() => onSelect?.(item.id)}
            />
          ))}
        </nav>

        <div className="activity-rail-divider" aria-hidden="true" />

        <nav className="activity-rail-nav activity-rail-nav-secondary" aria-label="Secondary surfaces">
          {NAV_ITEMS_SECONDARY.map((item) => (
            <ActivityRailItem
              key={item.id}
              item={item}
              active={active === item.id}
              collapsed={collapsed}
              onClick={() => onSelect?.(item.id)}
            />
          ))}
        </nav>
      </div>

      <footer className="activity-rail-footer">
        <button
          type="button"
          className="activity-rail-collapse"
          onClick={onToggleCollapse}
          title={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          aria-label={collapsed ? 'Expand navigation' : 'Collapse navigation'}
          aria-expanded={!collapsed}
        >
          <RailToggleIcon collapsed={collapsed} />
          <span className="activity-rail-collapse-label">Collapse</span>
        </button>
      </footer>
    </aside>
  )
}

function ActivityRailItem({ item, active, collapsed, onClick }) {
  const label = item.navLabel || item.label
  const tip = item.hint || item.label

  return (
    <button
      type="button"
      className={`activity-rail-item ${active ? 'is-active' : ''}`}
      onClick={onClick}
      title={collapsed ? tip : undefined}
      aria-label={item.label}
      aria-current={active ? 'page' : undefined}
    >
      <span className="activity-rail-icon-wrap" aria-hidden="true">
        <ActivityIcon id={item.id} />
      </span>
      <span className="activity-rail-label">{label}</span>
    </button>
  )
}

function RailToggleIcon({ collapsed }) {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      {collapsed ? (
        <path d="M6 4l4 4-4 4" strokeLinecap="round" strokeLinejoin="round" />
      ) : (
        <path d="M10 4L6 8l4 4" strokeLinecap="round" strokeLinejoin="round" />
      )}
    </svg>
  )
}

function ActivityIcon({ id }) {
  const icons = {
    incident: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
        <circle cx="12" cy="12" r="7.5" /><path d="M12 8v4l2.25 2.25" />
      </svg>
    ),
    patterns: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
        <path d="M3 12h2.5l2-5.5 2.75 11 2-7.5 1.75 1.75H21" />
      </svg>
    ),
    failures: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
        <path d="M12 8.5v4.5m0 3.5h.01M10.5 4.2L3.2 17.8a1.8 1.8 0 001.56 2.7h14.48a1.8 1.8 0 001.56-2.7L13.5 4.2a1.8 1.8 0 00-3 0z" />
      </svg>
    ),
    resources: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
        <rect x="3.5" y="3.5" width="6.5" height="6.5" rx="1.25" />
        <rect x="14" y="3.5" width="6.5" height="6.5" rx="1.25" />
        <rect x="3.5" y="14" width="6.5" height="6.5" rx="1.25" />
        <rect x="14" y="14" width="6.5" height="6.5" rx="1.25" />
      </svg>
    ),
    evidence: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
        <path d="M14 3H7a2 2 0 00-2 2v14a2 2 0 002 2h10a2 2 0 002-2V8z" />
        <path d="M14 3v5h5M8 13h8M8 17h6M8 9h2" />
      </svg>
    ),
    graph: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
        <circle cx="6" cy="17" r="2" /><circle cx="18" cy="7" r="2" /><circle cx="18" cy="17" r="2" />
        <path d="M8 17h8M18 9v6M7.75 15.25l8.5-6.5" />
      </svg>
    ),
    terminal: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
        <rect x="3" y="4.5" width="18" height="15" rx="2" />
        <path d="M7.5 10.5L10 13l-2.5 2.5" />
        <path d="M13 15.5h4" />
      </svg>
    ),
  }
  return <span className="activity-rail-icon">{icons[id]}</span>
}
