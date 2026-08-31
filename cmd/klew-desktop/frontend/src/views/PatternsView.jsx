import { useMemo, useState, useEffect } from 'react'
import { getState } from '../lib/investigationViews'
import { StreamPatternsPanel } from '../components/StreamPatternsPanel'
import { EvidenceBoardTeaser } from '../components/evidence/EvidenceBoardPanel'
import { TimelineView } from './TimelineView'
import {
  applyPatternFilterToPayload,
  patternFilterDescription,
  resolvePatternTabKind,
} from '../lib/patternFilters'

const KINDS = [
  { id: 'logs', label: 'Log Patterns', hint: 'Templates and top words from container logs' },
  { id: 'events', label: 'Event Patterns', hint: 'Templates from Pod, Node, and PVC events' },
]

/**
 * Dedicated Patterns page — Log Patterns | Event Patterns.
 */
export function PatternsView({
  view,
  running = false,
  onFilterLogs,
  onOpenEvidence,
  explorerFilter,
  onExplorerFilterChange,
}) {
  const [kind, setKind] = useState(() => resolvePatternTabKind(explorerFilter))

  useEffect(() => {
    setKind(resolvePatternTabKind(explorerFilter))
  }, [explorerFilter?.kind, explorerFilter?.signal])

  const state = getState(view)
  const patterns = view?.logPatterns || state.logPatterns || null
  const evidence = view?.evidence || state.liveEvidence || []

  const filteredPatterns = useMemo(
    () => applyPatternFilterToPayload(patterns, explorerFilter, kind),
    [patterns, explorerFilter, kind],
  )

  const filterLabel = patternFilterDescription(explorerFilter, kind)
  const rawTemplates = kind === 'events'
    ? (patterns?.eventTemplates || [])
    : (patterns?.templates || [])
  const shownTemplates = kind === 'events'
    ? (filteredPatterns?.eventTemplates || [])
    : (filteredPatterns?.templates || [])
  const filterActive = Boolean(
    filterLabel
    || explorerFilter?.kind === 'recurring'
    || explorerFilter?.kind === 'emerging'
    || explorerFilter?.signal,
  )
  const filterEmpty = filterActive && rawTemplates.length > 0 && shownTemplates.length === 0

  function selectKind(nextKind) {
    setKind(nextKind)
    onExplorerFilterChange?.({
      kind: nextKind === 'events' ? 'events' : 'logs',
      signal: explorerFilter?.signal || null,
    })
  }

  function clearExplorerFilter() {
    onExplorerFilterChange?.({ kind: null, signal: null })
  }

  return (
    <div className="inv-page patterns-page">
      <div className="patterns-kind-tabs" role="tablist" aria-label="Pattern kind">
        {KINDS.map((k) => (
          <button
            key={k.id}
            type="button"
            role="tab"
            aria-selected={kind === k.id}
            className={`patterns-kind-pill ${kind === k.id ? 'active' : ''}`}
            title={k.hint}
            onClick={() => selectKind(k.id)}
          >
            {k.label}
          </button>
        ))}
      </div>

      {filterLabel && (
        <div className="patterns-filter-banner">
          <span className="patterns-filter-label">
            Explorer filter: <strong>{filterLabel}</strong>
            {shownTemplates.length > 0 && (
              <span className="muted"> · {shownTemplates.length} shown</span>
            )}
          </span>
          <button type="button" className="text-link-btn" onClick={clearExplorerFilter}>
            Clear filter
          </button>
        </div>
      )}

      <EvidenceBoardTeaser board={patterns?.evidenceBoard} onOpen={onOpenEvidence} />
      <p className="patterns-kind-hint muted">
        {kind === 'logs'
          ? 'Templates from container logs. Click a pattern to expand samples; click a word or field to filter Live tail.'
          : 'Templates from Pod, Node, and PVC Kubernetes events.'}
      </p>

      <div className="patterns-kind-body">
        {filterEmpty ? (
          <div className="empty-stream patterns-empty">
            <strong>No patterns match this filter</strong>
            <span>
              {explorerFilter?.kind === 'emerging'
                ? 'No rising patterns (↑) in the current window. Emerging patterns have increasing volume in recent minutes.'
                : explorerFilter?.kind === 'recurring'
                  ? 'No patterns with 3 or more occurrences in the current window.'
                  : 'Try another filter or clear the explorer selection.'}
            </span>
            <button type="button" className="btn btn-outline btn-sm" onClick={clearExplorerFilter}>
              Clear filter
            </button>
          </div>
        ) : kind === 'logs' ? (
          <StreamPatternsPanel
            patterns={filteredPatterns}
            evidence={evidence}
            running={running}
            selectedPods={[]}
            onFilterLogs={onFilterLogs}
          />
        ) : (
          <TimelineView view={view} embedded filteredPatterns={filteredPatterns} />
        )}
      </div>
    </div>
  )
}
