import { useEffect, useMemo, useRef, useState } from 'react'
import { StatusBadge, RowStatusBadge } from './StatusBadge'
import { KindIcon } from '../KindIcon'
import { formatReady } from '../../lib/matches'
import {
  deriveSignalStats,
  hasAnomalyIssues,
} from '../../lib/incidentLayout'
import { decodeSecretValue } from '../../lib/secretDisplay'
import {
  enrichInspectGroups,
  resolveInspectRef,
  summaryMetrics,
} from '../../lib/inspectEnrich'
import { InspectContainersPanel } from './InspectContainersPanel'
import { RelationshipGraphPanel } from './RelationshipGraph'
import { ServiceEndpointSummary } from './ServiceEndpointSummary'
import { HelmReleaseInspectView } from './HelmReleaseInspectView.jsx'
import { hasContainerPanelData, parseInspectContainers } from '../../lib/inspectContainers'
import { buildRelationshipGraph } from '../../lib/relationshipGraph'
import { findServiceEndpointFields } from '../../lib/serviceEndpoints'

/**
 * Kind-aware object inspector.
 * Prefer live `detail` sections from GetObjectDetails; fall back to snapshot inspect.
 */
export function ComponentInspectPanel({
  inspect,
  emptyHint,
  layoutMode = 'detail-tabs',
  onFocus,
  onInspect,
  focusPinned = false,
  showFocusCta = false,
  loading = false,
  error = null,
}) {
  const groups = useMemo(
    () => enrichInspectGroups(
      inspect?.groups?.length ? inspect.groups : fallbackGroups(inspect),
      inspect,
    ),
    [inspect],
  )

  const metrics = useMemo(() => summaryMetrics(inspect), [inspect])

  const relatedPods = useMemo(() => {
    if (!inspect?.relatedPods?.length || inspect.kind === 'Pod') return []
    return inspect.relatedPods
  }, [inspect])

  const relationshipItems = useMemo(() => {
    if (!inspect) return []
    const items = inspect.relationships || []
    if (!relatedPods.length) return items
    return items.filter((r) => r.kind !== 'Pod' && r.role !== 'Pod' && r.role !== 'Target pod')
  }, [inspect, relatedPods])

  const relationshipCenter = useMemo(() => {
    if (!inspect) return null
    return {
      kind: inspect.kind,
      name: inspect.name,
      key: inspect.key,
      namespace: inspect.namespace,
    }
  }, [inspect])

  const hasRelationshipGraph = useMemo(() => {
    if (!relationshipCenter) return false
    return buildRelationshipGraph({
      center: relationshipCenter,
      items: relationshipItems,
      inspectNamespace: inspect?.namespace,
    }).nodes.length > 1
  }, [relationshipCenter, relationshipItems, inspect?.namespace])

  if (!inspect) {
    return (
      <div className="inspect-empty muted">
        {loading
          ? 'Loading object details…'
          : (emptyHint || 'Select a component to inspect.')}
      </div>
    )
  }

  if (inspect.kind === 'HelmRelease') {
    return (
      <HelmReleaseInspectView
        inspect={inspect}
        loading={loading}
        error={error}
      />
    )
  }

  const unhealthy = hasAnomalyIssues(inspect)
  const focusKey = inspect.key

  const relatedPodsBlock = relatedPods.length > 0 ? (
    <RelatedPodsSection pods={relatedPods} onInspect={onInspect} />
  ) : null

  const relationshipsBlock = hasRelationshipGraph ? (
    <RelationshipGraphPanel
      center={relationshipCenter}
      items={relationshipItems}
      inspectNamespace={inspect.namespace}
      onInspect={onInspect}
    />
  ) : null

  const summaryExtras = (
    <>
      {metrics.length > 0 && (
        <InspectSummaryMetrics metrics={metrics} onInspect={onInspect} inspectNamespace={inspect.namespace} />
      )}
      {relationshipsBlock}
    </>
  )

  const header = (
    <InspectIdentityHeader
      inspect={inspect}
      showFocusCta={showFocusCta && !focusPinned}
      onFocus={() => onFocus?.(focusKey)}
      onInspect={onInspect}
      loading={loading}
      error={error}
    />
  )

  switch (layoutMode) {
    case 'signal-first':
      return (
        <SignalFirstPanel
          inspect={inspect}
          unhealthy={unhealthy}
          header={header}
          summaryExtras={summaryExtras}
          groups={groups}
          relatedPodsBlock={relatedPodsBlock}
          relationshipItems={relationshipItems}
          onInspect={onInspect}
        />
      )

    case 'detail-tabs':
      return (
        <DetailTabsPanel
          inspect={inspect}
          unhealthy={unhealthy}
          header={header}
          summaryExtras={summaryExtras}
          groups={groups}
          relatedPodsBlock={relatedPodsBlock}
          onInspect={onInspect}
        />
      )

    case 'stacked':
    default:
      return (
        <div className={`inspect-panel mode-${layoutMode || 'stacked'}`}>
          {header}
          {summaryExtras}
          <SignalsBlock inspect={inspect} unhealthy={unhealthy} quietHealthy compact={metrics.length > 0} />
          {relatedPodsBlock}
          <StackedSections
            groups={groups}
            onInspect={onInspect}
            inspectNamespace={inspect.namespace}
          />
        </div>
      )
  }
}

