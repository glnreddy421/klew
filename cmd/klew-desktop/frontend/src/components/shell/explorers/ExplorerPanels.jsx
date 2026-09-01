import { useEffect, useRef, useState } from 'react'
import { ResourceNav } from '../../incident/ResourceNav.jsx'
import { ResourceCategoryIcon } from '../../ResourceCategoryIcon.jsx'
import { useScopeBrowse } from '../../../context/ScopeBrowseContext.jsx'
import {
  derivePatternSignalCounts,
  patternExplorerCounts,
} from '../../../lib/patternFilters.js'

export function ResourcesExplorer() {
  const ctx = useScopeBrowse()
  if (!ctx) {
    return <ExplorerEmpty message="Resources unavailable" />
  }
  const { nav } = ctx
  return (
    <ResourceNav
      categories={nav.categories}
      expandedGroups={nav.expandedGroups}
      selectedGroupId={nav.selectedGroupId}
      selectedKind={nav.selectedKind}
      selectedResourceId={nav.selectedResourceId}
      onToggleGroup={nav.toggleGroup}
      onSelectKind={nav.selectKind}
    />
  )
}

function useResourceCategoryNav() {
  const ctx = useScopeBrowse()
  const categories = ctx?.nav?.categories || []
  const selectedGroupId = ctx?.nav?.selectedGroupId
  const selectKind = ctx?.nav?.selectKind
  const toggleGroup = ctx?.nav?.toggleGroup

  function onCategoryClick(category) {
    if (!selectKind) return
    const kind = category.kinds?.[0]
    if (kind) {
      selectKind(category.id, kind.kind, kind.resourceId)
      return
    }
    toggleGroup?.(category.id)
  }

  return { categories, selectedGroupId, onCategoryClick }
}

/** Quick category icons — pinned at the top of the resources explorer. */
export function ResourcesCategoryStrip({ layout = 'horizontal' }) {
  const { categories, selectedGroupId, onCategoryClick } = useResourceCategoryNav()
  if (!categories.length) return null

  return (
    <nav
      className={`resources-category-strip resources-category-strip-${layout}`}
      aria-label="Resource categories"
    >
      {categories.map((category) => (
        <button
          key={category.id}
          type="button"
          className={[
            'resources-category-strip-item',
            selectedGroupId === category.id ? 'is-active' : '',
          ].filter(Boolean).join(' ')}
          title={category.label}
          aria-label={category.label}
          aria-current={selectedGroupId === category.id ? 'true' : undefined}
          onClick={() => onCategoryClick(category)}
        >
          <ResourceCategoryIcon categoryId={category.id} size={layout === 'vertical' ? 18 : 16} title={category.label} />
          {layout === 'horizontal' && (
            <span className="resources-category-strip-label">{category.label}</span>
          )}
        </button>
      ))}
    </nav>
  )
}

/** Icon-only resource category rail when the explorer is collapsed. */
export function ResourcesCollapsedExplorer({ onToggleCollapse }) {
  return (
    <div className="context-explorer context-explorer-collapsed context-explorer-resources-collapsed">
      <div className="context-explorer-resources-top">
        <button
          type="button"
          className="explorer-expand-btn"
          onClick={onToggleCollapse}
          title="Expand Resources"
          aria-label="Expand Resources explorer"
        >
          <ChevronRightIcon />
        </button>
        <ResourcesCategoryStrip layout="vertical" />
      </div>
    </div>
  )
}

function ChevronRightIcon() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.75" aria-hidden="true">
      <path d="M6 4l4 4-4 4" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

