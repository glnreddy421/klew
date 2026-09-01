import { useMemo } from 'react'
import {
  ResourcesExplorer,
  ResourcesCollapsedExplorer,
  FailuresExplorer,
  PatternsExplorer,
  EvidenceExplorer,
  GraphExplorer,
  OverviewExplorer,
} from './explorers/ExplorerPanels.jsx'
import { SURFACE_META } from '../../lib/constants.js'

const EXPLORER_TITLES = {
  incident: 'Investigation',
  resources: 'Resources',
  patterns: 'Patterns',
  failures: 'Failures',
  evidence: 'Evidence',
  graph: 'Graph',
}

export function ContextExplorer({
  tab,
  collapsed,
  onToggleCollapse,
  view,
  cluster,
  activeQuery,
  timeWindowLabel,
  live,
  prefs,
  onPrefsChange,
  explorerFilters,
  onExplorerFiltersChange,
  graphRelations,
  onGraphRelationsChange,
  inspectRow,
  width,
}) {
  const title = EXPLORER_TITLES[tab] || SURFACE_META[tab]?.title || 'Explorer'
  const focusLabel = inspectRow ? `${inspectRow.kind}/${inspectRow.name}` : activeQuery

  const content = useMemo(() => {
    switch (tab) {
      case 'resources':
        return <ResourcesExplorer />
      case 'failures':
        return (
          <FailuresExplorer
            view={view}
            filter={explorerFilters.failures}
            onFilterChange={(f) => onExplorerFiltersChange?.({ failures: { ...explorerFilters.failures, ...f } })}
          />
        )
      case 'patterns':
        return (
          <PatternsExplorer
            view={view}
            filter={explorerFilters.patterns}
            onFilterChange={(f) => onExplorerFiltersChange?.({ patterns: { ...explorerFilters.patterns, ...f } })}
          />
        )
      case 'evidence':
        return (
          <EvidenceExplorer
            view={view}
            filter={explorerFilters.evidence}
            onFilterChange={(f) => onExplorerFiltersChange?.({ evidence: { ...explorerFilters.evidence, ...f } })}
          />
        )
      case 'graph':
        return (
          <GraphExplorer
            focusLabel={focusLabel}
            relations={graphRelations}
            onRelationsChange={onGraphRelationsChange}
          />
        )
      case 'incident':
        return (
          <OverviewExplorer
            timeWindowLabel={timeWindowLabel}
            live={live}
            activeQuery={activeQuery}
          />
        )
      default:
        return null
    }
  }, [
    tab, view, explorerFilters, onExplorerFiltersChange, prefs, onPrefsChange,
    graphRelations, onGraphRelationsChange, focusLabel, timeWindowLabel, live, activeQuery,
  ])

  if (tab === 'settings') return null

  if (collapsed) {
    if (tab === 'resources') {
      return (
        <ResourcesCollapsedExplorer onToggleCollapse={onToggleCollapse} />
      )
    }
    return (
      <div className="context-explorer context-explorer-collapsed">
        <button
          type="button"
          className="explorer-expand-btn"
          onClick={onToggleCollapse}
          title={`Expand ${title}`}
          aria-label={`Expand ${title} explorer`}
        >
          <ChevronRight />
        </button>
      </div>
    )
  }

  return (
    <aside
      className="context-explorer"
      style={{ width: `${width}px`, minWidth: `${width}px` }}
      aria-label={`${title} explorer`}
    >
      <header className="explorer-header">
        <h2 className="explorer-header-title">{title}</h2>
        <button
          type="button"
          className="explorer-collapse-btn"
          onClick={onToggleCollapse}
          title="Collapse explorer"
          aria-label="Collapse explorer"
        >
          <ChevronLeft />
        </button>
      </header>
      <div className="explorer-scroll">
        {content}
      </div>
    </aside>
  )
}

function ChevronLeft() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M10 4L6 8l4 4" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

function ChevronRight() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M6 4l4 4-4 4" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}