function fallbackGroups(inspect) {
  if (!inspect) return []
  const groups = []
  if (inspect.status?.fields?.length) {
    groups.push({
      id: 'status',
      label: 'Status',
      sections: [{
        id: 'status',
        title: 'Status',
        fields: inspect.status.fields.map((f) => ({ key: f.k, value: f.v })),
      }],
    })
  }
  if (inspect.resourceBars?.length) {
    groups.push({
      id: 'runtime',
      label: 'Runtime',
      sections: [{ id: 'resources', title: 'Resources', _resourceBars: inspect.resourceBars }],
    })
  }
  const labels = inspect.meta?.labels || []
  const annotations = inspect.meta?.annotations || []
  if (labels.length || annotations.length) {
    groups.push({
      id: 'metadata',
      label: 'Metadata',
      sections: [{
        id: 'labels',
        title: 'Labels & annotations',
        _labels: labels,
        _annotations: annotations,
      }],
    })
  }
  if (inspect.events?.length) {
    groups.push({
      id: 'events',
      label: 'Events',
      sections: [{ id: 'events', title: 'Events', _events: inspect.events }],
    })
  }
  return groups
}

function InspectIdentityHeader({ inspect, showFocusCta, onFocus, onInspect, loading, error }) {
  const namespaceRef = inspect.namespace
    ? resolveInspectRef(inspect.namespace, {
      fieldKey: 'Namespace',
      inspectNamespace: inspect.namespace,
    })
    : null
  return (
    <header className="inspect-header inspect-header-actions inspect-identity">
      <div className="inspect-title-block">
        <div className="inspect-name-row">
          <KindIcon kind={inspect.kind} size={18} />
          <h4 className="inspect-name">
            <span className="inspect-name-text">{inspect.name}</span>
          </h4>
          {inspect.adhoc && (
            <span className="inspect-adhoc-tag" title="Fetched on demand — not in investigation scope">
              On demand
            </span>
          )}
        </div>
        <p className="inspect-identity-meta muted">
          <span>{inspect.kind}</span>
          {inspect.namespace ? (
            <>
              <span className="inspect-meta-sep">·</span>
              {namespaceRef && onInspect ? (
                <button
                  type="button"
                  className="inspect-field-link mono"
                  onClick={() => onInspect(namespaceRef.key)}
                  title={`Inspect Namespace/${inspect.namespace}`}
                >
                  {inspect.namespace}
                </button>
              ) : (
                <span className="mono">{inspect.namespace}</span>
              )}
            </>
          ) : (
            <>
              <span className="inspect-meta-sep">·</span>
              <span>Cluster-scoped</span>
            </>
          )}
        </p>
      </div>
      <div className="inspect-header-right">
        {loading && <span className="muted inspect-loading">Fetching live details…</span>}
        <StatusBadge status={inspect.status.tone} label={inspect.status.label} />
        {showFocusCta && (
          <button
            type="button"
            className="btn btn-outline inspect-focus-cta"
            onClick={onFocus}
            aria-label={`Focus chain for ${inspect.kind}/${inspect.name}`}
          >
            Focus chain
          </button>
        )}
      </div>
      {error && (
        <div className="inspect-fetch-error" role="alert">
          {error}
        </div>
      )}
    </header>
  )
}

