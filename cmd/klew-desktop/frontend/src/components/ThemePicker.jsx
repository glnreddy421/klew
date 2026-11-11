import { getTheme, THEME_OPTIONS, THEME_SYSTEM } from '../lib/themes'

function PreviewPane({ themeId }) {
  const theme = getTheme(themeId)
  return (
    <span className="theme-preview" aria-hidden="true">
      <span className="theme-preview-sidebar" style={{ background: theme.swatch[0] }} />
      <span className="theme-preview-main" style={{ background: theme.vars['--workspace-bg'] }}>
        <span className="theme-preview-accent" style={{ background: theme.swatch[1] }} />
        <span className="theme-preview-lines" style={{ '--preview-line': theme.vars['--border'] }} />
      </span>
    </span>
  )
}

/** Light / Dark / System as preview cards. Applied instantly on click. */
export function ThemePicker({ themeId, onChange }) {
  return (
    <div className="theme-picker">
      <div className="theme-grid" role="radiogroup" aria-label="Theme">
        {THEME_OPTIONS.map((opt) => {
          const selected = opt.id === themeId
          return (
            <button
              key={opt.id}
              type="button"
              role="radio"
              aria-checked={selected}
              className={`theme-card ${selected ? 'selected' : ''}`}
              onClick={() => onChange(opt.id)}
            >
              <span className="theme-card-preview">
                {opt.id === THEME_SYSTEM ? (
                  <span className="theme-preview-split" aria-hidden="true">
                    <PreviewPane themeId="light" />
                    <PreviewPane themeId="dark" />
                  </span>
                ) : (
                  <PreviewPane themeId={opt.id} />
                )}
                {selected && <span className="theme-check" aria-hidden="true">✓</span>}
              </span>
              <span className="theme-card-body">
                <span className="theme-name">{opt.name}</span>
                <span className="theme-desc">{opt.description}</span>
              </span>
            </button>
          )
        })}
      </div>
    </div>
  )
}
