import { useEffect, useState } from 'react'
import { ThemePicker } from '../components/ThemePicker'
import { FontPicker } from '../components/FontPicker'
import { applyUiFont } from '../lib/fonts'
import {
  applyTopbarScale,
  normalizeTopbarScale,
  TOPBAR_SCALE_MAX,
  TOPBAR_SCALE_MIN,
  TOPBAR_SCALE_STEP,
} from '../lib/topbarScale'
import { WorkspaceLayoutPicker } from '../components/incident/WorkspaceLayoutPicker'
import { TerminalShellSelect } from '../components/TerminalShellSelect'
import { TerminalAppearancePicker } from '../components/TerminalAppearancePicker'
import { EventHistorySettings } from '../components/EventHistorySettings.jsx'
import {
  InfoRow,
  NumberRow,
  SettingRow,
  SettingsGroup,
  SettingsMessage,
  SwitchRow,
} from '../components/settings/SettingsControls.jsx'
import { SETTINGS_SECTIONS } from '../lib/preferences'
import {
  OpenKubeconfigDir,
  SetKubeconfigPath,
  GetKubectlInfo,
  SetKubectlOptions,
  GetNetworkProxy,
  SetNetworkProxy,
} from '../../wailsjs/go/main/App'
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime'
import { useAppInfo } from '../hooks/useAppInfo'

const HELP_LINKS = [
  { label: 'Documentation & blog', url: 'https://klew.dev/blog' },
  { label: 'GitHub repository', url: 'https://github.com/glnreddy421/klew' },
  { label: 'Releases & downloads', url: 'https://github.com/glnreddy421/klew/releases' },
]

const KEYBOARD_SHORTCUTS = [
  { keys: '⌘K', action: 'Focus search' },
  { keys: '⌘N', action: 'Open a new window for another cluster' },
  { keys: '⌘R', action: 'Refresh investigation snapshot, or sync kubeconfig when idle' },
  { keys: '⌘C', action: 'Toggle sidebar labels' },
]

/**
 * Settings: section nav + grouped cards of rows (label left, control right).
 */