function InspectSummaryMetrics({ metrics, onInspect, inspectNamespace = '' }) {
  if (!metrics?.length) return null
  return (
    <dl className="inspect-prop-list inspect-summary-props" aria-label="Summary">
      {metrics.map((m) => (
        <div key={m.key} className="inspect-prop-row">
          <dt>{m.key}</dt>
          <dd>
            <InspectRefValue
              value={m.value}
              fieldKey={m.key}
              onInspect={onInspect}
              inspectNamespace={inspectNamespace}
            />
          </dd>
        </div>
      ))}
    </dl>
  )
}

function InspectRefValue({
  value,
  fieldKey = '',
  columnName = '',
  section = null,
  groupId = '',
  onInspect,
  inspectNamespace = '',
  row = null,
  columns = null,
  columnIndex = -1,
  className = '',
  mono = false,
}) {
  const text = value ?? '—'
  const ref = onInspect
    ? resolveInspectRef(text, {
      fieldKey,
      columnName,
      section,
      groupId,
      inspectNamespace,
      row,
      columns,
      columnIndex,
    })
    : null

  if (!ref?.key) {
    return (
      <span className={[className, mono ? 'mono' : ''].filter(Boolean).join(' ')} title={text}>
        {text || '—'}
      </span>
    )
  }

  return (
    <button
      type="button"
      className={['inspect-field-link', mono ? 'mono' : '', className].filter(Boolean).join(' ')}
      onClick={() => onInspect(ref.key)}
      title={`Inspect ${ref.kind}/${ref.name}`}
    >
      {text || '—'}
    </button>
  )
}

function SignalFirstPanel({
  inspect,
  unhealthy,
  header,
  summaryExtras,
  groups,
  relatedPodsBlock,
  relationshipItems = [],
  onInspect,
}) {
  const [metaOpen, setMetaOpen] = useState(!unhealthy)
  const stats = deriveSignalStats(inspect)

  useEffect(() => {
    setMetaOpen(!unhealthy)
  }, [inspect.key, unhealthy])

  return (
    <div className="inspect-panel mode-signal-first">
      {header}
      {summaryExtras}
      {unhealthy ? (
        <>
          <div className="inspect-signal-hero">
            <AnomalyCallout inspect={inspect} large />
            {stats.length > 0 && (
              <div className="inspect-signal-stats">
                {stats.map((s) => (
                  <div key={s.label} className="inspect-signal-stat">
                    <span className="signal-stat-value">{s.value}</span>
                    <span className="signal-stat-label">{s.label}</span>
                  </div>
                ))}
              </div>
            )}
          </div>
          <button
            type="button"
            className="inspect-meta-toggle"
            aria-expanded={metaOpen}
            onClick={() => setMetaOpen((v) => !v)}
          >
            {metaOpen ? 'Hide details' : 'Show details'}
          </button>
          {metaOpen && (
            <>
              {relatedPodsBlock}
              <StackedSections
                groups={groups}
                inspect={inspect}
                relationshipItems={relationshipItems}
                onInspect={onInspect}
                inspectNamespace={inspect.namespace}
              />
            </>
          )}
        </>
      ) : (
        <>
          <SignalsBlock inspect={inspect} unhealthy={unhealthy} quietHealthy compact />
          {relatedPodsBlock}
          <StackedSections
            groups={groups}
            inspect={inspect}
            relationshipItems={relationshipItems}
            onInspect={onInspect}
            inspectNamespace={inspect.namespace}
          />
        </>
      )}
    </div>
  )
}

