import { useState } from 'react'
import { ThemePicker } from '../components/ThemePicker'
import { SETTINGS_SECTIONS } from '../lib/preferences'
import { OpenKubeconfigDir, SetKubeconfigPath } from '../../wailsjs/go/main/App'

/**
 * Docker Desktop–style settings: section nav + form controls.
 */
export function SettingsView({
  cluster,
  themeId,
  onThemeChange,
  prefs,
  onPrefsChange,
  onClusterRefresh,
}) {
  const [section, setSection] = useState('general')
  const [kubeDraft, setKubeDraft] = useState(prefs.kubeconfigPath || cluster?.kubeconfigPath || '')
  const [kubeBusy, setKubeBusy] = useState(false)
  const [kubeMsg, setKubeMsg] = useState('')

  const set = (patch) => onPrefsChange?.(patch)

  const applyKubeconfig = async () => {
    setKubeBusy(true)
    setKubeMsg('')
    try {
      const path = kubeDraft.trim()
      set({ kubeconfigPath: path })
      if (typeof SetKubeconfigPath === 'function') {
        await SetKubeconfigPath(path)
      }
      await onClusterRefresh?.()
      setKubeMsg(path ? 'Kubeconfig path applied.' : 'Using default kubeconfig resolution.')
    } catch (err) {
      setKubeMsg(err?.message || String(err) || 'Failed to apply kubeconfig')
    } finally {
      setKubeBusy(false)
    }
  }

  return (
    <div className="settings-shell">
      <aside className="settings-nav" aria-label="Settings sections">
        <div className="settings-nav-search muted">Preferences</div>
        <nav className="settings-nav-list">
          {SETTINGS_SECTIONS.map((s) => (
            <button
              key={s.id}
              type="button"
              className={`settings-nav-item ${section === s.id ? 'active' : ''}`}
              onClick={() => setSection(s.id)}
            >
              <span className="settings-nav-label">{s.label}</span>
              <span className="settings-nav-hint">{s.hint}</span>
            </button>
          ))}
        </nav>
      </aside>

      <div className="settings-main">
        {section === 'general' && (
          <SettingsSection title="General" subtitle="Defaults when you start an investigation.">
            <Toggle
              label="Open live tail when investigating"
              checked={prefs.openStreamOnInvestigate}
              onChange={(v) => set({ openStreamOnInvestigate: v })}
            />
            <Toggle
              label="Follow (auto-scroll) live tail by default"
              checked={prefs.followLogsByDefault}
              onChange={(v) => set({ followLogsByDefault: v })}
            />
            <Toggle
              label="Keep query text after Stop"
              checked={prefs.rememberLastQuery}
              onChange={(v) => set({ rememberLastQuery: v })}
            />
            <div className="settings-about">
              <h4>About</h4>
              <p className="muted">CLI: <code>brew tap klew-labs/klew &amp;&amp; brew install klew</code></p>
              <p className="muted">Desktop: <code>brew install klew-desktop</code></p>
            </div>
          </SettingsSection>
        )}

        {section === 'appearance' && (
          <SettingsSection title="Appearance" subtitle="Theme and live-tail typography.">
            <h4 className="settings-subhead">Theme</h4>
            <ThemePicker themeId={themeId} onChange={onThemeChange} />
            <h4 className="settings-subhead">Live tail</h4>
            <NumberField
              label="Font size (px)"
              value={prefs.streamFontSize}
              min={10}
              max={18}
              onChange={(v) => set({ streamFontSize: v })}
            />
            <Toggle
              label="Dense stream rows"
              checked={prefs.streamDense}
              onChange={(v) => set({ streamDense: v })}
            />
            <Toggle
              label="Wrap long log lines"
              checked={prefs.streamWrapLines}
              onChange={(v) => set({ streamWrapLines: v })}
            />
          </SettingsSection>
        )}

        {section === 'investigation' && (
          <SettingsSection
            title="Investigation"
            subtitle="Applied the next time you click Investigate. Active sessions keep their current values."
          >
            <NumberField
              label="Log tail lines (per container)"
              value={prefs.tailLines}
              min={0}
              max={5000}
              hint="0 follows from now with no history buffer"
              onChange={(v) => set({ tailLines: v })}
            />
            <NumberField
              label="Snapshot refresh (seconds)"
              value={prefs.refreshSec}
              min={2}
              max={300}
              onChange={(v) => set({ refreshSec: v })}
            />
            <NumberField
              label="Evidence window (minutes)"
              value={prefs.windowMin}
              min={1}
              max={15}
              hint="Capped at 15 minutes"
              onChange={(v) => set({ windowMin: v })}
            />
            <Toggle
              label="Auto-refresh cluster snapshot while investigating"
              checked={prefs.autoRefresh}
              onChange={(v) => set({ autoRefresh: v })}
            />
          </SettingsSection>
        )}

        {section === 'concurrency' && (
          <SettingsSection
            title="Concurrency"
            subtitle="Protect the API server when many pods match. Extra containers wait for a free slot."
          >
            <NumberField
              label="Max concurrent log follows"
              value={prefs.maxLogRequests}
              min={1}
              max={200}
              hint="Default 50. Each follow is one pod/container GetLogs stream."
              onChange={(v) => set({ maxLogRequests: v })}
            />
            <p className="settings-note muted">
              Frontend pod filters only hide lines — they do not close kube streams.
              Lower this value on large namespaces.
            </p>
          </SettingsSection>
        )}

        {section === 'kubernetes' && (
          <SettingsSection title="Kubernetes" subtitle="Cluster access and metrics source.">
            <label className="settings-field">
              <span className="settings-field-label">Kubeconfig path</span>
              <div className="settings-field-row">
                <input
                  type="text"
                  className="settings-input"
                  value={kubeDraft}
                  placeholder={cluster?.kubeconfigPath || '~/.kube/config'}
                  onChange={(e) => setKubeDraft(e.target.value)}
                />
                <button type="button" className="btn btn-outline btn-sm" disabled={kubeBusy} onClick={applyKubeconfig}>
                  Apply
                </button>
                <button
                  type="button"
                  className="btn btn-outline btn-sm"
                  onClick={() => OpenKubeconfigDir?.()}
                >
                  Open folder
                </button>
              </div>
              {kubeMsg && <span className="settings-field-hint">{kubeMsg}</span>}
            </label>

            <div className="settings-readonly">
              <ReadOnly k="Active path" v={cluster?.kubeconfigPath || '—'} />
              <ReadOnly k="Context" v={cluster?.selectedContext || '—'} />
              <ReadOnly k="Cluster" v={cluster?.cluster || '—'} />
              <ReadOnly k="User" v={cluster?.user || '—'} />
              <ReadOnly k="Namespace" v={cluster?.selectedNamespace || '—'} />
            </div>

            <Toggle
              label="Search all namespaces (advanced)"
              checked={prefs.allNamespaces}
              onChange={(v) => set({ allNamespaces: v })}
            />

            <h4 className="settings-subhead">Metrics</h4>
            <Toggle
              label="Use metrics-server when available"
              checked={prefs.useMetricsServer}
              onChange={(v) => set({ useMetricsServer: v })}
            />
            <label className="settings-field">
              <span className="settings-field-label">Metrics API group</span>
              <input
                type="text"
                className="settings-input"
                value={prefs.metricsApiGroup}
                onChange={(e) => set({ metricsApiGroup: e.target.value })}
              />
              <span className="settings-field-hint">
                Klew probes the cluster aggregated API (default metrics.k8s.io).
                Custom endpoints require a metrics-server (or compatible) install in-cluster.
              </span>
            </label>
          </SettingsSection>
        )}
      </div>
    </div>
  )
}

function SettingsSection({ title, subtitle, children }) {
  return (
    <section className="settings-section">
      <header className="settings-section-head">
        <h2>{title}</h2>
        {subtitle && <p className="muted">{subtitle}</p>}
      </header>
      <div className="settings-section-body">{children}</div>
    </section>
  )
}

function Toggle({ label, checked, onChange }) {
  return (
    <label className="settings-toggle">
      <input
        type="checkbox"
        checked={Boolean(checked)}
        onChange={(e) => onChange?.(e.target.checked)}
      />
      <span>{label}</span>
    </label>
  )
}

function NumberField({ label, value, min, max, hint, onChange }) {
  return (
    <label className="settings-field">
      <span className="settings-field-label">{label}</span>
      <input
        type="number"
        className="settings-input settings-input-num"
        value={value}
        min={min}
        max={max}
        onChange={(e) => onChange?.(Number(e.target.value))}
      />
      {hint && <span className="settings-field-hint">{hint}</span>}
    </label>
  )
}

function ReadOnly({ k, v }) {
  return (
    <div className="kv-row">
      <span className="kv-k">{k}</span>
      <span className="kv-v mono" title={v}>{v}</span>
    </div>
  )
}
