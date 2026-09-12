import { useEffect } from 'react'
import { preloadUiFontPreviews, UI_FONT_PRESETS } from '../lib/fonts'

export function FontPicker({ value, onChange }) {
  useEffect(() => {
    preloadUiFontPreviews()
  }, [])

  return (
    <div className="font-picker">
      <p className="font-picker-lead muted">
        UI text and numeric columns. Applied instantly across the app.
      </p>
      <div className="font-picker-grid" role="radiogroup" aria-label="UI font">
        {UI_FONT_PRESETS.map((preset) => {
          const active = value === preset.id
          return (
            <button
              key={preset.id}
              type="button"
              role="radio"
              aria-checked={active}
              className={`font-picker-card ${active ? 'selected' : ''}`}
              onClick={() => onChange?.(preset.id)}
            >
              <span
                className="font-picker-preview"
                style={{ fontFamily: preset.sans }}
                aria-hidden="true"
              >
                <span className="font-picker-preview-title">{preset.name}</span>
                <span
                  className="font-picker-preview-mono"
                  style={{ fontFamily: preset.mono }}
                >
                  {preset.sample}
                </span>
              </span>
              <span className="font-picker-copy">
                <span className="font-picker-name">{preset.name}</span>
                <span className="font-picker-desc">{preset.description}</span>
              </span>
              {active && <span className="font-picker-check" aria-hidden="true">✓</span>}
            </button>
          )
        })}
      </div>
    </div>
  )
}