function DetailTabsPanel({
  inspect,
  unhealthy,
  header,
  summaryExtras,
  groups,
  relatedPodsBlock,
  onInspect,
}) {
  const [tab, setTab] = useState(groups[0]?.id || 'status')
  const hasSummaryTab = groups.some((g) => g.id === 'summary')
  const lastInspectKeyRef = useRef(inspect.key)
  const metrics = useMemo(() => summaryMetrics(inspect), [inspect])
  const relationshipItems = useMemo(() => {
    const items = inspect?.relationships || []
    const hasPodList = inspect?.relatedPods?.length && inspect.kind !== 'Pod'
    if (!hasPodList) return items
    return items.filter((r) => r.kind !== 'Pod' && r.role !== 'Pod' && r.role !== 'Target pod')
  }, [inspect])
  const relationshipCenter = useMemo(() => ({
    kind: inspect.kind,
    name: inspect.name,
    key: inspect.key,
    namespace: inspect.namespace,
  }), [inspect.kind, inspect.name, inspect.key, inspect.namespace])

  // Reset tab only when the user selects a different object. Live refreshes rebuild
  // `groups` with a new reference — preserve the active tab when it still exists.
  useEffect(() => {
    const objectChanged = lastInspectKeyRef.current !== inspect.key
    lastInspectKeyRef.current = inspect.key
    const ids = new Set(groups.map((g) => g.id))

    setTab((current) => {
      if (objectChanged) return groups[0]?.id || 'status'
      if (ids.has(current)) return current
      return groups[0]?.id || 'status'
    })
  }, [inspect.key, groups])

  const active = groups.find((g) => g.id === tab) || groups[0]
  const serviceEndpoints = useMemo(() => findServiceEndpointFields(inspect), [inspect])
  const relationshipsTabGraph = useMemo(
    () => buildRelationshipGraph({
      center: relationshipCenter,
      items: relationshipItems,
      sections: active?.id === 'relationships' ? active.sections : [],
      inspectNamespace: inspect.namespace,
    }),
    [relationshipCenter, relationshipItems, active, inspect.namespace],
  )
  const summaryRelationshipsBlock = buildRelationshipGraph({
    center: relationshipCenter,
    items: relationshipItems,
    inspectNamespace: inspect.namespace,
  }).nodes.length > 1 ? (
    <RelationshipGraphPanel
      center={relationshipCenter}
      items={relationshipItems}
      inspectNamespace={inspect.namespace}
      onInspect={onInspect}
    />
  ) : null

  return (
    <div className="inspect-panel mode-detail-tabs">
      {header}
      {!hasSummaryTab && summaryExtras}
      <SignalsBlock inspect={inspect} unhealthy={unhealthy} quietHealthy compact={!!summaryExtras} />
      {groups.length > 0 && (
        <>
          {!hasSummaryTab && relatedPodsBlock}
          <div className="inspect-tabs" role="tablist" aria-label="Detail sections">
            {groups.map((g) => (
              <button
                key={g.id}
                type="button"
                role="tab"
                aria-selected={tab === g.id}
                className={`inspect-tab ${tab === g.id ? 'active' : ''}`}
                onClick={() => setTab(g.id)}
              >
                {g.label}
                {g.sections.length > 1 && (
                  <span className="inspect-tab-count">{g.sections.length}</span>
                )}
              </button>
            ))}
          </div>
          <div className="inspect-tab-panel" role="tabpanel">
            {active?.id === 'summary' && metrics.length > 0 && !active.sections.some((s) => s.fields?.length) && (
              <InspectSummaryMetrics
                metrics={metrics}
                onInspect={onInspect}
                inspectNamespace={inspect.namespace}
              />
            )}
            {active?.id === 'summary' && serviceEndpoints && (
              <ServiceEndpointSummary {...serviceEndpoints} />
            )}
            {active?.id === 'summary' && summaryRelationshipsBlock}
            {active?.id === 'summary' && relatedPodsBlock}
            {active?.id === 'relationships' && relationshipsTabGraph.nodes.length > 1 && (
              <RelationshipGraphPanel
                center={relationshipCenter}
                items={relationshipItems}
                sections={active.sections}
                showTitle={false}
                inspectNamespace={inspect.namespace}
                onInspect={onInspect}
              />
            )}
            {active?.id === 'relationships' && relationshipsTabGraph.nodes.length <= 1 && (
              <GroupBody
                group={active}
                inspect={inspect}
                relationshipItems={relationshipItems}
                onInspect={onInspect}
                hideGroupTitle
                inspectNamespace={inspect.namespace}
                sectionCard
                forceTableFallback
              />
            )}
            {active?.id === 'containers' && inspect.kind === 'Pod' && hasContainerPanelData(parseInspectContainers(active.sections)) ? (
              <InspectContainersPanel
                sections={active.sections}
                inspectNamespace={inspect.namespace}
                onInspect={onInspect}
              />
            ) : active && active.id !== 'relationships' ? (
              <GroupBody
                group={active}
                inspect={inspect}
                relationshipItems={relationshipItems}
                onInspect={onInspect}
                hideGroupTitle
                inspectNamespace={inspect.namespace}
                sectionCard
              />
            ) : null}
          </div>
        </>
      )}
    </div>
  )
}

