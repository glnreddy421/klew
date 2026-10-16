import { useCallback, useEffect, useId, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { KindIcon } from '../KindIcon'
import { isAccessDenied, isAccessError, isUnavailable, kindDisplayLabel, resourceMetadataTitle } from '../../lib/resourceCatalog.js'
import { resourceCategoryToneClass } from '../../lib/resourceCategoryIcons.js'
import { singleBrowseScope, normalizeBrowseScope } from '../../lib/browseScope.js'
import {
  enrichEntitiesForTable,
  formatConfigMapData,
  sortEntitiesForTable,
  tableCellValue,
  truncateSelector,
} from '../../lib/entityTable.js'
import { useEntityTableColumns } from '../../hooks/useEntityTableColumns.js'
import { useEntityTableColumnWidths } from '../../hooks/useEntityTableColumnWidths.js'
import {
  COLUMN_KEYBOARD_STEP,
  COLUMN_MAX_WIDTH,
  COLUMN_MIN_WIDTH,
  clampColumnWidth,
  completeColumnWidths,
  hasColumnWidths,
  measureHeaderWidths,
  measureNaturalColumnWidths,
  totalColumnWidth,
} from '../../lib/entityTableColumnWidths.js'
import { runPointerColResize } from '../../lib/paneDragResize.js'
import { EntityTableColumnPicker } from './EntityTableColumnPicker.jsx'
import { NodeConditionsCell, NodeNameCell, NodeResourceCell } from './NodeTableCells.jsx'
import { PodMetricCell } from '../metrics/ResourceMetricDisplay.jsx'
import { buildPodCpuMetric, buildPodMemMetric } from '../../lib/metricDisplay.js'
import { SchedulingTableCell } from './SchedulingTableCells.jsx'
import { ContainerStatusIndicators } from './ContainerStatusIndicators.jsx'
import { DeploymentConditionIndicators } from './DeploymentConditionIndicators.jsx'
import { LoadingBar, ResourcePaneLoading } from '../LoadingSpinner.jsx'
import { ClusterFetchActions } from '../ClusterFetchPanel.jsx'
import { ResourceAccessPanel } from './ResourceAccessPanel.jsx'
import { rowKeysMatch } from '../../lib/matches.js'

function StatusCell({ row }) {
  const tone = row.status || 'unknown'
  const label = row.table?.statusLabel || '—'
  return (
    <span className={`entity-table-status status-${tone === 'degraded' ? 'warning' : tone}`}>
      <span className="entity-table-status-led" aria-hidden="true" />
      {label}
    </span>
  )
}

function NamespaceCell({ row, browseScope, onBrowseScopeChange }) {
  const ns = tableCellValue(row, 'namespace')
  if (!ns || ns === '—') {
    return <span className="entity-table-ns-empty">—</span>
  }

  const current = normalizeBrowseScope(browseScope)
  const isCurrent = current.mode === 'single' && current.namespace === ns

  return (
    <button
      type="button"
      className={['entity-table-ns-link', isCurrent ? 'is-current' : ''].filter(Boolean).join(' ')}
      title={isCurrent ? ns : `Switch scope to ${ns}`}
      onClick={(e) => {
        e.stopPropagation()
        if (!isCurrent) onBrowseScopeChange?.(singleBrowseScope(ns))
      }}
    >
      {ns}
    </button>
  )
}

/** Right-edge grip: drag to resize, double-click (or Enter) to fit content, arrows to nudge. */
function ColumnResizeHandle({ col, width, onResizeStart, onFit, onNudge }) {
  return (
    <span
      className="entity-table-col-resize"
      role="separator"
      aria-orientation="vertical"
      aria-label={`Resize ${col.label} column`}
      aria-valuenow={width}
      aria-valuemin={COLUMN_MIN_WIDTH}
      aria-valuemax={COLUMN_MAX_WIDTH}
      tabIndex={0}
      title="Drag to resize · double-click to fit content"
      onPointerDown={(e) => {
        if (e.button !== 0) return
        e.stopPropagation()
        onResizeStart(col.id, e)
      }}
      onClick={(e) => e.stopPropagation()}
      onDoubleClick={(e) => {
        e.stopPropagation()
        onFit(col.id)
      }}
      onKeyDown={(e) => {
        if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') {
          e.preventDefault()
          onNudge(col.id, e.key === 'ArrowRight' ? COLUMN_KEYBOARD_STEP : -COLUMN_KEYBOARD_STEP)
        } else if (e.key === 'Enter') {
          e.preventDefault()
          onFit(col.id)
        }
      }}
    />
  )
}

