import { RowStatusBadge } from './StatusBadge'
import { KindIcon } from '../KindIcon'
import { formatReady, groupRowsByKind } from '../../lib/matches'

/**
 * @param {'match' | 'chain'} mode
 *   match — select inspects; optional Focus enters isolated chain
 *   chain — select inspects only (already isolated)
 */
export function MatchedComponentsList({
  rows,
  focusKey,
  inspectKey,
  mode = 'match',
  grouped = false,
  onInspect,
  onFocus,
  showFocusButton = true,
  dense = false,
  statusLed = false,
  allowHoverInspect = true,
  focusChevron = false,
}) {
  if (!rows.length) {
    return <div className="matched-empty muted">No matched components in scope</div>
  }

  const chain = mode === 'chain'
  const perRowFocus = !chain && showFocusButton
  const chevronFocus = !chain && focusChevron
  const listClass = [
    'matched-list',
    chain ? 'chain-mode' : 'match-mode',
    dense ? 'dense' : '',
    statusLed ? 'with-led' : '',
  ].filter(Boolean).join(' ')

  if (grouped && !chain) {
    const groups = groupRowsByKind(rows)
    return (
      <div className="matched-groups" role="listbox" aria-label="Matched components">
        {groups.map((group) => (
          <section key={group.kind} className="matched-group">
            <header className="matched-group-h">
              <KindIcon kind={group.kind} size={13} />
              <span>{group.label}</span>
              <span className="matched-group-count muted mono">{group.items.length}</span>
            </header>
            <ul className={listClass}>
              {group.items.map((row) => (
                <MatchedRow
                  key={row.key}
                  row={row}
                  chain={chain}
                  focusKey={focusKey}
                  inspectKey={inspectKey}
                  perRowFocus={perRowFocus}
                  chevronFocus={chevronFocus}
                  dense={dense}
                  statusLed={statusLed}
                  allowHoverInspect={allowHoverInspect}
                  onInspect={onInspect}
                  onFocus={onFocus}
                />
              ))}
            </ul>
          </section>
        ))}
      </div>
    )
  }

  return (
    <ul className={listClass} role="listbox" aria-label={chain ? 'Focus chain' : 'Matched components'}>
      {rows.map((row) => (
        <MatchedRow
          key={row.key}
          row={row}
          chain={chain}
          focusKey={focusKey}
          inspectKey={inspectKey}
          perRowFocus={perRowFocus}
          chevronFocus={chevronFocus}
          dense={dense}
          statusLed={statusLed}
          allowHoverInspect={allowHoverInspect}
          onInspect={onInspect}
          onFocus={onFocus}
        />
      ))}
    </ul>
  )
}

function MatchedRow({
  row,
  chain,
  focusKey,
  inspectKey,
  perRowFocus,
  chevronFocus,
  dense,
  statusLed,
  allowHoverInspect,
  onInspect,
  onFocus,
}) {
  const isRoot = focusKey === row.key
  const active = inspectKey === row.key
  const tone = row.status || 'unknown'

  return (
    <li className="matched-item">
      <button
        type="button"
        role="option"
        aria-selected={active}
        className={[
          'matched-row',
          active ? 'selected' : '',
          chain && isRoot ? 'chain-root' : '',
          active ? 'inspecting' : '',
        ].filter(Boolean).join(' ')}
        onClick={() => onInspect?.(row.key)}
        onMouseEnter={allowHoverInspect ? () => onInspect?.(row.key) : undefined}
        onFocus={allowHoverInspect ? () => onInspect?.(row.key) : undefined}
        title={chain ? 'Select to inspect this component' : 'Select to inspect'}
      >
        {statusLed && (
          <span
            className={`match-led status-${tone === 'degraded' ? 'warning' : tone}`}
            aria-hidden="true"
            title={tone}
          />
        )}
        <KindIcon kind={row.kind} />
        <span className="match-name" title={row.name}>{row.name}</span>
        <span className="match-ready">{formatReady(row.ready, row.total)}</span>
        {!dense && <span className="match-restarts">{row.restarts ?? 0}</span>}
        {!statusLed && <RowStatusBadge status={row.status} />}
        {statusLed && !dense && tone !== 'healthy' && (
          <RowStatusBadge status={row.status} />
        )}
        {statusLed && dense && tone !== 'healthy' && (
          <span className={`match-status-short status-${tone === 'degraded' ? 'warning' : tone}`}>
            {tone === 'critical' ? 'Crit' : tone === 'degraded' ? 'Degraded' : tone}
          </span>
        )}
      </button>
      {perRowFocus && (
        <button
          type="button"
          className="match-focus-btn"
          onClick={(e) => {
            e.stopPropagation()
            onFocus?.(row.key)
          }}
          title={`Focus ${row.kind}/${row.name} — isolate related resources`}
          aria-label={`Focus ${row.name}`}
        >
          <span className="match-focus-label">Focus</span>
          <svg className="match-focus-chevron" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" aria-hidden="true">
            <path d="M9 6l6 6-6 6" />
          </svg>
        </button>
      )}
      {chevronFocus && (
        <button
          type="button"
          className="match-focus-chevron-btn"
          onClick={(e) => {
            e.stopPropagation()
            onFocus?.(row.key)
          }}
          title={`Focus ${row.kind}/${row.name}`}
          aria-label={`Focus ${row.name}`}
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" aria-hidden="true">
            <path d="M9 6l6 6-6 6" />
          </svg>
        </button>
      )}
    </li>
  )
}
