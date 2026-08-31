import { KindIcon } from '../KindIcon'
import { kindDisplayLabel, resourceMetadataTitle } from '../../lib/resourceCatalog.js'
import { enrichEntitiesForTable, tableColumnsForDensity } from '../../lib/entityTable.js'
import { ResourceAccessPanel } from './ResourceAccessPanel.jsx'

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

function TableRow({ row, selected, focusKey, columns, onSelect }) {
  const isRoot = focusKey === row.key
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
        const t = row.table || {}
        switch (col.id) {
          case 'name':
            return (
              <td key={col.id} className={col.className} title={row.name}>
                <span className="entity-table-name">{row.name}</span>
              </td>
            )
          case 'status':
            return (
              <td key={col.id} className={col.className}>
                <StatusCell row={row} />
              </td>
            )
          case 'namespace':
            return <td key={col.id} className={`${col.className} mono`}>{t.namespace || '—'}</td>
          case 'node':
            return <td key={col.id} className={`${col.className} mono`} title={t.node}>{t.node || '—'}</td>
          case 'restarts':
            return <td key={col.id} className={`${col.className} mono`}>{t.restarts ?? '—'}</td>
          case 'cpu':
            return <td key={col.id} className={`${col.className} mono`}>{t.cpu || '—'}</td>
          case 'memory':
            return <td key={col.id} className={`${col.className} mono`}>{t.memory || '—'}</td>
          case 'age':
            return <td key={col.id} className={`${col.className} mono`}>{t.age || '—'}</td>
          default:
            return <td key={col.id}>—</td>
        }
      })}
    </tr>
  )
}

/**
 * Tabular entity collection for table-oriented workspace layouts.
 */
export function EntityTable({
  kind,
  kindGroup,
  entities = [],
  filteredEntities = [],
  entitiesLoading = false,
  pods = [],
  inspectKey,
  focusKey,
  density = 'standard',
  onSelect,
  hasSearchQuery = '',
}) {
  const label = kindGroup?.label || (kind ? kindDisplayLabel(kind) : 'Resources')
  const columns = tableColumnsForDensity(density)
  const rows = enrichEntitiesForTable(filteredEntities, pods)

  const accessBlocked = entities.length === 0
    && !entitiesLoading
    && (
      kindGroup?.accessState === 'forbidden'
      || kindGroup?.countState?.state === 'forbidden'
      || (!kindGroup?.discovered && kindGroup?.builtin && !kindGroup?.discoveredOnly)
      || kindGroup?.accessState === 'unavailable'
      || kindGroup?.countState?.state === 'unavailable'
    )

  return (
    <section className={`entity-table entity-table-${density}`} aria-label={`${label} entities`}>
      <header className="entity-table-header">
        <div className="entity-table-heading">
          {kind ? <KindIcon kind={kind} size={16} /> : null}
          <h4 className="entity-table-title">{label}</h4>
          {!accessBlocked && (
            <span className="entity-table-count">{entitiesLoading ? '…' : entities.length}</span>
          )}
        </div>
        {kindGroup && (
          <p className="entity-table-meta muted" title={resourceMetadataTitle(kindGroup)}>
            {kindGroup.apiVersion || kindGroup.group ? `${kindGroup.group || 'core'}/${kindGroup.resource || kind}` : kind}
          </p>
        )}
      </header>

      <div className="entity-table-body">
        {accessBlocked ? (
          <ResourceAccessPanel kindGroup={kindGroup} />
        ) : (
          <>
            {entitiesLoading && <p className="entity-table-loading">Loading…</p>}
            {!entitiesLoading && rows.length > 0 && (
              <div className="entity-table-scroll">
                <table className="entity-table-grid">
                  <thead>
                    <tr>
                      {columns.map((col) => (
                        <th key={col.id} className={col.className}>{col.label}</th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {rows.map((row) => (
                      <TableRow
                        key={row.key}
                        row={row}
                        columns={columns}
                        selected={inspectKey === row.key}
                        focusKey={focusKey}
                        onSelect={onSelect}
                      />
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            {!entitiesLoading && !rows.length && (
              <div className="entity-table-empty">
                <p>{hasSearchQuery ? `No matches for "${hasSearchQuery}"` : `No ${label.toLowerCase()} in this scope`}</p>
              </div>
            )}
          </>
        )}
      </div>
    </section>
  )
}
