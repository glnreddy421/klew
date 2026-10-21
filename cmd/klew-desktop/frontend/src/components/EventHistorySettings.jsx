import { useEffect, useState } from 'react'
import { useEventHistoryContext } from '../context/EventHistoryContext.jsx'
import {
  MAX_MB_OPTIONS,
  RETENTION_DAY_OPTIONS,
  formatBytes,
  optionsWithValue,
  settingsContextRows,
} from '../lib/eventHistory'
import {
  SelectRow,
  SettingRow,
  SettingsGroup,
  SettingsMessage,
  SwitchRow,
} from './settings/SettingsControls.jsx'

/** Settings → Investigation → Event history. */
export function EventHistorySettings() {
  const history = useEventHistoryContext()
  const [confirmClear, setConfirmClear] = useState(false)

  useEffect(() => {
    history?.loadSettings?.()
    history?.refresh?.()
    // Load once when the section opens.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    if (!confirmClear) return undefined
    const t = setTimeout(() => setConfirmClear(false), 4000)
    return () => clearTimeout(t)
  }, [confirmClear])

  if (!history) return null
  const { settings, busy, saveSettings, clearHistory, statuses } = history
  const rows = settingsContextRows(statuses)
  const disabled = busy || !settings.available
  const off = !settings.enabled

  return (
    <>
      <SettingsGroup
        title="Event history"
        description="Kubernetes keeps Events for about an hour. Klew records them locally while it runs — for the active context and your favorites — so investigations can show what happened earlier. Stored only on this machine, never uploaded. History has gaps whenever Klew was not running."
        footer={<SettingsMessage tone="warn">{settings.error}</SettingsMessage>}
      >
        <SwitchRow
          label="Record Kubernetes events locally"
          checked={settings.enabled}
          disabled={disabled}
          onChange={(v) => saveSettings({ enabled: v })}
        />
        <SelectRow
          label="Keep history for"
          value={settings.retentionDays}
          disabled={disabled || off}
          options={optionsWithValue(RETENTION_DAY_OPTIONS, settings.retentionDays)
            .map((d) => ({ value: d, label: d === 1 ? '1 day' : `${d} days` }))}
          onChange={(v) => saveSettings({ retentionDays: Number(v) })}
        />
        <SelectRow
          label="Size limit"
          hint="Oldest events are removed first when either limit is reached."
          value={settings.maxMB}
          disabled={disabled || off}
          options={optionsWithValue(MAX_MB_OPTIONS, settings.maxMB)
            .map((mb) => ({ value: mb, label: mb >= 1000 ? `${mb / 1000} GB` : `${mb} MB` }))}
          onChange={(v) => saveSettings({ maxMB: Number(v) })}
        />
        <SettingRow label="Stored on disk" hint={formatBytes(settings.sizeBytes)}>
          <button
            type="button"
            className={`btn btn-sm ${confirmClear ? 'btn-danger' : 'btn-outline'}`}
            disabled={disabled}
            onClick={() => {
              if (!confirmClear) {
                setConfirmClear(true)
                return
              }
              setConfirmClear(false)
              clearHistory()
            }}
          >
            {confirmClear ? 'Click again to clear' : 'Clear history'}
          </button>
        </SettingRow>
      </SettingsGroup>

      {rows.length > 0 && (
        <SettingsGroup title="Recorded contexts">
          {rows.map((r) => (
            <SettingRow key={r.context} label={r.context} hint={r.text} />
          ))}
        </SettingsGroup>
      )}
    </>
  )
}
