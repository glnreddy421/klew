import { NAV_ITEMS } from '../lib/constants'

export function Sidebar({ active, onSelect, onSettings, collapsed, onToggle }) {
  return (
    <aside className={`sidebar rail ${collapsed ? 'collapsed' : ''}`}>
      <button
        type="button"
        className="sidebar-toggle"
        onClick={onToggle}
        aria-label={collapsed ? 'Show nav labels' : 'Hide nav labels'}
        title={collapsed ? 'Show labels (⌘C)' : 'Icon-only (⌘C)'}
      >
        <ChevronIcon direction={collapsed ? 'right' : 'left'} />
      </button>

      <div className="sidebar-brand">
        <div className="brand-mark" aria-hidden="true">K</div>
        {!collapsed && <span className="brand-name">Klew</span>}
      </div>

      <nav className="sidebar-nav" aria-label="Primary">
        {NAV_ITEMS.map((item) => {
          const short = item.navLabel || item.label
          return (
            <button
              key={item.id}
              type="button"
              className={`nav-item ${active === item.id ? 'active' : ''}`}
              onClick={() => onSelect(item.id)}
              title={item.hint || item.label}
              aria-current={active === item.id ? 'page' : undefined}
            >
              <NavIcon id={item.id} />
              {!collapsed && <span className="nav-label">{short}</span>}
            </button>
          )
        })}
      </nav>

      <div className="sidebar-footer">
        <button
          type="button"
          className={`nav-item ${active === 'settings' ? 'active' : ''}`}
          onClick={onSettings}
          title="Settings"
          aria-current={active === 'settings' ? 'page' : undefined}
        >
          <NavIcon id="settings" />
          {!collapsed && <span className="nav-label">Settings</span>}
        </button>
        <button
          type="button"
          className="nav-item help-btn"
          onClick={onSettings}
          title="Help & settings"
        >
          <NavIcon id="help" />
          {!collapsed && <span className="nav-label">Help</span>}
        </button>
      </div>
    </aside>
  )
}

function ChevronIcon({ direction }) {
  return (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
      {direction === 'left' ? (
        <path d="M10 3L5 8l5 5" strokeLinecap="round" strokeLinejoin="round" />
      ) : (
        <path d="M6 3l5 5-5 5" strokeLinecap="round" strokeLinejoin="round" />
      )}
    </svg>
  )
}

function NavIcon({ id }) {
  const icons = {
    incident: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
        <path d="M3 10.5L12 3l9 7.5" />
        <path d="M5 10v9a1 1 0 001 1h4v-6h4v6h4a1 1 0 001-1v-9" />
      </svg>
    ),
    patterns: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
        <path d="M3 12h3l2-6 3 12 2-8 2 2h6" />
      </svg>
    ),
    graph: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
        <circle cx="6" cy="18" r="2" />
        <circle cx="18" cy="6" r="2" />
        <circle cx="18" cy="18" r="2" />
        <path d="M8 18h8M18 8v8M7.5 16.5l9-9" />
      </svg>
    ),
    failures: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
        <path d="M12 9v4m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z" />
      </svg>
    ),
    resources: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
        <rect x="3" y="3" width="7" height="7" rx="1" />
        <rect x="14" y="3" width="7" height="7" rx="1" />
        <rect x="3" y="14" width="7" height="7" rx="1" />
        <rect x="14" y="14" width="7" height="7" rx="1" />
      </svg>
    ),
    evidence: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
        <path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z" />
        <path d="M14 2v6h6M16 13H8M16 17H8M10 9H8" />
      </svg>
    ),
    settings: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
        <circle cx="12" cy="12" r="3" />
        <path d="M12 1v2m0 18v2M4.22 4.22l1.42 1.42m12.72 12.72l1.42 1.42M1 12h2m18 0h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42" />
      </svg>
    ),
    help: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">
        <circle cx="12" cy="12" r="9" />
        <path d="M9.1 9a3 3 0 015.8 1c0 2-3 2-3 4" />
        <path d="M12 17h.01" />
      </svg>
    ),
  }
  return <span className="nav-icon">{icons[id]}</span>
}