export function FailuresExplorer({ view, filter, onFilterChange }) {
  const stats = deriveFailureStats(view)
  const types = deriveFailureTypes(view)

  return (
    <>
      <ExplorerSection title="Failures">
        <ExplorerFilterRow
          label="All"
          count={stats.all}
          active={filter?.severity === 'all' || !filter?.severity}
          onClick={() => onFilterChange?.({ severity: 'all' })}
        />
        <ExplorerFilterRow
          label="Critical"
          count={stats.critical}
          active={filter?.severity === 'critical'}
          tone="crit"
          onClick={() => onFilterChange?.({ severity: 'critical' })}
        />
        <ExplorerFilterRow
          label="Warning"
          count={stats.warning}
          active={filter?.severity === 'warning'}
          tone="warn"
          onClick={() => onFilterChange?.({ severity: 'warning' })}
        />
        <ExplorerFilterRow
          label="Stable"
          count={stats.stable}
          active={filter?.severity === 'stable'}
          onClick={() => onFilterChange?.({ severity: 'stable' })}
        />
      </ExplorerSection>
      {types.length > 0 && (
        <ExplorerSection title="By type">
          {types.map((t) => (
            <ExplorerFilterRow
              key={t.id}
              label={t.label}
              count={t.count}
              active={filter?.type === t.id}
              onClick={() => onFilterChange?.({ type: t.id, severity: filter?.severity || 'all' })}
            />
          ))}
        </ExplorerSection>
      )}
    </>
  )
}

export function PatternsExplorer({ view, filter, onFilterChange }) {
  const patterns = view?.logPatterns || view?.state?.logPatterns || null
  const logTpl = patterns?.templates || []
  const eventTpl = patterns?.eventTemplates || []
  const counts = patternExplorerCounts(patterns)
  const signals = derivePatternSignalCounts(logTpl, eventTpl)

  return (
    <>
      <ExplorerSection title="Patterns">
        <ExplorerFilterRow
          label="All"
          count={counts.total}
          active={!filter?.kind && !filter?.signal}
          onClick={() => onFilterChange?.({ kind: null, signal: null })}
        />
        <ExplorerFilterRow
          label="Recurring"
          count={counts.recurring}
          active={filter?.kind === 'recurring'}
          onClick={() => onFilterChange?.({ kind: 'recurring', signal: filter?.signal || null })}
        />
        <ExplorerFilterRow
          label="Emerging"
          count={counts.emerging}
          active={filter?.kind === 'emerging'}
          onClick={() => onFilterChange?.({ kind: 'emerging', signal: filter?.signal || null })}
        />
        <ExplorerFilterRow
          label="Log"
          count={counts.logs}
          active={filter?.kind === 'logs'}
          onClick={() => onFilterChange?.({ kind: 'logs', signal: filter?.signal || null })}
        />
        <ExplorerFilterRow
          label="Events"
          count={counts.events}
          active={filter?.kind === 'events'}
          onClick={() => onFilterChange?.({ kind: 'events', signal: filter?.signal || null })}
        />
      </ExplorerSection>
      {signals.length > 0 && (
        <ExplorerSection title="By signal">
          {signals.map((s) => (
            <ExplorerFilterRow
              key={s.id}
              label={s.label}
              count={s.count}
              active={filter?.signal === s.id}
              onClick={() => onFilterChange?.({ kind: filter?.kind || null, signal: s.id })}
            />
          ))}
        </ExplorerSection>
      )}
    </>
  )
}

export function EvidenceExplorer({ view, filter, onFilterChange }) {
  const evidence = view?.evidence || view?.state?.liveEvidence || []
  const counts = countEvidenceTypes(evidence)

  return (
    <>
      <ExplorerSection title="Evidence">
        <ExplorerFilterRow label="All" count={evidence.length} active={!filter?.type} onClick={() => onFilterChange?.({ type: null })} />
        <ExplorerFilterRow label="Logs" count={counts.log} active={filter?.type === 'log'} onClick={() => onFilterChange?.({ type: 'log' })} />
        <ExplorerFilterRow label="Events" count={counts.event} active={filter?.type === 'event'} onClick={() => onFilterChange?.({ type: 'event' })} />
        <ExplorerFilterRow label="Changes" count={counts.change} active={filter?.type === 'change'} onClick={() => onFilterChange?.({ type: 'change' })} />
        <ExplorerFilterRow label="Metrics" count={counts.metric} active={filter?.type === 'metric'} onClick={() => onFilterChange?.({ type: 'metric' })} />
      </ExplorerSection>
    </>
  )
}

