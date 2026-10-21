/**
 * Settings building blocks: titled groups of rounded cards holding rows
 * (label + hint on the left, control on the right).
 */

export function SettingsGroup({ title, description, children, footer }) {
  return (
    <section className="settings-group">
      {(title || description) && (
        <header className="settings-group-head">
          {title && <h3 className="settings-group-title">{title}</h3>}
          {description && <p className="settings-group-desc">{description}</p>}
        </header>
      )}
      <div className="settings-card">{children}</div>
      {footer && <div className="settings-group-footer">{footer}</div>}
    </section>
  )
}

/** One card row. `stacked` puts the control under the label (wide inputs, pickers). */
export function SettingRow({ label, hint, children, stacked = false, disabled = false, htmlFor }) {
  const cls = ['settings-row', stacked ? 'is-stacked' : '', disabled ? 'is-disabled' : '']
    .filter(Boolean).join(' ')
  const Label = htmlFor ? 'label' : 'div'
  return (
    <div className={cls}>
      {(label || hint) && (
        <Label className="settings-row-text" {...(htmlFor ? { htmlFor } : {})}>
          {label && <span className="settings-row-label">{label}</span>}
          {hint && <span className="settings-row-hint">{hint}</span>}
        </Label>
      )}
      {children != null && <div className="settings-row-control">{children}</div>}
    </div>
  )
}

/** Card row with a switch; the whole row toggles. */
export function SwitchRow({ label, hint, checked, disabled = false, onChange }) {
  return (
    <label className={['settings-row', 'settings-row-switch', disabled ? 'is-disabled' : ''].filter(Boolean).join(' ')}>
      <span className="settings-row-text">
        <span className="settings-row-label">{label}</span>
        {hint && <span className="settings-row-hint">{hint}</span>}
      </span>
      <span className="settings-row-control">
        <input
          type="checkbox"
          role="switch"
          className="settings-switch"
          checked={Boolean(checked)}
          disabled={disabled}
          aria-checked={Boolean(checked)}
          onChange={(e) => onChange?.(e.target.checked)}
        />
      </span>
    </label>
  )
}

export function NumberRow({ label, hint, value, min, max, step, suffix, disabled = false, onChange }) {
  return (
    <SettingRow label={label} hint={hint} disabled={disabled}>
      <span className="settings-num">
        <input
          type="number"
          className="settings-input settings-input-num"
          value={value}
          min={min}
          max={max}
          step={step}
          disabled={disabled}
          aria-label={label}
          onChange={(e) => onChange?.(Number(e.target.value))}
        />
        <span className="settings-num-suffix" aria-hidden={!suffix}>{suffix || ''}</span>
      </span>
    </SettingRow>
  )
}

export function SelectRow({ label, hint, value, options, disabled = false, onChange }) {
  return (
    <SettingRow label={label} hint={hint} disabled={disabled}>
      <select
        className="settings-input settings-select-compact"
        value={value}
        disabled={disabled}
        aria-label={label}
        onChange={(e) => onChange?.(e.target.value)}
      >
        {options.map((o) => (
          <option key={o.value} value={o.value}>{o.label}</option>
        ))}
      </select>
    </SettingRow>
  )
}

/** Read-only key/value row (paths, versions, active values). */
export function InfoRow({ label, value, mono = true }) {
  const v = value == null || value === '' ? '—' : String(value)
  return (
    <div className="settings-row settings-row-info">
      <span className="settings-row-text">
        <span className="settings-row-label">{label}</span>
      </span>
      <span className={['settings-info-value', mono ? 'mono' : ''].filter(Boolean).join(' ')} title={v}>{v}</span>
    </div>
  )
}

/** Inline status message under a group (applied / error). */
export function SettingsMessage({ children, tone = 'muted' }) {
  if (!children) return null
  return <p className={`settings-message tone-${tone}`} role="status">{children}</p>
}
