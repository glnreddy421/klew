import { KindIcon } from '../KindIcon'
import { formatReady, rowKeysMatch } from '../../lib/matches'
import { isAccessDenied, isAccessError, isUnavailable, kindDisplayLabel } from '../../lib/resourceCatalog.js'
import { LoadingBar, ResourcePaneLoading } from '../LoadingSpinner.jsx'
import { ClusterFetchActions } from '../ClusterFetchPanel.jsx'
import { ResourceAccessPanel } from './ResourceAccessPanel.jsx'

function EmptyEntityState({ label, hasSearch }) {
  const name = label || 'resources'
  return (
    <div className="entity-list-empty">
      <p>{hasSearch ? `No matches for "${hasSearch}"` : `No ${name.toLowerCase()} in this scope`}</p>
    </div>
  )
}

function EntityListItem({
  row,
  selected,
  onSelect,
}) {
  const tone = row.status || 'unknown'
  const isPod = row.kind === 'Pod'
  const showReady = isPod && (row.ready != null || row.total != null)

  return (
    <li className="entity-list-item">
      <button
        type="button"
        className={[
          'entity-list-row',
          selected ? 'selected' : '',
        ].filter(Boolean).join(' ')}
        aria-selected={selected}
        onClick={() => onSelect?.(row.key)}
      >
        <span
          className={`entity-list-led status-${tone === 'degraded' ? 'warning' : tone}`}
          aria-hidden="true"
        />
        <span className="entity-list-name" title={row.name}>{row.name}</span>
        {showReady && (
          <span className="entity-list-ready">{formatReady(row.ready, row.total)}</span>
        )}
      </button>
    </li>
  )
}

/**
 * Entity collection for the selected resource kind.
 */
export function EntityList({
  kind,
  kindGroup,
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
  inspectKey,
  onSelect,
  title,
  hasSearchQuery = '',
}) {
  const label = title || kindGroup?.label || (kind ? kindDisplayLabel(kind) : 'Resources')
  const list = filteredEntities

  const centerLoading = resourceCenterLoading || (entitiesLoading && list.length === 0)

  const accessBlocked = entities.length === 0
    && !entitiesLoading
    && !resourceCenterLoading
    && !catalogPending
    && (isAccessDenied(kindGroup) || isUnavailable(kindGroup) || isAccessError(kindGroup))

  return (
    <section className="entity-list" aria-label={`${label} entities`}>
      <header className="entity-list-header">
        <div className="entity-list-heading">
          {kind ? <KindIcon kind={kind} size={16} /> : null}
          <h4 className="entity-list-title">{label}</h4>
          {!accessBlocked && (
            <span className="entity-list-count">
              {entitiesLoading && entities.length === 0 ? '…' : entities.length}
            </span>
          )}
        </div>
      </header>

      <div className="entity-list-body">
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
          <>
            {refreshing && list.length > 0 && (
              <LoadingBar className="resource-pane-refresh-bar" label="Refreshing resources" />
            )}
            {list.length > 0 && (
              <ul className="entity-list-rows" data-inspector-keep>
                {list.map((row) => (
                  <EntityListItem
                    key={row.key}
                    row={row}
                    selected={rowKeysMatch(inspectKey, row.key)}
                    onSelect={onSelect}
                  />
                ))}
              </ul>
            )}
            {!list.length && !entitiesLoading && (
              <EmptyEntityState label={label} hasSearch={hasSearchQuery} />
            )}
          </>
        )}
      </div>
    </section>
  )
}
