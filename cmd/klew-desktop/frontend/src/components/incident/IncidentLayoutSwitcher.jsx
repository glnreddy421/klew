import { WORKSPACE_LAYOUTS } from '../../lib/incidentLayout'

export function IncidentLayoutSwitcher({ value, onChange }) {
  return (
    <div
      className="incident-layout-switcher"
      role="radiogroup"
      aria-label="Workspace layout"
    >
      {WORKSPACE_LAYOUTS.map((m) => {
        const active = value === m.id
        return (
          <button
            key={m.id}
            type="button"
            role="radio"
            aria-checked={active}
            className={`incident-layout-pill ${active ? 'active' : ''}`}
            onClick={() => onChange?.(m.id)}
            title={`${m.label} — ${m.hint}`}
          >
            <span className="layout-pill-letter">{m.letter}</span>
            <span className="layout-pill-label">{m.shortLabel}</span>
          </button>
        )
      })}
    </div>
  )
}