function RelatedPodsSection({ pods, onInspect }) {
  if (!pods?.length) return null
  return (
    <section className="inspect-section inspect-related-pods">
      <h5 className="inspect-section-label">Pods</h5>
      <ul className="inspect-link-list">
        {pods.map((pod) => (
          <li key={pod.key}>
            <button
              type="button"
              className="inspect-link-row"
              onClick={() => onInspect?.(pod.key)}
              aria-label={`Inspect pod ${pod.name}`}
            >
              <KindIcon kind="Pod" size={15} />
              <span className="inspect-link-name">{pod.name}</span>
              <span className="inspect-link-meta">{pod.phase}</span>
              <span className="inspect-link-meta">{formatReady(pod.ready, pod.total)}</span>
              {pod.restarts > 0 && (
                <span className="inspect-link-meta muted">{pod.restarts} restarts</span>
              )}
              <RowStatusBadge status={pod.status} />
            </button>
          </li>
        ))}
      </ul>
    </section>
  )
}

function StackedSections({
  groups,
  inspect,
  relationshipItems = [],
  onInspect,
  inspectNamespace = '',
}) {
  if (!groups?.length) return null
  return (
    <div className="inspect-stack">
      {groups.map((g) => (
        <GroupBody
          key={g.id}
          group={g}
          inspect={inspect}
          relationshipItems={relationshipItems}
          showHeading
          onInspect={onInspect}
          inspectNamespace={inspectNamespace}
        />
      ))}
    </div>
  )
}

function GroupBody({
  group,
  inspect,
  relationshipItems = [],
  showHeading = false,
  hideGroupTitle = false,
  onInspect,
  inspectNamespace = '',
  sectionCard = false,
  forceTableFallback = false,
}) {
  if (group.id === 'relationships' && inspect && !forceTableFallback) {
    const graph = buildRelationshipGraph({
      center: {
        kind: inspect.kind,
        name: inspect.name,
        key: inspect.key,
        namespace: inspect.namespace,
      },
      items: relationshipItems,
      sections: group.sections,
      inspectNamespace: inspectNamespace || inspect.namespace,
    })
    if (graph.nodes.length > 1) {
      return (
        <RelationshipGraphPanel
          center={{
            kind: inspect.kind,
            name: inspect.name,
            key: inspect.key,
            namespace: inspect.namespace,
          }}
          items={relationshipItems}
          sections={group.sections}
          title={group.label}
          showTitle={showHeading && !hideGroupTitle}
          inspectNamespace={inspectNamespace || inspect.namespace}
          onInspect={onInspect}
        />
      )
    }
  }

  return (
    <div className="inspect-group">
      {showHeading && !hideGroupTitle && (
        <h5 className="inspect-group-title">{group.label}</h5>
      )}
      {group.sections.map((s) => {
        if (group.id === 'summary' && s.id === 'serviceEndpoint') return null
        return (
          <DetailSection
            key={s.id || s.title}
            section={s}
            groupId={group.id}
            onInspect={onInspect}
            inspectNamespace={inspectNamespace}
            card={sectionCard}
          />
        )
      })}
    </div>
  )
}