function SortableColumnHeader({ col, sort, onSort, width, resize }) {
  const active = sort.columnId === col.id
  const ascending = active && sort.direction === 'asc'
  const descending = active && sort.direction === 'desc'

  return (
    <th
      key={col.id}
      data-col-id={col.id}
      className={[col.className, 'entity-table-sortable', active ? 'is-sorted' : ''].filter(Boolean).join(' ')}
      aria-sort={active ? (ascending ? 'ascending' : 'descending') : 'none'}
    >
      <button
        type="button"
        className="entity-table-sort-btn"
        onClick={() => onSort(col.id)}
        title={`Sort by ${col.label}`}
      >
        <span className="entity-table-sort-label">{col.label}</span>
        <span className="entity-table-sort-icons" aria-hidden="true">
          <span className={['entity-table-sort-arrow', 'up', ascending ? 'active' : ''].filter(Boolean).join(' ')}>▲</span>
          <span className={['entity-table-sort-arrow', 'down', descending ? 'active' : ''].filter(Boolean).join(' ')}>▼</span>
        </span>
      </button>
      {resize && <ColumnResizeHandle col={col} width={width} {...resize} />}
    </th>
  )
}

const ACTIONS_COL_ID = '__actions'

function TableCellClip({ value, className = '', mono = false }) {
  const text = value ?? '—'
  return (
    <span className={['entity-table-cell-clip', mono ? 'mono' : ''].filter(Boolean).join(' ')}>
      {text}
    </span>
  )
}

function SelectorTooltip({ value, tipId, coords }) {
  return createPortal(
    <div
      className="entity-table-selector-tooltip entity-table-selector-tooltip-fixed"
      role="tooltip"
      id={tipId}
      style={{ left: coords.x, top: coords.y }}
    >
      <div className="entity-table-selector-tooltip-k">Selector</div>
      <div className="entity-table-selector-tooltip-v mono">{value}</div>
    </div>,
    document.body,
  )
}

function SelectorCell({ row }) {
  const full = tableCellValue(row, 'selector')
  const [open, setOpen] = useState(false)
  const [coords, setCoords] = useState({ x: 0, y: 0 })
  const triggerRef = useRef(null)
  const tipId = useId()

  if (!full || full === '—') {
    return <span className="entity-table-selector-empty">—</span>
  }

  const short = truncateSelector(full, 18)
  const truncated = short !== full

  function showTooltip() {
    const rect = triggerRef.current?.getBoundingClientRect()
    if (rect) {
      setCoords({
        x: rect.left,
        y: rect.top - 8,
      })
    }
    setOpen(true)
  }

  return (
    <span
      ref={triggerRef}
      className="entity-table-selector"
      onMouseEnter={truncated ? showTooltip : undefined}
      onMouseLeave={() => setOpen(false)}
      onFocus={truncated ? showTooltip : undefined}
      onBlur={() => setOpen(false)}
      onClick={(e) => e.stopPropagation()}
      tabIndex={truncated ? 0 : -1}
      aria-label={truncated ? `Selector: ${full}` : undefined}
      aria-describedby={open ? tipId : undefined}
    >
      <svg
        className="entity-table-selector-icon"
        viewBox="0 0 16 16"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        aria-hidden="true"
      >
        <path d="M3.5 4.5h9M3.5 8h6M3.5 11.5h8" strokeLinecap="round" />
      </svg>
      <span className="entity-table-selector-text">{short}</span>
      {open && truncated && <SelectorTooltip value={full} tipId={tipId} coords={coords} />}
    </span>
  )
}

