import { useEffect, useState } from 'react'
import { GetTerminalShellChoices } from '../../wailsjs/go/main/App'
import { shellLabel } from '../lib/shellLabel'

export function TerminalShellModal({
  open,
  initialChoice = 'system',
  confirmLabel = 'Open terminal',
  onConfirm,
  onCancel,
}) {
  const [choices, setChoices] = useState([])
  const [selected, setSelected] = useState(initialChoice || 'system')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!open) return
    setSelected(initialChoice || 'system')
    setLoading(true)
    GetTerminalShellChoices()
      .then((list) => setChoices(Array.isArray(list) ? list : []))
      .catch(() => setChoices([]))
      .finally(() => setLoading(false))
  }, [open, initialChoice])

  if (!open) return null

  const systemChoice = choices.find((c) => c.id === 'system')

  return (
    <div className="modal-overlay" role="dialog" aria-modal="true" aria-labelledby="terminal-shell-title">
      <div className="modal terminal-shell-modal">
        <header className="scope-picker-header">
          <h2 id="terminal-shell-title">Choose your shell</h2>
          <p className="scope-picker-meta">
            Pick a shell for the in-app terminal.
            {' '}
            Use
            {' '}
            <strong>System default</strong>
            {' '}
            to always follow
            {' '}
            {systemChoice?.path ? shellLabel(systemChoice.path) : '$SHELL'}
            .
          </p>
        </header>

        <div className="scope-picker-body terminal-shell-options">
          {loading && choices.length === 0 && (
            <p className="muted">Loading shells…</p>
          )}
          {choices.map((choice) => {
            const id = choice.id || choice.path
            const subtitle = choice.id === 'system'
              ? shellLabel(choice.path)
              : choice.path
            return (
              <label key={id} className={`terminal-shell-option ${selected === id ? 'is-selected' : ''}`}>
                <input
                  type="radio"
                  name="terminal-shell"
                  value={id}
                  checked={selected === id}
                  onChange={() => setSelected(id)}
                />
                <span className="terminal-shell-option-text">
                  <strong>{choice.label}</strong>
                  <span className="muted">{subtitle}</span>
                </span>
              </label>
            )
          })}
        </div>

        <footer className="scope-picker-actions">
          <button
            type="button"
            className="btn btn-primary"
            disabled={!selected || loading}
            onClick={() => onConfirm?.(selected)}
          >
            {confirmLabel}
          </button>
          <button type="button" className="btn btn-ghost" onClick={onCancel}>
            Cancel
          </button>
        </footer>
      </div>
    </div>
  )
}