export function GraphExplorer({ focusLabel, relations, onRelationsChange }) {
  const rels = relations || defaultRelations()
  return (
    <>
      <ExplorerSection title="Graph">
        <div className="explorer-meta-row">
          <span className="explorer-meta-label">Focus</span>
          <span className="explorer-meta-value mono">{focusLabel || 'Investigation scope'}</span>
        </div>
      </ExplorerSection>
      <ExplorerSection title="Relationships">
        {['owns', 'selects', 'references', 'routesTo'].map((key) => (
          <label key={key} className="explorer-check-row">
            <input
              type="checkbox"
              checked={rels[key] !== false}
              onChange={(e) => onRelationsChange?.({ ...rels, [key]: e.target.checked })}
            />
            <span>{relationLabel(key)}</span>
          </label>
        ))}
      </ExplorerSection>
    </>
  )
}

export function OverviewExplorer({ timeWindowLabel, live, activeQuery }) {
  return (
    <>
      <ExplorerSection title="Investigation">
        <div className="explorer-meta-row">
          <span className="explorer-meta-label">Query</span>
          <span className="explorer-meta-value mono" title={activeQuery || ''}>{activeQuery || '—'}</span>
        </div>
        <div className="explorer-meta-row">
          <span className="explorer-meta-label">Window</span>
          <span className="explorer-meta-value">{live ? 'Live' : 'Paused'} · {timeWindowLabel}</span>
        </div>
      </ExplorerSection>
    </>
  )
}

function ExplorerSection({ title, children }) {
  return (
    <section className="explorer-section">
      <h3 className="explorer-section-title">{title}</h3>
      <div className="explorer-section-body">{children}</div>
    </section>
  )
}

function ExplorerFilterRow({ label, count, active, tone, onClick }) {
  return (
    <button
      type="button"
      className={[
        'explorer-filter-row',
        active ? 'active' : '',
        tone ? `tone-${tone}` : '',
      ].filter(Boolean).join(' ')}
      onClick={onClick}
    >
      <span className="explorer-filter-label">{label}</span>
      {count != null && <span className="explorer-filter-count mono">{count}</span>}
    </button>
  )
}

function ExplorerEmpty({ message }) {
  return <p className="explorer-empty muted">{message}</p>
}

function deriveFailureStats(view) {
  const pods = view?.state?.snapshot?.pods || []
  let critical = 0
  let warning = 0
  let stable = 0
  for (const p of pods) {
    const reason = String(p.containers?.[0]?.lastReason || p.containers?.[0]?.reason || '').toLowerCase()
    const notReady = !p.ready
    if (reason.includes('oom') || reason.includes('crash') || reason.includes('error')) critical += 1
    else if (notReady || (p.restartCount || 0) >= 3) warning += 1
    else stable += 1
  }
  return { all: pods.length, critical, warning, stable }
}

function deriveFailureTypes(view) {
  const pods = view?.state?.snapshot?.pods || []
  const buckets = {
    oom: { id: 'oom', label: 'OOMKilled', count: 0 },
    probe: { id: 'probe', label: 'Probe failures', count: 0 },
    image: { id: 'image', label: 'Image pull', count: 0 },
    crash: { id: 'crash', label: 'Crash loop', count: 0 },
  }
  for (const p of pods) {
    const r = String(p.containers?.[0]?.lastReason || p.containers?.[0]?.reason || '').toLowerCase()
    if (r.includes('oom')) buckets.oom.count += 1
    else if (r.includes('probe') || r.includes('unhealthy')) buckets.probe.count += 1
    else if (r.includes('image')) buckets.image.count += 1
    else if (r.includes('crash') || r.includes('backoff')) buckets.crash.count += 1
  }
  return Object.values(buckets).filter((b) => b.count > 0)
}

function countEvidenceTypes(evidence) {
  const out = { log: 0, event: 0, change: 0, metric: 0 }
  for (const e of evidence) {
    const t = String(e.sourceType || '').toLowerCase()
    if (t === 'log') out.log += 1
    else if (t === 'k8s_event' || t === 'event') out.event += 1
    else if (t === 'object_change' || t === 'change') out.change += 1
    else if (t === 'metric' || t === 'metrics') out.metric += 1
  }
  return out
}

function defaultRelations() {
  return { owns: true, selects: true, references: true, routesTo: true }
}

function relationLabel(key) {
  switch (key) {
    case 'owns': return 'Owns'
    case 'selects': return 'Selects'
    case 'references': return 'References'
    case 'routesTo': return 'Routes'
    default: return key
  }
}

export { defaultRelations }