export function SettingsView({
  cluster,
  themeId,
  onThemeChange,
  prefs,
  onPrefsChange,
  onClusterRefresh,
  onReconnect,
  onDisconnect,
  reconnectBusy = false,
  monitoringPaused = false,
  connection = null,
  onClearDefaultContext,
  section = 'general',
  onSectionChange,
  onTerminalShellChange,
}) {
  const [kubeDraft, setKubeDraft] = useState(prefs.kubeconfigPath || cluster?.kubeconfigPath || '')
  const [kubeBusy, setKubeBusy] = useState(false)
  const [kubeMsg, setKubeMsg] = useState('')
  const [kubectlDraft, setKubectlDraft] = useState(prefs.kubectlPath || '')
  const [kubectlBusy, setKubectlBusy] = useState(false)
  const [kubectlMsg, setKubectlMsg] = useState('')
  const [kubectlInfo, setKubectlInfo] = useState(null)
  const [proxyDraft, setProxyDraft] = useState({
    httpProxy: prefs.httpProxy || '',
    httpsProxy: prefs.httpsProxy || '',
    noProxy: prefs.noProxy || '',
  })
  const [proxyBusy, setProxyBusy] = useState(false)
  const [proxyMsg, setProxyMsg] = useState('')
  const [activeProxy, setActiveProxy] = useState(null)
  const appInfo = useAppInfo()

  useEffect(() => {
    setKubeDraft(prefs.kubeconfigPath || cluster?.kubeconfigPath || '')
  }, [prefs.kubeconfigPath, cluster?.kubeconfigPath])

  useEffect(() => {
    setKubectlDraft(prefs.kubectlPath || '')
  }, [prefs.kubectlPath])

  useEffect(() => {
    if (section !== 'kubernetes') return
    GetKubectlInfo?.().then(setKubectlInfo).catch(() => {})
    GetNetworkProxy?.().then(setActiveProxy).catch(() => {})
  }, [section, cluster?.syncedAt, prefs.useBundledKubectl, prefs.kubectlPath, prefs.matchClusterKubectl, prefs.httpProxy, prefs.httpsProxy, prefs.noProxy])

  useEffect(() => {
    setProxyDraft({
      httpProxy: prefs.httpProxy || '',
      httpsProxy: prefs.httpsProxy || '',
      noProxy: prefs.noProxy || '',
    })
  }, [prefs.httpProxy, prefs.httpsProxy, prefs.noProxy])

  const setSection = (id) => onSectionChange?.(id)

  const set = (patch) => onPrefsChange?.(patch)

  const applyProxy = async () => {
    setProxyBusy(true)
    setProxyMsg('')
    try {
      const httpProxy = proxyDraft.httpProxy.trim()
      const httpsProxy = proxyDraft.httpsProxy.trim()
      const noProxy = proxyDraft.noProxy.trim()
      set({ httpProxy, httpsProxy, noProxy })
      if (typeof SetNetworkProxy === 'function') {
        const active = await SetNetworkProxy(httpProxy, httpsProxy, noProxy)
        setActiveProxy(active)
      }
      setProxyMsg('Proxy settings applied.')
      onClusterRefresh?.()
    } catch (err) {
      setProxyMsg(err?.message || String(err) || 'Failed to apply proxy settings')
    } finally {
      setProxyBusy(false)
    }
  }

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

  const applyKubectl = async (patch = {}) => {
    const useBundled = patch.useBundledKubectl ?? prefs.useBundledKubectl
    const matchCluster = patch.matchClusterKubectl ?? prefs.matchClusterKubectl
    const customPath = (patch.kubectlPath ?? kubectlDraft).trim()
    setKubectlBusy(true)
    setKubectlMsg('')
    try {
      set({
        useBundledKubectl: useBundled,
        matchClusterKubectl: matchCluster,
        kubectlPath: useBundled ? '' : customPath,
      })
      if (typeof SetKubectlOptions === 'function') {
        const info = await SetKubectlOptions(useBundled, useBundled ? '' : customPath, matchCluster)
        setKubectlInfo(info)
      }
      await onClusterRefresh?.()
      if (useBundled) {
        setKubectlMsg('Using bundled kubectl.')
      } else if (customPath) {
        setKubectlMsg('Custom kubectl path applied.')
      } else {
        setKubectlMsg('Using kubectl from system PATH.')
      }
    } catch (err) {
      setKubectlMsg(err?.message || String(err) || 'Failed to apply kubectl settings')
    } finally {
      setKubectlBusy(false)
    }
  }

  const monitoringLabel = monitoringPaused
    ? 'Paused'
    : (connection?.phase === 'connected' ? 'Active' : (connection?.phase || '—'))

  return (
    <div className="settings-shell">
      <aside className="settings-nav" aria-label="Settings sections">
        <div className="settings-nav-search">Settings</div>
        <nav className="settings-nav-list">
          {SETTINGS_SECTIONS.map((s) => (
            <button
              key={s.id}
              type="button"
              className={`settings-nav-item ${section === s.id ? 'active' : ''}`}
              aria-current={section === s.id ? 'page' : undefined}
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
            <SettingsGroup>
              <SwitchRow
                label="Open logs panel when investigating"
                checked={prefs.openStreamOnInvestigate}
                onChange={(v) => set({ openStreamOnInvestigate: v })}
              />
              <SwitchRow
                label="Auto-scroll while gathering logs"
                checked={prefs.followLogsByDefault}
                onChange={(v) => set({ followLogsByDefault: v })}
              />
              <SwitchRow
                label="Keep query text after Stop"
                checked={prefs.rememberLastQuery}
                onChange={(v) => set({ rememberLastQuery: v })}
              />
            </SettingsGroup>

            <SettingsGroup title="Session">
              <SwitchRow
                label="End investigation after inactivity"
                checked={prefs.idleAutoStop}
                onChange={(v) => set({ idleAutoStop: v })}
              />
              <NumberRow
                label="Inactivity limit"
                value={prefs.idleAutoStopMin}
                min={30}
                max={480}
                suffix="min"
                disabled={!prefs.idleAutoStop}
                onChange={(v) => set({ idleAutoStopMin: v })}
              />
            </SettingsGroup>

            <SettingsGroup title="Terminal">
              <SettingRow stacked>
                <TerminalShellSelect
                  value={prefs.terminalShell}
                  onChange={(shell) => {
                    set({ terminalShell: shell, terminalShellPrompted: true })
                    onTerminalShellChange?.(shell)
                  }}
                />
              </SettingRow>
            </SettingsGroup>

            <SettingsGroup title="About">
              <InfoRow label="Version" value={appInfo?.version ? `Klew v${appInfo.version}` : '—'} mono={false} />
              {appInfo?.commit && appInfo.commit !== 'none' && (
                <InfoRow label="Build" value={appInfo.commit} />
              )}
              {appInfo?.commit && appInfo.commit !== 'none' && appInfo.date && appInfo.date !== 'unknown' && (
                <InfoRow label="Built" value={appInfo.date} />
              )}
              <InfoRow label="Install" value="brew tap glnreddy421/klew && brew install --cask klew" />
              <InfoRow label="Upgrade" value="brew update && brew upgrade --cask klew" />
              <SettingRow label="Releases">
                <button
                  type="button"
                  className="btn btn-ghost btn-sm"
                  onClick={() => BrowserOpenURL('https://github.com/glnreddy421/klew/releases')}
                >
                  github.com/glnreddy421/klew/releases
                </button>
              </SettingRow>
            </SettingsGroup>
          </SettingsSection>
        )}

        {section === 'appearance' && (
          <SettingsSection title="Appearance" subtitle="Theme, fonts, workspace layout, and live-tail typography.">
            <SettingsGroup title="Theme">
              <SettingRow stacked>
                <ThemePicker themeId={themeId} onChange={onThemeChange} />
              </SettingRow>
            </SettingsGroup>

            <SettingsGroup title="Font">
              <SettingRow stacked>
                <FontPicker
                  value={prefs.uiFont}
                  onChange={(id) => {
                    set({ uiFont: id })
                    applyUiFont(id).catch(() => {})
                  }}
                />
              </SettingRow>
            </SettingsGroup>

            <SettingsGroup title="Top bar">
              <NumberRow
                label="Size"
                hint="Zoom the header — logo, scope, search, and Investigate (85–130%)."
                value={prefs.topbarScale}
                min={TOPBAR_SCALE_MIN}
                max={TOPBAR_SCALE_MAX}
                step={TOPBAR_SCALE_STEP}
                suffix="%"
                onChange={(v) => {
                  const next = normalizeTopbarScale(v)
                  set({ topbarScale: next })
                  applyTopbarScale(next)
                }}
              />
            </SettingsGroup>

            <SettingsGroup
              title="Workspace layout"
              description="How resources, entities, and signals are arranged on Overview. You can also switch from the header while investigating."
            >
              <SettingRow stacked>
                <WorkspaceLayoutPicker
                  value={prefs.workspaceLayout}
                  onChange={(id) => set({ workspaceLayout: id })}
                />
              </SettingRow>
            </SettingsGroup>

            <SettingsGroup title="Live tail">
              <NumberRow
                label="Font size"
                value={prefs.streamFontSize}
                min={10}
                max={18}
                suffix="px"
                onChange={(v) => set({ streamFontSize: v })}
              />
              <SwitchRow
                label="Dense stream rows"
                checked={prefs.streamDense}
                onChange={(v) => set({ streamDense: v })}
              />
              <SwitchRow
                label="Wrap long log lines"
                checked={prefs.streamWrapLines}
                onChange={(v) => set({ streamWrapLines: v })}
              />
            </SettingsGroup>

            <SettingsGroup
              title="In-app terminal"
              description="Terminal panel colors — pair with a Black theme for a fully dark workspace. Applied instantly."
            >
              <SettingRow stacked>
                <TerminalAppearancePicker
                  value={prefs.terminalAppearance}
                  onChange={(id) => set({ terminalAppearance: id })}
                />
              </SettingRow>
            </SettingsGroup>
          </SettingsSection>
        )}

        {section === 'investigation' && (
          <SettingsSection
            title="Investigation"
            subtitle="Applied on the next Investigate, or immediately when a session is already running."
          >
            <SettingsGroup title="Collection">
              <NumberRow
                label="Log tail lines"
                hint="Per container. 0 follows from now with no history buffer."
                value={prefs.tailLines}
                min={0}
                max={5000}
                onChange={(v) => set({ tailLines: v })}
              />
              <NumberRow
                label="Snapshot refresh"
                value={prefs.refreshSec}
                min={2}
                max={300}
                suffix="sec"
                onChange={(v) => set({ refreshSec: v })}
              />
              <NumberRow
                label="Evidence window"
                hint="Capped at 15 minutes."
                value={prefs.windowMin}
                min={1}
                max={15}
                suffix="min"
                onChange={(v) => set({ windowMin: v })}
              />
              <SwitchRow
                label="Auto-refresh cluster snapshot while investigating"
                checked={prefs.autoRefresh}
                onChange={(v) => set({ autoRefresh: v })}
              />
            </SettingsGroup>
            <EventHistorySettings />
          </SettingsSection>
        )}

        {section === 'concurrency' && (
          <SettingsSection
            title="Concurrency"
            subtitle="Protect the API server when many pods match. Extra containers wait for a free slot."
          >
            <SettingsGroup
              description="Frontend pod filters only hide lines — they do not close kube streams. Lower this value on large namespaces."
            >
              <NumberRow
                label="Max concurrent log follows"
                hint="Default 50. Each follow is one pod/container GetLogs stream."
                value={prefs.maxLogRequests}
                min={1}
                max={200}
                onChange={(v) => set({ maxLogRequests: v })}
              />
            </SettingsGroup>
          </SettingsSection>
        )}

        {section === 'kubernetes' && (
          <SettingsSection title="Kubernetes" subtitle="Cluster access and metrics source.">
            <SettingsGroup title="Kubeconfig" footer={<SettingsMessage>{kubeMsg}</SettingsMessage>}>
              <SettingRow label="Kubeconfig path" stacked>
                <div className="settings-field-row">
                  <input
                    type="text"
                    className="settings-input"
                    value={kubeDraft}
                    placeholder={cluster?.kubeconfigPath || '~/.kube/config'}
                    aria-label="Kubeconfig path"
                    onChange={(e) => setKubeDraft(e.target.value)}
                  />
                  <button type="button" className="btn btn-primary btn-sm" disabled={kubeBusy} onClick={applyKubeconfig}>
                    Apply
                  </button>
                  <button
                    type="button"
                    className="btn btn-ghost btn-sm"
                    onClick={() => OpenKubeconfigDir?.()}
                  >
                    Open folder
                  </button>
                </div>
              </SettingRow>
              <InfoRow label="Active path" value={cluster?.kubeconfigPath} />
              <InfoRow label="Context" value={cluster?.selectedContext} />
              <InfoRow label="Cluster" value={cluster?.cluster} />
              <InfoRow label="User" value={cluster?.user} />
              <InfoRow label="Namespace" value={cluster?.selectedNamespace} />
            </SettingsGroup>

            <SettingsGroup
              title="Contexts"
              description="Star favorites on Home or in the context picker (many allowed). The home icon sets an optional launch default — it does not lock which cluster you connect to."
            >
              <InfoRow
                label="Favorites"
                value={prefs.favoriteContexts?.length ? prefs.favoriteContexts.join(', ') : 'None'}
              />
              <SettingRow label="Launch default" hint={prefs.defaultContext || 'None'}>
                {prefs.defaultContext && (
                  <button
                    type="button"
                    className="btn btn-ghost btn-sm"
                    onClick={() => onClearDefaultContext?.()}
                  >
                    Clear default
                  </button>
                )}
              </SettingRow>
            </SettingsGroup>

            <SettingsGroup title="Cluster connection">
              <SwitchRow
                label="Automatically reconnect when disconnected"
                hint="When off, Klew will not retry or poll in the background after a connection loss."
                checked={prefs.autoMonitorCluster !== false}
                onChange={(v) => set({ autoMonitorCluster: v })}
              />
              <SettingRow
                label="Monitoring"
                hint={monitoringPaused ? 'Paused — Klew is not polling this cluster.' : monitoringLabel}
              >
                {!monitoringPaused ? (
                  <button
                    type="button"
                    className="btn btn-outline btn-sm"
                    disabled={reconnectBusy || !(cluster?.selectedContext || cluster?.currentContext)}
                    onClick={() => onDisconnect?.()}
                  >
                    Disconnect
                  </button>
                ) : (
                  <button
                    type="button"
                    className="btn btn-primary btn-sm"
                    disabled={reconnectBusy}
                    onClick={() => onReconnect?.()}
                  >
                    {reconnectBusy ? 'Connecting…' : 'Reconnect'}
                  </button>
                )}
              </SettingRow>
            </SettingsGroup>

            <SettingsGroup
              title="Network proxy"
              description={(
                <>
                  Route Kubernetes API traffic through an HTTP(S) proxy (for example corporate VPN gateways).
                  Same variables as kubectl (<span className="mono">HTTP_PROXY</span>, <span className="mono">HTTPS_PROXY</span>, <span className="mono">NO_PROXY</span>).
                </>
              )}
              footer={<SettingsMessage>{proxyMsg}</SettingsMessage>}
            >
              <SettingRow label="HTTP proxy" stacked>
                <input
                  type="text"
                  className="settings-input"
                  value={proxyDraft.httpProxy}
                  placeholder="http://127.0.0.1:8888"
                  aria-label="HTTP proxy"
                  onChange={(e) => setProxyDraft((d) => ({ ...d, httpProxy: e.target.value }))}
                />
              </SettingRow>
              <SettingRow label="HTTPS proxy" stacked>
                <input
                  type="text"
                  className="settings-input"
                  value={proxyDraft.httpsProxy}
                  placeholder="http://127.0.0.1:8888"
                  aria-label="HTTPS proxy"
                  onChange={(e) => setProxyDraft((d) => ({ ...d, httpsProxy: e.target.value }))}
                />
              </SettingRow>
              <SettingRow label="No proxy" stacked>
                <input
                  type="text"
                  className="settings-input"
                  value={proxyDraft.noProxy}
                  placeholder="localhost,127.0.0.1,.cluster.local"
                  aria-label="No proxy"
                  onChange={(e) => setProxyDraft((d) => ({ ...d, noProxy: e.target.value }))}
                />
              </SettingRow>
              <SettingRow
                label="Active"
                hint={[
                  activeProxy?.httpProxy && `HTTP ${activeProxy.httpProxy}`,
                  activeProxy?.httpsProxy && `HTTPS ${activeProxy.httpsProxy}`,
                  activeProxy?.noProxy && `NO_PROXY ${activeProxy.noProxy}`,
                ].filter(Boolean).join(' · ') || 'No proxy'}
              >
                <button type="button" className="btn btn-primary btn-sm" disabled={proxyBusy} onClick={applyProxy}>
                  Apply proxy
                </button>
              </SettingRow>
            </SettingsGroup>

            <SettingsGroup title="kubectl" footer={<SettingsMessage>{kubectlMsg}</SettingsMessage>}>
              <SwitchRow
                label="Use bundled kubectl"
                hint="Recommended."
                checked={prefs.useBundledKubectl}
                disabled={kubectlBusy}
                onChange={(v) => applyKubectl({ useBundledKubectl: v })}
              />
              <SwitchRow
                label="Download kubectl matching cluster version"
                hint="Only when the cluster is more than one minor version away from the bundled kubectl (Kubernetes skew policy)."
                checked={prefs.matchClusterKubectl}
                disabled={!prefs.useBundledKubectl || kubectlBusy}
                onChange={(v) => applyKubectl({ matchClusterKubectl: v })}
              />
              {!prefs.useBundledKubectl && (
                <SettingRow label="Custom kubectl path" hint="Leave empty to use kubectl from your shell PATH." stacked>
                  <div className="settings-field-row">
                    <input
                      type="text"
                      className="settings-input"
                      value={kubectlDraft}
                      placeholder={kubectlInfo?.systemPath || '/opt/homebrew/bin/kubectl'}
                      aria-label="Custom kubectl path"
                      onChange={(e) => setKubectlDraft(e.target.value)}
                    />
                    <button
                      type="button"
                      className="btn btn-primary btn-sm"
                      disabled={kubectlBusy}
                      onClick={() => applyKubectl()}
                    >
                      Apply
                    </button>
                  </div>
                </SettingRow>
              )}
              <InfoRow label="Active kubectl" value={kubectlInfo?.activePath} />
              <InfoRow
                label="Version"
                value={[kubectlInfo?.version, kubectlInfo?.source && `(${kubectlInfo.source})`].filter(Boolean).join(' ')}
              />
              <details className="settings-disclosure">
                <summary>More kubectl details</summary>
                <InfoRow label="Cluster version" value={kubectlInfo?.clusterVersion} />
                <InfoRow label="Skew detected" value={kubectlInfo?.clusterSkewDetected ? 'yes' : 'no'} />
                <InfoRow label="Bundled path" value={kubectlInfo?.bundledPath} />
                <InfoRow label="Cluster-matched path" value={kubectlInfo?.clusterMatchedPath} />
                <InfoRow label="System path" value={kubectlInfo?.systemPath} />
              </details>
            </SettingsGroup>

            <SettingsGroup title="Investigation">
              <SwitchRow
                label="Check admission webhooks when pods are missing"
                hint="Lists cluster-scoped mutating and validating webhook configurations that apply to the investigate namespace. Requires elevated RBAC; without permission KLEW labels the perimeter as unknown."
                checked={prefs.checkAdmissionPerimeter}
                onChange={(v) => set({ checkAdmissionPerimeter: v })}
              />
            </SettingsGroup>

            <SettingsGroup title="Metrics">
              <SwitchRow
                label="Use metrics-server when available"
                checked={prefs.useMetricsServer}
                onChange={(v) => set({ useMetricsServer: v })}
              />
              <SettingRow
                label="Metrics API group"
                hint="Klew probes the cluster aggregated API (default metrics.k8s.io). Custom endpoints require a metrics-server (or compatible) install in-cluster."
                stacked
              >
                <input
                  type="text"
                  className="settings-input"
                  value={prefs.metricsApiGroup}
                  aria-label="Metrics API group"
                  onChange={(e) => set({ metricsApiGroup: e.target.value })}
                />
              </SettingRow>
            </SettingsGroup>
          </SettingsSection>
        )}

        {section === 'help' && (
          <SettingsSection title="Help" subtitle="Shortcuts and where to learn more.">
            <SettingsGroup title="Keyboard shortcuts">
              {KEYBOARD_SHORTCUTS.map((row) => (
                <SettingRow key={row.keys} label={row.action}>
                  <kbd className="settings-kbd">{row.keys}</kbd>
                </SettingRow>
              ))}
            </SettingsGroup>

            <SettingsGroup title="Links">
              {HELP_LINKS.map((link) => (
                <button
                  key={link.url}
                  type="button"
                  className="settings-row settings-row-link"
                  onClick={() => BrowserOpenURL(link.url)}
                >
                  <span className="settings-row-label">{link.label}</span>
                  <span className="settings-row-chevron" aria-hidden="true">↗</span>
                </button>
              ))}
            </SettingsGroup>

            <SettingsGroup title="Getting started">
              <SettingRow hint="Pick a context and namespace in the top bar, then click Investigate." />
            </SettingsGroup>
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