export function DetailSection({ section, groupId, onInspect, inspectNamespace = '', card = false }) {
  if (!section) return null
  const sectionClass = ['inspect-section', card ? 'inspect-section-card' : ''].filter(Boolean).join(' ')

  if (section._resourceBars) {
    return (
      <section className={sectionClass}>
        <h5 className="inspect-section-label">{section.title}</h5>
        <div className="resource-bars">
          {section._resourceBars.map((bar) => (
            <ResourceBar key={bar.id} bar={bar} />
          ))}
        </div>
      </section>
    )
  }

  if (section._labels || section._annotations) {
    const labels = section._labels || []
    const annotations = section._annotations || []
    if (!labels.length && !annotations.length) return null
    return (
      <section className={sectionClass}>
        <h5 className="inspect-section-label">{section.title}</h5>
        {labels.length > 0 && (
          <div className="inspect-meta-block">
            <span className="inspect-meta-label">Labels</span>
            <div className="inspect-chips">
              {labels.map((l) => (
                <span key={l.k} className="inspect-chip" title={`${l.k}=${l.v}`}>
                  <span className="chip-k">{l.k}</span>
                  <span className="chip-v">{l.v}</span>
                </span>
              ))}
            </div>
          </div>
        )}
        {annotations.length > 0 && (
          <div className="inspect-meta-block">
            <span className="inspect-meta-label">Annotations</span>
            <div className="inspect-chips">
              {annotations.map((a) => (
                <span key={a.k} className="inspect-chip annot" title={`${a.k}=${a.v}`}>
                  <span className="chip-k">{a.k}</span>
                  <span className="chip-v">{a.v}</span>
                </span>
              ))}
            </div>
          </div>
        )}
      </section>
    )
  }

  if (section._events) {
    return (
      <section className={sectionClass}>
        <h5 className="inspect-section-label">{section.title}</h5>
        <ul className="inspect-events">
          {section._events.map((ev, i) => (
            <li key={`${ev.time}-${i}`} className={`inspect-event sev-${ev.severity}`}>
              <span className="inspect-event-time mono">{ev.time}</span>
              <span className="inspect-event-reason">
                {ev.reason || ev.type}
                {ev.count > 1 ? ` ×${ev.count}` : ''}
              </span>
              <span className="inspect-event-msg" title={ev.message}>{ev.message}</span>
            </li>
          ))}
        </ul>
      </section>
    )
  }

  const hasFields = section.fields?.length > 0
  const hasKV = section.keyValues?.length > 0
  const hasTable = section.table?.rows?.length > 0
  const hasNotes = section.notes?.length > 0
  if (!hasFields && !hasKV && !hasTable && !hasNotes) return null

  return (
    <section className={sectionClass}>
      <h5 className="inspect-section-label">{section.title}</h5>
      {hasFields && (
        <dl className="inspect-prop-list">
          {section.fields.map((f, i) => (
            <div key={`${f.key}-${i}`} className="inspect-prop-row">
              <dt>{f.key}</dt>
              <dd>
                <InspectRefValue
                  value={f.value}
                  fieldKey={f.key}
                  section={section}
                  groupId={groupId}
                  onInspect={onInspect}
                  inspectNamespace={inspectNamespace}
                />
              </dd>
            </div>
          ))}
        </dl>
      )}
      {hasKV && (
        <div className="inspect-chips">
          {section.keyValues.map((kv) => (
            <span key={kv.key} className="inspect-chip" title={`${kv.key}=${kv.value}`}>
              <span className="chip-k">{kv.key}</span>
              <span className="chip-v">
                <InspectRefValue
                  value={kv.value}
                  fieldKey={kv.key}
                  section={section}
                  groupId={groupId}
                  onInspect={onInspect}
                  inspectNamespace={inspectNamespace}
                />
              </span>
            </span>
          ))}
        </div>
      )}
      {hasTable && (
        <div className="inspect-table-wrap">
          <table className="inspect-table">
            <thead>
              <tr>
                {section.table.columns.map((c) => (
                  <th key={c}>{c}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {section.table.rows.map((row, i) => (
                <tr key={i}>
                  {row.map((cell, j) => {
                    const valueCol = section.table.valueColumn ?? 1
                    if (section.table.sensitive && j === valueCol) {
                      return (
                        <td key={j}>
                          <SensitiveValueCell encoded={cell} />
                        </td>
                      )
                    }
                    const colName = section.table.columns[j]
                    const isValueCol = colName === 'Value'
                    const ref = onInspect
                      ? resolveInspectRef(cell, {
                        columnName: colName,
                        section,
                        groupId,
                        inspectNamespace,
                        row,
                        columns: section.table.columns,
                        columnIndex: j,
                      })
                      : null
                    return (
                      <td
                        key={j}
                        title={cell}
                        className={isValueCol ? 'inspect-value-cell' : undefined}
                      >
                        {ref?.key ? (
                          <button
                            type="button"
                            className="inspect-table-link"
                            onClick={() => onInspect(ref.key)}
                            title={`Inspect ${ref.kind}/${ref.name}`}
                          >
                            {cell || '—'}
                          </button>
                        ) : (
                          cell || '—'
                        )}
                      </td>
                    )
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {hasNotes && (
        <ul className="inspect-notes">
          {section.notes.map((n, i) => (
            <li key={i} className="muted">{n}</li>
          ))}
        </ul>
      )}
    </section>
  )
}

function SensitiveValueCell({ encoded }) {
  const [revealed, setRevealed] = useState(false)
  const display = revealed ? decodeSecretValue(encoded) : (encoded || '—')

  return (
    <div className="inspect-sensitive-cell">
      <span
        className={`inspect-sensitive-value mono ${revealed ? 'revealed' : 'encoded'}`}
        title={display}
      >
        {display}
      </span>
      <button
        type="button"
        className="inspect-reveal-btn"
        aria-label={revealed ? 'Hide decoded value' : 'Reveal decoded value'}
        aria-pressed={revealed}
        onClick={() => setRevealed((v) => !v)}
      >
        {revealed ? <EyeOffIcon /> : <EyeIcon />}
      </button>
    </div>
  )
}

function EyeIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z"
        stroke="currentColor"
        strokeWidth="1.75"
      />
      <circle cx="12" cy="12" r="3" stroke="currentColor" strokeWidth="1.75" />
    </svg>
  )
}

function EyeOffIcon() {
  return (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="M3 3l18 18M10.5 10.7A3 3 0 0 0 12 15a3 3 0 0 0 2.3-1M6.7 6.8C4.6 8.1 3 10 2 12s3.5 7 10 7c1.8 0 3.4-.4 4.8-1.1M14 9.2c.6.6 1 1.4 1 2.3a3 3 0 0 1-3 3"
        stroke="currentColor"
        strokeWidth="1.75"
        strokeLinecap="round"
      />
    </svg>
  )
}

function SignalsBlock({ inspect, unhealthy, quietHealthy, compact = false }) {
  if (quietHealthy && !unhealthy) {
    if (compact) return null
    return <p className="inspect-quiet-ok muted">No anomalies on this component.</p>
  }

  return (
    <section className={`inspect-section inspect-anomalies ${unhealthy ? 'has-issues' : 'clear'}`}>
      <h5 className="inspect-section-label">Signals</h5>
      <p className="inspect-events-hint muted">
        Anomalies for this component only — not the overall investigation.
      </p>
      {inspect.anomalies?.length ? (
        <ul className="inspect-anomaly-list">
          {inspect.anomalies.map((a, i) => (
            <li key={`${a.text}-${i}`} className={`inspect-anomaly level-${a.level}`}>
              <span className="anomaly-mark">{a.level === 'crit' ? '!' : a.level === 'warn' ? '▲' : '·'}</span>
              <span className="anomaly-text">{a.text}</span>
              {a.source && <span className="anomaly-src muted">{a.source}</span>}
            </li>
          ))}
        </ul>
      ) : (
        <p className="inspect-ok">No anomalies on this component.</p>
      )}
    </section>
  )
}

function AnomalyCallout({ inspect, large = false }) {
  const primary = (inspect.anomalies || []).find((a) => a.level === 'crit' || a.level === 'warn')
    || inspect.anomalies?.[0]
  if (!primary) return null
  return (
    <div className={`inspect-anomaly-callout ${large ? 'large' : ''} level-${primary.level}`}>
      <span className="callout-icon" aria-hidden="true">
        {primary.level === 'crit' ? '!' : '▲'}
      </span>
      <div className="callout-body">
        <strong className="callout-title">{primary.text}</strong>
        {primary.source && (
          <p className="callout-sub muted">{primary.source}</p>
        )}
        {(inspect.anomalies || []).length > 1 && (
          <ul className="inspect-anomaly-list callout-rest">
            {inspect.anomalies.slice(1).map((a, i) => (
              <li key={`${a.text}-${i}`} className={`inspect-anomaly level-${a.level}`}>
                <span className="anomaly-text">{a.text}</span>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}

function ResourceBar({ bar }) {
  if (bar.empty) {
    return (
      <div className="resource-bar empty">
        <div className="resource-bar-head">
          <span>{bar.label}</span>
          <span className="muted">{bar.source}</span>
        </div>
        <p className="muted inspect-none">{bar.detail}</p>
      </div>
    )
  }

  return (
    <div className={`resource-bar ${bar.compact ? 'compact' : ''}`}>
      <div className="resource-bar-head">
        <span>{bar.label}</span>
        <span className="resource-bar-detail mono">{bar.detail}</span>
      </div>
      <div className="resource-track" title={bar.detail}>
        {bar.limitPct > 0 && (
          <div className="resource-fill limit" style={{ width: `${bar.limitPct}%` }} />
        )}
        <div className="resource-fill request" style={{ width: `${bar.requestPct}%` }} />
        {bar.usagePct != null && (
          <div className="resource-fill usage" style={{ width: `${bar.usagePct}%` }} />
        )}
      </div>
      <div className="resource-legend">
        <span><i className="swatch request" /> request</span>
        {bar.limit > 0 && <span><i className="swatch limit" /> limit</span>}
        {bar.usage != null && <span><i className="swatch usage" /> usage</span>}
        <span className="muted">{bar.source}</span>
      </div>
    </div>
  )
}
