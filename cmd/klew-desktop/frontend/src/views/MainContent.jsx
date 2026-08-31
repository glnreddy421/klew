import { useEffect, useMemo, useState } from 'react'
import { OverviewView } from './OverviewView'
import {
  ResourcesWorkbenchRoot,
  ResourcesWorkbenchView,
  ResourcesWorkbenchInspector,
  useResourcesCatalog,
} from './ResourcesWorkbenchView'
import { PatternsView } from './PatternsView'
import { GraphView } from './GraphView'
import { NodesView } from './NodesView'
import { FailuresView } from './FailuresView'
import { EvidenceView } from './EvidenceView'
import { SettingsView } from './SettingsView'
import { TerminalView } from './TerminalView'
import { WorkspaceChrome } from '../components/WorkspaceChrome'
import { CollectingMatchesSplash } from '../components/incident/CollectingMatchesSplash'
import { deriveMatchRows, getMatchedObjects } from '../lib/matches'
import { inspectRowFromKey } from '../lib/investigationContext'
import { loadLayoutMode } from '../lib/incidentLayout'
import { defaultRelations } from '../components/shell/explorers/ExplorerPanels.jsx'

export function MainContent({
  tab,
  view,
  running,
  starting = false,
  scopePickerOpen = false,
  activeQuery = '',
  cluster,
  clusterStatus,
  syncing = false,
  themeId,
  onThemeChange,
  onOpenSettings,
  onNavigate,
  nodesFocus = 'cluster',
  onOpenEvidence,
  prefs,
  onPrefsChange,
  onClusterRefresh,
  onTerminalShellChange,
  onOpenTerminalShellPicker,
  terminalShellRestartToken = 0,
  terminalMounted = false,
  onTerminalMounted,
  settingsSection = 'general',
  onSettingsSectionChange,
  focusKey,
  focusPinned,
  drillDown,
  onFocusChange,
  onClearFocus,
  onFilterLogsFromPatterns,
  inspectKey,
  onInspectKeyChange,
  explorerFilters,
  onExplorerFiltersChange,
  graphRelations,
  onGraphRelationsChange,
  renderShell,
}) {
  const matchCount = getMatchedObjects(view).length
  const allRows = useMemo(() => deriveMatchRows(view, getMatchedObjects(view)), [view])
  const inspectRow = useMemo(
    () => inspectRowFromKey(inspectKey, view, allRows),
    [inspectKey, view, allRows],
  )

  const resourcesCatalog = useResourcesCatalog(view, cluster)

  const [layoutMode, setLayoutMode] = useState(() => loadLayoutMode(prefs))

  useEffect(() => {
    if (prefs?.workspaceLayout) {
      setLayoutMode(loadLayoutMode(prefs))
    }
  }, [prefs?.workspaceLayout])

  const collecting = !scopePickerOpen && (
    starting || (running && matchCount === 0 && tab === 'resources')
  )

  const timeWindowLabel = prefs?.windowMin ? `Last ${prefs.windowMin}m` : 'Last 15m'
  const live = running && prefs?.autoRefresh !== false

  const handleNavigate = (target) => {
    onNavigate?.(target)
  }

  useEffect(() => {
    if (tab === 'terminal') {
      onTerminalMounted?.()
    }
  }, [tab, onTerminalMounted])

  if (!running && !starting && tab !== 'settings' && tab !== 'terminal') {
    const welcome = <WelcomePanel onOpenSettings={onOpenSettings} />
    return renderShell?.({ workspace: welcome, showInspector: false }) ?? welcome
  }

  if (tab === 'settings') {
    const settings = (
      <div className="main-content main-content-settings">
        <SettingsView
          cluster={cluster}
          themeId={themeId}
          onThemeChange={onThemeChange}
          prefs={prefs}
          onPrefsChange={onPrefsChange}
          onClusterRefresh={onClusterRefresh}
          onTerminalShellChange={onTerminalShellChange}
          section={settingsSection}
          onSectionChange={onSettingsSectionChange}
        />
      </div>
    )
    return renderShell?.({ workspace: settings, showInspector: false }) ?? settings
  }

  const workspaceInner = (
    <>
      {tab !== 'terminal' && (
        <WorkspaceChrome
          tab={tab}
          cluster={cluster}
          running={running}
          view={view}
          inspectKey={inspectKey}
          inspectRow={inspectRow}
          focusPinned={focusPinned}
          drillDown={drillDown}
          onClearFocus={onClearFocus}
          timeWindowLabel={timeWindowLabel}
          live={live}
          compact
        />
      )}
      <div className="content-body content-body-workbench">
        {tab === 'incident' && (
          <OverviewView
            view={view}
            cluster={cluster}
            clusterStatus={clusterStatus}
            running={running}
            syncing={syncing}
            collecting={collecting}
            inspectRow={inspectRow}
            onNavigate={handleNavigate}
            onOpenEvidence={onOpenEvidence}
            onOpenSettings={onOpenSettings}
            onInspectKeyChange={onInspectKeyChange}
            timeWindowLabel={timeWindowLabel}
            live={live}
          />
        )}
        {tab === 'resources' && (
          <ResourcesWorkbenchView shellMode />
        )}
        {tab === 'patterns' && (
          <PatternsView
            view={view}
            running={running}
            onFilterLogs={onFilterLogsFromPatterns}
            onOpenEvidence={onOpenEvidence}
            explorerFilter={explorerFilters?.patterns}
            onExplorerFilterChange={(f) => onExplorerFiltersChange?.({
              patterns: { ...explorerFilters?.patterns, ...f },
            })}
          />
        )}
        {tab === 'graph' && (
          <GraphView
            view={view}
            graphRelations={graphRelations || defaultRelations()}
          />
        )}
        {tab === 'nodes' && (
          <NodesView
            view={view}
            clusterStatus={clusterStatus}
            focus={nodesFocus}
          />
        )}
        {tab === 'failures' && (
          <FailuresView
            view={view}
            onOpenEvidence={onOpenEvidence}
            explorerFilter={explorerFilters?.failures}
          />
        )}
        {tab === 'evidence' && (
          <EvidenceView
            view={view}
            onFilterLogs={onFilterLogsFromPatterns}
            explorerFilter={explorerFilters?.evidence}
          />
        )}
      </div>
    </>
  )

  const workspace = (
    <div className="main-content">
      {tab !== 'terminal' && workspaceInner}
      {terminalMounted && (
        <TerminalView
          cluster={cluster}
          shellPref={prefs?.terminalShell}
          appearance={prefs?.terminalAppearance}
          onChangeShell={onOpenTerminalShellPicker}
          shellRestartToken={terminalShellRestartToken}
          hidden={tab !== 'terminal'}
        />
      )}
    </div>
  )

  const shellPayload = {
    workspace,
    showInspector: tab === 'resources',
    inspector: tab === 'resources' ? <ResourcesWorkbenchInspector /> : null,
    resourcesWrap: tab === 'resources' ? {
      view,
      cluster,
      catalog: resourcesCatalog.catalog,
      rows: resourcesCatalog.allRows,
      focusKey,
      focusPinned,
      onFocusChange,
      onClearFocus,
      collecting,
      layoutMode,
      inspectKey,
      onInspectKeyChange,
      shellMode: true,
    } : null,
  }

  if (renderShell) {
    return renderShell(shellPayload)
  }

  return workspace
}

function WelcomePanel({ onOpenSettings }) {
  return (
    <div className="welcome welcome-orbit">
      <CollectingMatchesSplash variant="idle" />
      <div className="welcome-actions">
        <p className="welcome-sub">
          Klew is a Kubernetes investigation engine — connect a cluster and start investigating.
        </p>
        <button type="button" className="btn btn-outline" onClick={onOpenSettings}>
          Open Settings
        </button>
      </div>
    </div>
  )
}
