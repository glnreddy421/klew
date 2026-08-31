import { TERMINAL_APPEARANCES } from '../lib/terminalAppearance'

export function TerminalAppearancePicker({ value, onChange }) {
  return (
    <div className="terminal-appearance-picker" role="radiogroup" aria-label="Terminal appearance">
      {TERMINAL_APPEARANCES.map((preset) => {
        const active = value === preset.id
        return (
          <button
            key={preset.id}
            type="button"
            role="radio"
            aria-checked={active}
            className={`terminal-appearance-card ${active ? 'active' : ''}`}
            onClick={() => onChange?.(preset.id)}
          >
            <span className="terminal-appearance-swatch" aria-hidden="true">
              <span style={{ background: preset.swatch[0] }} />
              <span style={{ background: preset.swatch[1] }} />
            </span>
            <span className="terminal-appearance-copy">
              <strong>{preset.label}</strong>
              <span className="muted">{preset.description}</span>
            </span>
            {active && <span className="terminal-appearance-check" aria-hidden="true">✓</span>}
          </button>
        )
      })}
    </div>
  )
}