function ConfigMapDataKeysPopover({ entries, tipId, coords, above }) {
  return createPortal(
    <div
      className={[
        'entity-table-configmap-popover',
        above ? 'is-above' : '',
      ].filter(Boolean).join(' ')}
      role="tooltip"
      id={tipId}
      style={{ left: coords.x, top: coords.y }}
    >
      <div className="entity-table-configmap-popover-title">Data Keys</div>
      <div className="entity-table-configmap-popover-scroll">
        <table className="entity-table-configmap-tooltip-table">
          <thead>
            <tr>
              <th>Key</th>
              <th>Value</th>
              <th>Size</th>
            </tr>
          </thead>
          <tbody>
            {entries.map((entry) => (
              <tr key={entry.key}>
                <td className="mono entity-table-configmap-key">{entry.key}</td>
                <td className="mono entity-table-configmap-value" title={entry.value || ''}>
                  {entry.value || '—'}
                </td>
                <td className="mono entity-table-configmap-size">{entry.sizeBytes} bytes</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>,
    document.body,
  )
}

function ConfigMapDataKeysCell({ row }) {
  const entries = row.table?.dataKeysDetail || row.configMapData || []
  const summary = formatConfigMapData(entries)
  const [open, setOpen] = useState(false)
  const [coords, setCoords] = useState({ x: 0, y: 0, above: false })
  const triggerRef = useRef(null)
  const tipId = useId()

  if (!entries.length) {
    return <span className="entity-table-cell-clip">—</span>
  }

  function showPopover() {
    const rect = triggerRef.current?.getBoundingClientRect()
    if (!rect) return
    const popoverWidth = Math.min(560, window.innerWidth - 24)
    const spaceBelow = window.innerHeight - rect.bottom
    const above = spaceBelow < 220 && rect.top > spaceBelow
    const x = Math.max(12, Math.min(rect.left, window.innerWidth - popoverWidth - 12))
    setCoords({
      x,
      y: above ? rect.top - 8 : rect.bottom + 8,
      above,
    })
    setOpen(true)
  }

  return (
    <span
      ref={triggerRef}
      className="entity-table-configmap-data"
      onMouseEnter={showPopover}
      onMouseLeave={() => setOpen(false)}
      onFocus={showPopover}
      onBlur={() => setOpen(false)}
      onClick={(e) => e.stopPropagation()}
      tabIndex={0}
      aria-describedby={open ? tipId : undefined}
    >
      <button type="button" className="entity-table-configmap-pill" tabIndex={-1}>
        {summary}
      </button>
      {open && (
        <ConfigMapDataKeysPopover
          entries={entries}
          tipId={tipId}
          coords={coords}
          above={coords.above}
        />
      )}
    </span>
  )
}

function ControlledByCell({ row, onInspect }) {
  const label = row.table?.controlledBy || '—'
  const inspectKey = row.table?.controlledByKey
  const title = row.table?.controlledByTitle || label
  if (!inspectKey || label === '—') {
    return <span className="entity-table-owner-empty">—</span>
  }
  return (
    <button
      type="button"
      className="entity-table-owner-link"
      title={title}
      onClick={(e) => {
        e.stopPropagation()
        onInspect?.(inspectKey)
      }}
    >
      {label}
    </button>
  )
}

const FOCUSABLE_KINDS = new Set([
  'Deployment', 'StatefulSet', 'DaemonSet', 'Job', 'CronJob', 'ReplicaSet',
  'Pod', 'Service', 'Ingress', 'ConfigMap', 'Secret', 'PersistentVolumeClaim',
])

function TableRow({
  row,
  selected,
  focusKey,
  columns,
  onSelect,
  onInspect,
  onFocus,
  showFocusButton,
  browseScope,
  onBrowseScopeChange,
}) {
  const isRoot = rowKeysMatch(focusKey, row.key)
  const canFocus = showFocusButton && onFocus && FOCUSABLE_KINDS.has(row.kind)
  return (
    <tr
      className={[
        'entity-table-row',
        selected ? 'selected' : '',
        isRoot ? 'focus-root' : '',
      ].filter(Boolean).join(' ')}
      onClick={() => onSelect?.(row.key)}
      tabIndex={0}
      role="row"
      aria-selected={selected}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault()
          onSelect?.(row.key)
        }
      }}
    >
      {columns.map((col) => {
        switch (col.id) {
          case 'name':
            return (
              <td key={col.id} className={col.className} title={row.name}>
                {row.kind === 'Node' ? (
                  <NodeNameCell row={row} />
                ) : (
                  <span className="entity-table-name">{row.name}</span>
                )}
              </td>
            )
          case 'status':
            return (
              <td key={col.id} className={col.className}>
                <StatusCell row={row} />
              </td>
            )
          case 'containers':
            return (
              <td key={col.id} className={col.className}>
                <ContainerStatusIndicators containers={row.table?.containers || []} />
              </td>
            )
          case 'cpu':
            if (row.kind === 'Node') {
              return (
                <td key={col.id} className={col.className}>
                  <NodeResourceCell
                    capacity={row.nodeResources?.capacityCpuMilli}
                    allocatable={row.nodeResources?.allocatableCpuMilli}
                  />
                </td>
              )
            }
            if (row.kind === 'Pod') {
              return (
                <td key={col.id} className={col.className}>
                  <PodMetricCell
                    metric={buildPodCpuMetric(row)}
                    fallback={tableCellValue(row, col.id)}
                  />
                </td>
              )
            }
            return (
              <td key={col.id} className={col.className}>
                <TableCellClip mono value={tableCellValue(row, col.id)} />
              </td>
            )
          case 'memory':
            if (row.kind === 'Node') {
              return (
                <td key={col.id} className={col.className}>
                  <NodeResourceCell
                    capacity={row.nodeResources?.capacityMemoryBytes}
                    allocatable={row.nodeResources?.allocatableMemoryBytes}
                  />
                </td>
              )
            }
            if (row.kind === 'Pod') {
              return (
                <td key={col.id} className={col.className}>
                  <PodMetricCell
                    metric={buildPodMemMetric(row)}
                    fallback={tableCellValue(row, col.id)}
                  />
                </td>
              )
            }
            return (
              <td key={col.id} className={col.className}>
                <TableCellClip mono value={tableCellValue(row, col.id)} />
              </td>
            )
          case 'disk':
            return (
              <td key={col.id} className={col.className}>
                {row.kind === 'Node' ? (
                  <NodeResourceCell
                    capacity={row.nodeResources?.capacityDiskBytes}
                    allocatable={row.nodeResources?.allocatableDiskBytes}
                  />
                ) : (
                  <TableCellClip mono value={tableCellValue(row, col.id)} />
                )}
              </td>
            )
          case 'taints':
          case 'nodeSelector':
          case 'tolerations':
          case 'affinity':
            return (
              <td key={col.id} className={col.className}>
                <SchedulingTableCell row={row} columnId={col.id} />
              </td>
            )
          case 'qos':
          case 'age':
          case 'duration':
          case 'lastSchedule':
            return (
              <td key={col.id} className={col.className}>
                <TableCellClip mono value={tableCellValue(row, col.id)} />
              </td>
            )
          case 'controlledBy':
            return (
              <td key={col.id} className={col.className}>
                <ControlledByCell row={row} onInspect={onInspect} />
              </td>
            )
          case 'conditions':
            return (
              <td key={col.id} className={col.className}>
                {row.kind === 'Node' ? (
                  <NodeConditionsCell row={row} />
                ) : (
                  <DeploymentConditionIndicators conditions={row.table?.conditionsDetail || []} />
                )}
              </td>
            )
          case 'version':
          case 'roles':
            return (
              <td key={col.id} className={col.className}>
                <TableCellClip mono value={tableCellValue(row, col.id)} />
              </td>
            )
          case 'namespace':
            return (
              <td key={col.id} className={col.className}>
                <NamespaceCell
                  row={row}
                  browseScope={browseScope}
                  onBrowseScopeChange={onBrowseScopeChange}
                />
              </td>
            )
          case 'selector':
            return (
              <td key={col.id} className={col.className}>
                <SelectorCell row={row} />
              </td>
            )
          case 'dataKeys':
            return (
              <td key={col.id} className={col.className}>
                <ConfigMapDataKeysCell row={row} />
              </td>
            )
          case 'node':
          case 'desired':
          case 'current':
          case 'ready':
          case 'updated':
          case 'available':
          case 'misscheduled':
          case 'completions':
          case 'schedule':
          case 'suspend':
          case 'active':
          case 'restarts':
          case 'pods':
          case 'replicas':
          case 'type':
          case 'clusterIP':
          case 'ports':
          case 'externalIP':
          case 'endpoints':
          case 'service':
          case 'loadBalancers':
          case 'rules':
          case 'controller':
          case 'apiGroup':
          case 'scope':
          case 'parameterKind':
          case 'policyTypes':
          case 'provisioner':
          case 'reclaimPolicy':
          case 'volumeBindingMode':
          case 'defaultClass':
          case 'volume':
          case 'capacity':
          case 'accessModes':
          case 'storageClass':
          case 'claim':
          case 'keys':
          case 'secretType':
          case 'scaleTarget':
          case 'targets':
          case 'minPods':
          case 'maxPods':
          case 'hpaReplicas':
          case 'minAvailable':
          case 'maxUnavailable':
          case 'allowedDisruptions':
          case 'holder':
          case 'reason':
          case 'object':
          case 'message':
          case 'lastSeen':
          case 'secrets':
          case 'role':
          case 'subjects':
          case 'hard':
          case 'limits':
          case 'value':
          case 'globalDefault':
          case 'handler':
          case 'attacher':
          case 'pv':
          case 'attachRequired':
          case 'podInfoOnMount':
          case 'storageCapacity':
          case 'drivers':
          case 'group':
          case 'webhooks':
          case 'failurePolicy':
          case 'matchConstraints':
          case 'policy':
          case 'validationActions':
          case 'addressType':
          case 'chart':
          case 'revision':
          case 'chartVersion':
          case 'appVersion':
          case 'updated':
          case 'releases':
            return (
              <td key={col.id} className={col.className}>
                <TableCellClip mono value={tableCellValue(row, col.id)} />
              </td>
            )
          default: {
            const value = tableCellValue(row, col.id)
            if (value === '—') return <td key={col.id} className={col.className}>—</td>
            return (
              <td key={col.id} className={col.className}>
                <TableCellClip mono value={value} />
              </td>
            )
          }
        }
      })}
      {showFocusButton && onFocus && (
        <td className="entity-table-actions">
          {canFocus ? (
            <button
              type="button"
              className="entity-list-focus entity-table-focus"
              title={`Focus ${row.name}`}
              aria-label={`Focus ${row.name}`}
              onClick={(e) => {
                e.stopPropagation()
                onFocus?.(row.key)
              }}
            >
              Focus
            </button>
          ) : null}
        </td>
      )}
    </tr>
  )
}

