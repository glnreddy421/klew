import { useEffect, useState } from 'react'
import { GetTerminalShellChoices } from '../../wailsjs/go/main/App'
import { shellLabel } from '../lib/shellLabel'
import { resolveTerminalShellPref } from '../lib/terminalShell'

function choiceValue(choice) {
  return choice?.id === 'system' ? 'system' : (choice?.id || choice?.path || 'system')
}

/** Settings dropdown for the in-app terminal shell preference. */
export function TerminalShellSelect({ value = '', onChange, disabled = false }) {
  const [choices, setChoices] = useState([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    setLoading(true)
    GetTerminalShellChoices()
      .then((list) => setChoices(Array.isArray(list) ? list : []))
      .catch(() => setChoices([]))
      .finally(() => setLoading(false))
  }, [])

  const selected = value ? value : 'system'
  const systemChoice = choices.find((c) => c.id === 'system')

  return (
    <label className="settings-field">
      <span className="settings-field-label">Terminal shell</span>
      <select
        className="settings-input settings-select"
        value={selected}
        disabled={disabled || loading || choices.length === 0}
        onChange={(e) => onChange?.(resolveTerminalShellPref(e.target.value))}
      >
        {choices.map((choice) => {
          const id = choiceValue(choice)
          const suffix = choice.id === 'system'
            ? shellLabel(choice.path)
            : choice.path
          return (
            <option key={id} value={id}>
              {choice.label}{suffix ? ` (${suffix})` : ''}
            </option>
          )
        })}
      </select>
      <span className="settings-field-hint">
        Used for new in-app terminal sessions.
        {systemChoice?.path ? (
          <> System default is currently {shellLabel(systemChoice.path)}.</>
        ) : null}
        {' '}Restart open tabs or use Restart in the terminal panel to apply immediately.
      </span>
    </label>
  )
}
