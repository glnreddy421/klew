import { useState } from 'react'
import { IncidentView, incidentMetaLine } from './IncidentView'
import { PatternsView } from './PatternsView'
import { GraphView } from './GraphView'
import { FailuresView } from './FailuresView'
import { ResourcesView } from './ResourcesView'
import { EvidenceView } from './EvidenceView'
import { SettingsView } from './SettingsView'
import { IncidentLayoutSwitcher } from '../components/incident/IncidentLayoutSwitcher'
import { CollectingMatchesSplash } from '../components/incident/CollectingMatchesSplash'
import { getMatchedObjects } from '../lib/matches'
import { normalizeInvestigationQuery } from '../lib/investigationQuery'
import { loadLayoutMode, saveLayoutMode } from '../lib/incidentLayout'

const META = {
  incident: {
    title: 'Overview',
    subtitle: 'Matched components, status, and investigation focus',
  },
  patterns: {
    title: 'Patterns',
    subtitle: 'Log Patterns and Event Patterns from this investigation',
  },
  graph: { title: 'Graph', subtitle: 'End-to-end ownership and traffic relations' },
  failures: { title: 'Failures', subtitle: 'Failing pods and container runtime' },
  resources: { title: 'Resources', subtitle: 'CPU, memory, nodes, and pressure' },
  evidence: { title: 'Evidence', subtitle: 'Correlated signals, claims, and next checks' },
  settings: { title: 'Settings', subtitle: 'General, appearance, investigation, and Kubernetes' },
}

export function MainContent({
  tab,
  view,
  running,
  starting = false,
  scopePickerOpen = false,
  activeQuery = '',
  cluster,
  themeId,
  onThemeChange,
  onOpenSettings,
  onOpenEvidence,
  prefs,
  onPrefsChange,
  onClusterRefresh,
  focusKey,
  focusPinned,
  drillDown,
  onFocusChange,
  onClearFocus,
  onFilterLogsFromPatterns,
}) {
  const meta = META[tab] || META.incident
  const matchCount = getMatchedObjects(view).length
  const viewQuery = normalizeInvestigationQuery(view?.summary?.query ?? '')
  const expectedQuery = normalizeInvestigationQuery(activeQuery)
  const viewStale = Boolean(
    (running || starting) && expectedQuery && viewQuery && viewQuery !== expectedQuery,
  )
  const [layoutMode, setLayoutMode] = useState(loadLayoutMode)

  const handleLayoutChange = (id) => {
    setLayoutMode(id)
    saveLayoutMode(id)
  }

  let subtitle = meta.subtitle
  if (tab === 'incident' && (running || starting)) {
    subtitle = running && !viewStale && matchCount > 0
      ? incidentMetaLine(view, cluster, matchCount)
      : expectedQuery
        ? `${expectedQuery} · ${cluster?.selectedNamespace || '—'} · scanning…`
        : `${cluster?.selectedNamespace || '—'} · scanning namespace…`
  }

  if (!running && !starting && tab !== 'settings') {
    return <WelcomePanel onOpenSettings={onOpenSettings} />
  }

  const collecting = !scopePickerOpen && (
    starting || viewStale || (running && matchCount === 0)
  )

  return (
    <div className="main-content">
      <div className="content-header">
        <div className="content-header-row">
          <div className="content-header-lead">
            <h1>{meta.title}</h1>
            {subtitle && <p>{subtitle}</p>}
          </div>
          <div className="content-header-actions">
            {tab === 'incident' && focusPinned && drillDown?.active && (
              <div className="focus-chip" title="Overview focused on this workload and related resources">
                <span className="focus-chip-label">
                  Focused · {drillDown.label}
                  {drillDown.relatedPodCount > 0 ? ` · ${drillDown.relatedPodCount} pods` : ''}
                </span>
                <button type="button" className="focus-chip-clear" onClick={onClearFocus}>
                  Clear
                </button>
              </div>
            )}
            {tab === 'incident' && (
              <IncidentLayoutSwitcher value={layoutMode} onChange={handleLayoutChange} />
            )}
          </div>
        </div>
      </div>

      <div className="content-body">
        {tab === 'patterns' && (
          <PatternsView
            view={view}
            running={running}
            onFilterLogs={onFilterLogsFromPatterns}
            onOpenEvidence={onOpenEvidence}
          />
        )}
        {tab === 'incident' && (
          <IncidentView
            view={view}
            focusKey={focusKey}
            focusPinned={focusPinned}
            onFocusChange={onFocusChange}
            onClearFocus={onClearFocus}
            collecting={collecting}
            layoutMode={layoutMode}
          />
        )}
        {tab === 'graph' && <GraphView view={view} />}
        {tab === 'failures' && <FailuresView view={view} />}
        {tab === 'resources' && <ResourcesView view={view} />}
        {tab === 'evidence' && (
          <EvidenceView view={view} onFilterLogs={onFilterLogsFromPatterns} />
        )}
        {tab === 'settings' && (
          <SettingsView
            cluster={cluster}
            themeId={themeId}
            onThemeChange={onThemeChange}
            prefs={prefs}
            onPrefsChange={onPrefsChange}
            onClusterRefresh={onClusterRefresh}
          />
        )}
      </div>
    </div>
  )
}

function WelcomePanel({ onOpenSettings }) {
  return (
    <div className="welcome welcome-orbit">
      <CollectingMatchesSplash variant="idle" />
      <div className="welcome-actions">
        <p className="welcome-sub">
          Leave search empty to browse the whole namespace. Klew streams pods, events, and logs.
        </p>
        <button type="button" className="btn btn-outline" onClick={onOpenSettings}>
          Open Settings
        </button>
      </div>
    </div>
  )
}