/**
 * Tabular entity collection for table-oriented workspace layouts.
 */
export function EntityTable({
  kind,
  kindGroup,
  categoryId = '',
  entities = [],
  filteredEntities = [],
  entitiesLoading = false,
  resourceCenterLoading = false,
  catalogPending = false,
  loadingHint,
  onRetry,
  onReconnect,
  onOpenProxySettings,
  reconnectBusy = false,
  refreshing = false,
  pods = [],
  inspectKey,
  focusKey,
  showFocusButton = false,
  browseScope,
  onBrowseScopeChange,
  onSelect,
  onFocus,
  hasSearchQuery = '',
}) {
  const label = kindGroup?.label || (kind ? kindDisplayLabel(kind) : 'Resources')
  const resolvedKind = kind || kindGroup?.kind || ''
  const { columns, selectable, visibleIds, setVisibleIds } = useEntityTableColumns({
    kind: resolvedKind,
    kindGroup,
    browseScope,
  })
  const [sort, setSort] = useState({ columnId: 'name', direction: 'asc' })
  const tableRef = useRef(null)
  const { widths, previewWidths, commitWidths, setWidths, resetWidths } = useEntityTableColumnWidths(resolvedKind)
  const showActionsColumn = showFocusButton && !!onFocus
  const layoutIds = useMemo(
    () => [...columns.map((c) => c.id), ...(showActionsColumn ? [ACTIONS_COL_ID] : [])],
    [columns, showActionsColumn],
  )
  const manualWidths = hasColumnWidths(widths)

  /** Widths for every rendered column: saved, else as currently laid out. */
  const frozenWidths = useCallback(
    () => completeColumnWidths(widths, layoutIds, measureHeaderWidths(tableRef.current)),
    [layoutIds, widths],
  )

  // Widths freeze (switching to fixed layout) on the first real move, so a plain
  // click on the grip changes nothing; they persist once, when the drag ends.
  const startColumnResize = useCallback((colId, e) => {
    const base = frozenWidths()
    let moved = false
    runPointerColResize(e, {
      startX: e.clientX,
      startSize: base[colId],
      min: COLUMN_MIN_WIDTH,
      max: COLUMN_MAX_WIDTH,
      onSize: (w) => {
        moved = true
        previewWidths(() => ({ ...base, [colId]: Math.round(w) }))
      },
      onEnd: () => {
        if (moved) commitWidths()
      },
    })
  }, [commitWidths, frozenWidths, previewWidths])

  const fitColumn = useCallback((colId) => {
    const base = frozenWidths()
    const natural = measureNaturalColumnWidths(tableRef.current)
    if (natural[colId] == null) return
    setWidths(() => ({ ...base, [colId]: clampColumnWidth(natural[colId]) }))
  }, [frozenWidths, setWidths])

  const nudgeColumn = useCallback((colId, delta) => {
    const base = frozenWidths()
    setWidths(() => ({ ...base, [colId]: clampColumnWidth(base[colId] + delta) }))
  }, [frozenWidths, setWidths])

  const columnResize = useMemo(
    () => ({ onResizeStart: startColumnResize, onFit: fitColumn, onNudge: nudgeColumn }),
    [fitColumn, nudgeColumn, startColumnResize],
  )

  useEffect(() => {
    setSort({ columnId: 'name', direction: 'asc' })
  }, [resolvedKind])

  const rows = useMemo(
    () => enrichEntitiesForTable(filteredEntities, pods, resolvedKind),
    [filteredEntities, pods, resolvedKind],
  )
  const sortedRows = useMemo(
    () => sortEntitiesForTable(rows, sort.columnId, sort.direction),
    [rows, sort.columnId, sort.direction],
  )

  function handleSort(columnId) {
    setSort((prev) => {
      if (prev.columnId === columnId) {
        return { columnId, direction: prev.direction === 'asc' ? 'desc' : 'asc' }
      }
      return { columnId, direction: 'asc' }
    })
  }

  const centerLoading = resourceCenterLoading || (entitiesLoading && sortedRows.length === 0)

  const accessBlocked = entities.length === 0
    && !entitiesLoading
    && !resourceCenterLoading
    && !catalogPending
    && (isAccessDenied(kindGroup) || isUnavailable(kindGroup) || isAccessError(kindGroup))

  const toneClass = resourceCategoryToneClass(categoryId)

  return (
    <section className="entity-table" aria-label={`${label} entities`}>
      <header className={`entity-table-header ${toneClass}`.trim()}>
        <div className="entity-table-heading">
          {kind ? (
            <span className="entity-table-icon-badge">
              <KindIcon kind={kind} size={16} />
            </span>
          ) : null}
          <div className="entity-table-heading-copy">
            <div className="entity-table-heading-row">
              <h4 className="entity-table-title">
                {label}
                {resolvedKind === 'Endpoints' && (
                  <span className="entity-table-legacy-tag">legacy</span>
                )}
              </h4>
              {!accessBlocked && (
                <span className="entity-table-count">
                  {entitiesLoading && entities.length === 0 ? '…' : entities.length}
                </span>
              )}
            </div>
            {kindGroup && (
              <p className="entity-table-meta" title={resourceMetadataTitle(kindGroup)}>
                <span className="entity-table-api mono">
                  {kindGroup.apiVersion || kindGroup.group
                    ? `${kindGroup.group || 'core'}/${kindGroup.resource || kind}`
                    : kind}
                </span>
              </p>
            )}
          </div>
          {!accessBlocked && manualWidths && (
            <button
              type="button"
              className="text-link-btn entity-table-reset-widths"
              onClick={resetWidths}
              title="Return all columns to automatic widths"
            >
              Auto widths
            </button>
          )}
          {!accessBlocked && selectable.length > 0 && (
            <EntityTableColumnPicker
              columns={selectable}
              visibleIds={visibleIds}
              onChange={setVisibleIds}
            />
          )}
        </div>
      </header>

      <div className="entity-table-body">
        {accessBlocked ? (
          <ResourceAccessPanel
            kindGroup={kindGroup}
            onRetry={onRetry}
            onReconnect={onReconnect}
            onOpenProxySettings={onOpenProxySettings}
            reconnectBusy={reconnectBusy}
          />
        ) : centerLoading ? (
          <ResourcePaneLoading slowHint={loadingHint}>
            <ClusterFetchActions
              onReconnect={onReconnect}
              onOpenProxySettings={onOpenProxySettings}
              reconnectBusy={reconnectBusy}
            />
          </ResourcePaneLoading>
        ) : (
          <div className="entity-table-scroll">
            {refreshing && sortedRows.length > 0 && (
              <LoadingBar className="resource-pane-refresh-bar" label="Refreshing resources" />
            )}
            <table
              ref={tableRef}
              className={['entity-table-grid', manualWidths ? 'is-sized' : ''].filter(Boolean).join(' ')}
              style={manualWidths ? { width: `${totalColumnWidth(widths, layoutIds)}px` } : undefined}
              data-inspector-keep
            >
              {manualWidths && (
                <colgroup>
                  {layoutIds.map((id) => (
                    <col key={id} style={{ width: `${completeColumnWidths(widths, [id])[id]}px` }} />
                  ))}
                </colgroup>
              )}
              <thead>
                <tr>
                  {columns.map((col) => (
                    <SortableColumnHeader
                      key={col.id}
                      col={col}
                      sort={sort}
                      onSort={handleSort}
                      width={widths[col.id]}
                      resize={columnResize}
                    />
                  ))}
                  {showActionsColumn && (
                    <th
                      data-col-id={ACTIONS_COL_ID}
                      className="entity-table-actions col-actions"
                      aria-label="Focus actions"
                    />
                  )}
                </tr>
              </thead>
              <tbody>
                {sortedRows.map((row) => (
                  <TableRow
                    key={row.key}
                    row={row}
                    columns={columns}
                    selected={rowKeysMatch(inspectKey, row.key)}
                    focusKey={focusKey}
                    onSelect={onSelect}
                    onInspect={onSelect}
                    onFocus={onFocus}
                    showFocusButton={showFocusButton}
                    browseScope={browseScope}
                    onBrowseScopeChange={onBrowseScopeChange}
                  />
                ))}
              </tbody>
            </table>
            {!entitiesLoading && !sortedRows.length && hasSearchQuery && (
              <div className="entity-table-empty entity-table-empty-filtered">
                <p>No matches for &quot;{hasSearchQuery}&quot;</p>
              </div>
            )}
          </div>
        )}
      </div>
    </section>
  )
}
