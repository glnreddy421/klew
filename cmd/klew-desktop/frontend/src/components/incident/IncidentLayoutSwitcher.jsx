import { LAYOUT_MODES } from '../../lib/incidentLayout'

export function IncidentLayoutSwitcher({ value, onChange }) {
  return (
    <div
      className="incident-layout-switcher"
      role="radiogroup"
      aria-label="Incident panel layout"
    >
      {LAYOUT_MODES.map((m) => {
        const active = value === m.id
        return (
          <button
            key={m.id}
            type="button"
            role="radio"
            aria-checked={active}
            className={`incident-layout-pill ${active ? 'active' : ''}`}
            onClick={() => onChange?.(m.id)}
            title={`${m.letter} · ${m.label}`}
          >
            <span className="layout-pill-letter">{m.letter}</span>
            <span className="layout-pill-label">{m.label}</span>
          </button>
        )
      })}
    </div>
  )
}
