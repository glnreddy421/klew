import { THEME_GROUPS } from '../lib/themes'

export function ThemePicker({ themeId, onChange }) {
  return (
    <div className="theme-picker">
      <p className="theme-picker-lead">
        Choose an accent and shell palette. Applied instantly across the app.
      </p>
      {THEME_GROUPS.map((group) => (
        <section key={group.id} className="theme-group">
          <h4 className="theme-group-label">{group.label}</h4>
          <div className="theme-grid" role="radiogroup" aria-label={`${group.label} color themes`}>
            {group.themes.map((theme) => {
              const selected = theme.id === themeId
              return (
                <button
                  key={theme.id}
                  type="button"
                  role="radio"
                  aria-checked={selected}
                  className={`theme-card ${selected ? 'selected' : ''}`}
                  onClick={() => onChange(theme.id)}
                >
                  <span className="theme-preview" aria-hidden="true">
                    <span className="theme-preview-sidebar" style={{ background: theme.swatch[0] }} />
                    <span
                      className="theme-preview-main"
                      style={{ background: theme.mode === 'light' ? '#ffffff' : undefined }}
                    >
                      <span className="theme-preview-accent" style={{ background: theme.swatch[1] }} />
                      <span className="theme-preview-lines" />
                    </span>
                  </span>
                  <span className="theme-card-body">
                    <span className="theme-name">{theme.name}</span>
                    <span className="theme-desc">{theme.description}</span>
                  </span>
                  {selected && <span className="theme-check" aria-hidden="true">✓</span>}
                </button>
              )
            })}
          </div>
        </section>
      ))}
    </div>
  )
}
