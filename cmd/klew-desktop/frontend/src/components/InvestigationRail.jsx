import { NAV_ITEMS, NAV_ITEMS_SECONDARY } from '../lib/constants'

export function InvestigationRail({ active, onSelect }) {
  return (
    <aside className="investigation-rail" aria-label="Investigation surfaces">
      <nav className="rail-nav rail-nav-primary">
        {NAV_ITEMS.map((item) => (
          <RailItem
            key={item.id}
            item={item}
            active={active === item.id}
            onSelect={onSelect}
          />
        ))}
      </nav>
      <div className="rail-divider" aria-hidden="true" />
      <nav className="rail-nav rail-nav-secondary">
        {NAV_ITEMS_SECONDARY.map((item) => (
          <RailItem
            key={item.id}
            item={item}
            active={active === item.id}
            onSelect={onSelect}
          />
        ))}
      </nav>
    </aside>
  )
}

function RailItem({ item, active, onSelect }) {
  const short = item.navLabel || item.label
  return (
    <button
      type="button"
      className={`rail-item ${active ? 'active' : ''}`}
      onClick={() => onSelect(item.id)}
      title={item.hint || item.label}
      aria-current={active ? 'page' : undefined}
    >
      <span className="rail-indicator" aria-hidden="true" />
      <RailIcon id={item.id} />
      <span className="rail-label">{short}</span>
    </button>
  )
}

function RailIcon({ id }) {
  const icons = {
    incident: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
        <circle cx="12" cy="12" r="8" />
        <path d="M12 8v4l2.5 2.5" />
      </svg>
    ),
    patterns: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
        <path d="M3 12h3l2-6 3 12 2-8 2 2h6" />
      </svg>
    ),
    failures: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
        <path d="M12 9v4m0 4h.01M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z" />
      </svg>
    ),
    resources: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
        <rect x="3" y="3" width="7" height="7" rx="1" />
        <rect x="14" y="3" width="7" height="7" rx="1" />
        <rect x="3" y="14" width="7" height="7" rx="1" />
        <rect x="14" y="14" width="7" height="7" rx="1" />
      </svg>
    ),
    evidence: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
        <path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z" />
        <path d="M14 2v6h6M16 13H8M16 17H8M10 9H8" />
      </svg>
    ),
    graph: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
        <circle cx="6" cy="18" r="2" />
        <circle cx="18" cy="6" r="2" />
        <circle cx="18" cy="18" r="2" />
        <path d="M8 18h8M18 8v8M7.5 16.5l9-9" />
      </svg>
    ),
  }
  return <span className="rail-icon">{icons[id]}</span>
}
