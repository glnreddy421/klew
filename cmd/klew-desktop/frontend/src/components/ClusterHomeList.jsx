import { useCallback, useEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { ClusterCloudIcon } from './shell/ClusterCloudIcon.jsx'
import { ClusterFetchPanel } from './ClusterFetchPanel.jsx'
import { ContextTagsEditor } from './shell/ContextTagsEditor.jsx'
import { LoadingState } from './LoadingSpinner.jsx'
import { classifyConnectionError, connectionErrorMessage } from '../lib/clusterConnection.js'
import {
  aliasForContext,
  detailsForContext,
  displayContextName,
  formatContextDetailMeta,
  isConnectedContext,
  isFavoriteContext,
  MAX_CONTEXT_ALIAS_LEN,
  orderContextsForDisplay,
  tagsForContext,
} from '../lib/clusterContexts.js'
import { FavoriteStarIcon, LaunchDefaultIcon } from './shell/ContextActionIcons.jsx'
import { ContextRecordingToggle } from './shell/ContextRecordingToggle.jsx'
import { HoverTip } from './HoverTip.jsx'

const COLUMNS = [
  { id: 'name', label: 'Context', className: 'col-name', sortable: true },
  { id: 'cluster', label: 'Cluster', className: 'col-cluster', sortable: true },
  { id: 'version', label: 'Version', className: 'col-version', sortable: true },
  { id: 'tags', label: 'Tags', className: 'col-tags', sortable: true },
  { id: 'history', label: '', className: 'col-history', sortable: false },
  { id: 'status', label: 'Status', className: 'col-status', sortable: false },
  { id: 'actions', label: '', className: 'col-actions', sortable: false },
]

function formatContextVersion(version) {
  const v = String(version || '').trim()
  if (!v) return '—'
  return v.startsWith('v') ? v : `v${v}`
}

/**
 * Full-width kubeconfig contexts table for Home.
 */
export function ClusterHomeList({
  cluster,
  connection,
  syncing = false,
  connecting = false,
  statusLoading = false,
  reconnectBusy = false,
  monitoringPaused = false,
  defaultContext = '',
  favoriteContexts = [],
  connectedContexts = [],
  contextTags = {},
  contextAliases = {},
  contextDetails = {},
  displayContexts = null,
  filtering = false,
  sort = null,
  onSort,
  onSelectCluster,
  onSetDefaultContext,
  onToggleFavoriteContext,
  onAddContextTag,
  onRemoveContextTag,
  onRenameContext,
  onReconnect,
  onDisconnect,
  onOpenProxySettings,
  layout = 'list',
}) {
  const [menu, setMenu] = useState(null)
  const closeMenu = useCallback(() => setMenu(null), [])
  const contexts = cluster?.contexts || []
  const rows = Array.isArray(displayContexts)
    ? displayContexts
    : orderContextsForDisplay(contexts, { defaultContext, favoriteContexts })
  const active = cluster?.selectedContext || cluster?.currentContext || ''
  const syncError = String(cluster?.syncError || '').trim()
  const attempting = !monitoringPaused && (syncing || connecting || statusLoading)
  const phase = connection?.phase
  const disconnected = phase === 'disconnected' || phase === 'retrying' || Boolean(syncError)
  const classified = classifyConnectionError(
    syncError || (disconnected ? connectionErrorMessage(cluster, null) : ''),
  )
  const errorMessage = classified.message
  const busy = connecting || reconnectBusy
  const authFailed = classified.kind === 'auth'

  return (
    <div className="entity-table-body klew-contexts-body">
      {attempting && active && (
        <div className="cluster-home-status" role="status" aria-live="polite">
          <LoadingState
            message={connecting
              ? `Switching to ${active}…`
              : `Attempting connection to ${active}…`}
            compact
          />
        </div>
      )}

      {monitoringPaused && (
        <div className="cluster-home-callout">
          <p><strong>Monitoring paused</strong> for {active || 'this cluster'}. One click reconnects.</p>
          <ClusterFetchPanel
            loading={false}
            showActions
            onReconnect={onReconnect}
            onOpenProxySettings={onOpenProxySettings}
            reconnectBusy={reconnectBusy}
          />
        </div>
      )}

      {!monitoringPaused && disconnected && errorMessage && !attempting && active && (
        <div className="cluster-home-callout cluster-home-callout-error" role="alert">
          <p>
            <strong>{authFailed ? 'Authentication failed' : 'Disconnected'}</strong>
            {' · '}
            {active}
          </p>
          <p className="cluster-home-callout-detail">{errorMessage}</p>
          {classified.guidance && (
            <p className="cluster-home-callout-detail muted">{classified.guidance}</p>
          )}
          <ClusterFetchPanel
            loading={false}
            showActions
            onReconnect={onReconnect}
            onOpenProxySettings={onOpenProxySettings}
            reconnectBusy={reconnectBusy}
          />
        </div>
      )}

      {contexts.length === 0 ? (
        <p className="muted cluster-home-empty">
          No contexts in kubeconfig. Check your kubeconfig path in Settings.
        </p>
      ) : rows.length === 0 ? (
        <p className="muted cluster-home-empty">
          No contexts match this search{filtering ? ' or tag' : ''}.
        </p>
      ) : layout === 'tiles' ? (
        <ul
          className="klew-tiles"
          aria-label="Contexts"
          style={{ '--klew-tile-cols': Math.min(rows.length, rows.length > 6 ? 4 : 3) }}
        >
          {rows.map((c) => {
            const isActive = c.name === active
            const isDefault = c.name === defaultContext
            const isFavorite = isFavoriteContext(favoriteContexts, c.name)
            const detail = detailsForContext(contextDetails, c.name)
            const knownConnected = isConnectedContext(connectedContexts, c.name)
              || Boolean(detail?.version)
            const tags = tagsForContext(contextTags, c.name)
            const status = rowStatus({
              isActive,
              knownConnected,
              attempting,
              disconnected,
              syncError,
              paused: monitoringPaused,
              switching: connecting && isActive,
            })
            const tone = status?.tone === 'critical'
              ? 'crit'
              : status?.tone === 'warning'
                ? 'warn'
                : (status?.tone === 'ready' || knownConnected)
                  ? 'ok'
                  : 'muted'
            // Subtitle comes only from preferences — keeps the tile quiet until we learn facts.
            const meta = formatContextDetailMeta(detail, c.name)
            const label = displayContextName(contextAliases, c.name)
            const fullLabel = displayContextName(contextAliases, c.name, { truncate: false })
            const tipTitle = fullLabel !== c.name ? `${fullLabel} · ${c.name}` : c.name
            return (
              <li key={c.name} className={['klew-tile', isActive ? 'is-active' : ''].filter(Boolean).join(' ')}>
                <button
                  type="button"
                  className="klew-tile-main"
                  onClick={() => { if (!busy) onSelectCluster?.(c.name) }}
                  disabled={busy}
                  title={`Open ${tipTitle}`}
                >
                  <span className="klew-tile-head">
                    <ClusterCloudIcon tone={tone} size={20} className="klew-tile-cloud" />
                    <span className="klew-tile-name mono">{label}</span>
                  </span>
                  {meta ? <span className="klew-tile-meta">{meta}</span> : null}
                  <span className="klew-tile-foot">
                    {status ? (
                      <HoverTip label={statusTip(status).label} detail={statusTip(status).detail}>
                        <span className={`entity-table-status status-${status.tone}`}>
                          <span className="entity-table-status-led" aria-hidden="true" />
                          {status.label}
                        </span>
                      </HoverTip>
                    ) : (
                      <span className="klew-tile-foot-spacer" aria-hidden="true">&nbsp;</span>
                    )}
                  </span>
                </button>
                <span className="klew-tile-tags">
                  <ContextTagsEditor
                    tags={tags}
                    disabled={busy}
                    onAdd={onAddContextTag ? (tag) => onAddContextTag(c.name, tag) : undefined}
                    onRemove={onRemoveContextTag ? (tag) => onRemoveContextTag(c.name, tag) : undefined}
                  />
                </span>
                <span className="klew-tile-actions">
                  <ContextRecordingToggle contextName={c.name} className="klew-tile-record-btn" />
                  {onToggleFavoriteContext && (
                    <HoverTip
                      label={isFavorite ? 'Favorite' : 'Add favorite'}
                      detail={isFavorite
                        ? 'Shown first on Contexts. Click to remove.'
                        : 'Pin this context for quick access. Click to favorite.'}
                    >
                      <button
                        type="button"
                        className={['cluster-home-pin', isFavorite ? 'is-favorite' : ''].filter(Boolean).join(' ')}
                        aria-label={isFavorite ? `Unfavorite ${c.name}` : `Favorite ${c.name}`}
                        aria-pressed={isFavorite}
                        onClick={() => onToggleFavoriteContext(c.name)}
                        disabled={busy}
                      >
                        <FavoriteStarIcon filled={isFavorite} size={16} />
                      </button>
                    </HoverTip>
                  )}
                  {isDefault && (
                    <HoverTip
                      label="Launch default"
                      detail="Klew opens this cluster on launch instead of the Contexts page."
                    >
                      <span className="klew-tile-default">
                        <LaunchDefaultIcon active size={14} />
                      </span>
                    </HoverTip>
                  )}
                  <button
                    type="button"
                    className={['klew-contexts-more', menu?.name === c.name ? 'is-open' : ''].filter(Boolean).join(' ')}
                    aria-label={`More actions for ${c.name}`}
                    aria-haspopup="menu"
                    aria-expanded={menu?.name === c.name}
                    onClick={(e) => {
                      const r = e.currentTarget.getBoundingClientRect()
                      setMenu(menu?.name === c.name ? null : { name: c.name, isActive, isDefault, top: r.bottom + 4, right: window.innerWidth - r.right })
                    }}
                  >
                    <MoreIcon />
                  </button>
                </span>
              </li>
            )
          })}
        </ul>
      ) : (
        <div className="entity-table-scroll">
          <table className="entity-table-grid klew-contexts-grid">
            <thead>
              <tr>
                {COLUMNS.map((col) => {
                  const active = sort?.columnId === col.id
                  const ascending = active && sort?.direction === 'asc'
                  const descending = active && sort?.direction === 'desc'
                  return (
                    <th
                      key={col.id}
                      className={[
                        col.className,
                        col.sortable ? 'entity-table-sortable' : '',
                        active ? 'is-sorted' : '',
                      ].filter(Boolean).join(' ')}
                      scope="col"
                      aria-sort={col.sortable ? (active ? (ascending ? 'ascending' : 'descending') : 'none') : undefined}
                    >
                      {col.sortable ? (
                        <button
                          type="button"
                          className="entity-table-sort-btn"
                          onClick={() => onSort?.(col.id)}
                          title={`Sort by ${col.label}`}
                        >
                          <span className="entity-table-sort-label">{col.label}</span>
                          <span className="entity-table-sort-icons" aria-hidden="true">
                            <span className={['entity-table-sort-arrow', 'up', ascending ? 'active' : ''].filter(Boolean).join(' ')}>▲</span>
                            <span className={['entity-table-sort-arrow', 'down', descending ? 'active' : ''].filter(Boolean).join(' ')}>▼</span>
                          </span>
                        </button>
                      ) : col.label}
                    </th>
                  )
                })}
              </tr>
            </thead>
            <tbody>
              {rows.map((c) => {
                const isActive = c.name === active
                const isDefault = c.name === defaultContext
                const isFavorite = isFavoriteContext(favoriteContexts, c.name)
                const detail = detailsForContext(contextDetails, c.name)
                const knownConnected = isConnectedContext(connectedContexts, c.name)
                  || Boolean(detail?.version)
                const tags = tagsForContext(contextTags, c.name)
                const version = detail?.version || ''
                const clusterLabel = detail?.cluster || c.cluster || ''
                const status = rowStatus({
                  isActive,
                  knownConnected,
                  attempting,
                  disconnected,
                  syncError,
                  paused: monitoringPaused,
                  switching: connecting && isActive,
                })
                const tone = status?.tone === 'critical'
                  ? 'crit'
                  : status?.tone === 'warning'
                    ? 'warn'
                    : (status?.tone === 'ready' || knownConnected)
                      ? 'ok'
                      : 'muted'
                return (
                  <tr
                    key={c.name}
                    className={['entity-table-row', isActive ? 'selected' : ''].filter(Boolean).join(' ')}
                    onClick={() => {
                      if (!busy) onSelectCluster?.(c.name)
                    }}
                    onKeyDown={(e) => {
                      if (busy) return
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault()
                        onSelectCluster?.(c.name)
                      }
                    }}
                    tabIndex={0}
                    aria-selected={isActive}
                  >
                    <td className="col-name">
                      <span className="klew-contexts-name">
                        <ClusterCloudIcon tone={tone} size={18} className="cluster-home-row-cloud" />
                        <span
                          className="entity-table-name mono"
                          title={displayContextName(contextAliases, c.name, { truncate: false }) !== c.name
                            ? `${displayContextName(contextAliases, c.name, { truncate: false })} (${c.name})`
                            : c.name}
                        >
                          {displayContextName(contextAliases, c.name)}
                        </span>
                        {isFavorite && (
                          <FavoriteStarIcon filled size={13} className="cluster-home-fav-inline" />
                        )}
                        {isDefault && (
                          <span className="cluster-home-default-badge" title="Launch default on Home">
                            <LaunchDefaultIcon active size={13} />
                            <span className="sr-only">Launch default</span>
                          </span>
                        )}
                      </span>
                    </td>
                    <td className="col-cluster">
                      <span className="mono muted">{clusterLabel || '—'}</span>
                    </td>
                    <td className="col-version">
                      <span className="mono muted" title={version ? `Kubernetes ${version}` : 'Connect once to learn the cluster version'}>
                        {formatContextVersion(version)}
                      </span>
                    </td>
                    <td className="col-tags" onClick={(e) => e.stopPropagation()}>
                      <ContextTagsEditor
                        tags={tags}
                        disabled={busy}
                        onAdd={onAddContextTag ? (tag) => onAddContextTag(c.name, tag) : undefined}
                        onRemove={onRemoveContextTag ? (tag) => onRemoveContextTag(c.name, tag) : undefined}
                      />
                    </td>
                    <td className="col-history" onClick={(e) => e.stopPropagation()}>
                      <ContextRecordingToggle contextName={c.name} />
                    </td>
                    <td className="col-status">
                      {status ? (
                        <HoverTip label={statusTip(status).label} detail={statusTip(status).detail}>
                          <span className={`entity-table-status status-${status.tone}`}>
                            <span className="entity-table-status-led" aria-hidden="true" />
                            {status.label}
                          </span>
                        </HoverTip>
                      ) : (
                        <span className="muted">—</span>
                      )}
                    </td>
                    <td className="col-actions" onClick={(e) => e.stopPropagation()}>
                      <div className="klew-contexts-actions">
                        <HoverTip label={`Open ${c.name}`} detail="Switch to this cluster and open Resources.">
                          <button
                            type="button"
                            className="klew-contexts-open"
                            onClick={() => onSelectCluster?.(c.name)}
                            disabled={busy}
                          >
                            {connecting && isActive ? '…' : 'Open'}
                          </button>
                        </HoverTip>
                        {onToggleFavoriteContext && (
                          <HoverTip
                            label={isFavorite ? 'Favorite' : 'Add favorite'}
                            detail={isFavorite
                              ? 'Shown first on Contexts. Click to remove.'
                              : 'Pin this context for quick access.'}
                          >
                            <button
                              type="button"
                              className={['cluster-home-pin', isFavorite ? 'is-favorite' : ''].filter(Boolean).join(' ')}
                              aria-label={isFavorite ? `Unfavorite ${c.name}` : `Favorite ${c.name}`}
                              aria-pressed={isFavorite}
                              onClick={() => onToggleFavoriteContext(c.name)}
                              disabled={busy}
                            >
                              <FavoriteStarIcon filled={isFavorite} size={18} />
                            </button>
                          </HoverTip>
                        )}
                        {onSetDefaultContext && (
                          <HoverTip
                            label={isDefault ? 'Launch default' : 'Set launch default'}
                            detail={isDefault
                              ? 'Opens this cluster on launch. Click to clear.'
                              : 'Skip Contexts on launch and open this cluster.'}
                          >
                            <button
                              type="button"
                              className={['cluster-home-default-btn', isDefault ? 'is-default' : ''].filter(Boolean).join(' ')}
                              aria-label={isDefault ? `Clear ${c.name} as launch default` : `Set ${c.name} as launch default`}
                              aria-pressed={isDefault}
                              onClick={() => onSetDefaultContext(c.name)}
                              disabled={busy}
                            >
                              <LaunchDefaultIcon active={isDefault} size={18} />
                            </button>
                          </HoverTip>
                        )}
                        <button
                          type="button"
                          className={['klew-contexts-more', menu?.name === c.name ? 'is-open' : ''].filter(Boolean).join(' ')}
                          aria-label={`More actions for ${c.name}`}
                          aria-haspopup="menu"
                          aria-expanded={menu?.name === c.name}
                          onClick={(e) => {
                            const r = e.currentTarget.getBoundingClientRect()
                            setMenu(menu?.name === c.name ? null : { name: c.name, isActive, top: r.bottom + 4, right: window.innerWidth - r.right })
                          }}
                        >
                          <MoreIcon />
                        </button>
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
      <ContextRowMenu
        menu={menu}
        paused={monitoringPaused}
        busy={busy}
        alias={menu ? aliasForContext(contextAliases, menu.name) : ''}
        onOpen={onSelectCluster}
        onDisconnect={onDisconnect}
        onReconnect={onReconnect}
        onSetDefault={layout === 'tiles' ? onSetDefaultContext : undefined}
        onRename={onRenameContext}
        onClose={closeMenu}
      />
    </div>
  )
}

function MoreIcon() {
  return (
    <svg viewBox="0 0 16 16" width="18" height="18" fill="currentColor" aria-hidden="true">
      <circle cx="8" cy="3.5" r="1.3" />
      <circle cx="8" cy="8" r="1.3" />
      <circle cx="8" cy="12.5" r="1.3" />
    </svg>
  )
}

/**
 * Row ⋯ menu. Only the active context is connected, so Disconnect (pause live
 * watches) and Reconnect apply to it; other rows offer Open.
 */
function ContextRowMenu({
  menu,
  paused,
  busy,
  alias = '',
  onOpen,
  onDisconnect,
  onReconnect,
  onSetDefault,
  onRename,
  onClose,
}) {
  const ref = useRef(null)
  const inputRef = useRef(null)
  const [renaming, setRenaming] = useState(false)
  const [draft, setDraft] = useState('')

  useEffect(() => {
    if (!menu) {
      setRenaming(false)
      setDraft('')
      return undefined
    }
    function onDoc(e) {
      if (!ref.current?.contains(e.target) && !e.target.closest?.('.klew-contexts-more')) onClose()
    }
    function onKey(e) {
      if (e.key === 'Escape') {
        if (renaming) {
          setRenaming(false)
          return
        }
        onClose()
      }
    }
    document.addEventListener('mousedown', onDoc)
    document.addEventListener('keydown', onKey)
    window.addEventListener('resize', onClose)
    document.addEventListener('scroll', onClose, true)
    return () => {
      document.removeEventListener('mousedown', onDoc)
      document.removeEventListener('keydown', onKey)
      window.removeEventListener('resize', onClose)
      document.removeEventListener('scroll', onClose, true)
    }
  }, [menu, onClose, renaming])

  useEffect(() => {
    if (renaming) inputRef.current?.focus()
  }, [renaming])

  if (!menu) return null

  function startRename() {
    setDraft(alias || '')
    setRenaming(true)
  }

  function commitRename() {
    onRename?.(menu.name, draft)
    setRenaming(false)
    onClose()
  }

  const item = (label, action, { disabled = false, danger = false, hint = '' } = {}) => (
    <button
      type="button"
      role="menuitem"
      className={['klew-contexts-menu-item', danger ? 'is-danger' : ''].filter(Boolean).join(' ')}
      disabled={disabled}
      title={hint || undefined}
      onClick={() => { onClose(); action?.() }}
    >
      {label}
    </button>
  )
  return createPortal(
    <div
      ref={ref}
      className="klew-contexts-menu"
      role="menu"
      aria-label={`${menu.name} actions`}
      style={{ top: menu.top, right: Math.max(8, menu.right) }}
    >
      {renaming ? (
        <div className="klew-contexts-rename" onClick={(e) => e.stopPropagation()}>
          <label className="klew-contexts-rename-label" htmlFor={`klew-rename-${menu.name}`}>
            Display name
          </label>
          <input
            id={`klew-rename-${menu.name}`}
            ref={inputRef}
            type="text"
            className="klew-contexts-rename-input"
            value={draft}
            maxLength={MAX_CONTEXT_ALIAS_LEN}
            placeholder={menu.name}
            spellCheck={false}
            autoComplete="off"
            aria-label={`Display name for ${menu.name}`}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault()
                commitRename()
              } else if (e.key === 'Escape') {
                e.preventDefault()
                setRenaming(false)
              }
            }}
          />
          <p className="klew-contexts-rename-hint muted">
            Shown in tabs and tiles. Leave empty to use the real context name.
          </p>
          <div className="klew-contexts-rename-actions">
            <button type="button" className="klew-contexts-menu-item" onClick={() => setRenaming(false)}>
              Cancel
            </button>
            <button type="button" className="klew-contexts-menu-item is-primary" onClick={commitRename}>
              Save
            </button>
          </div>
        </div>
      ) : (
        <>
          {!menu.isActive && item('Open', () => onOpen?.(menu.name), { disabled: busy })}
          {onRename && (
            <button
              type="button"
              role="menuitem"
              className="klew-contexts-menu-item"
              disabled={busy}
              title={alias
                ? `Currently “${alias}”. Clears back to ${menu.name} if left empty.`
                : `Short label for ${menu.name}`}
              onClick={startRename}
            >
              Rename…
            </button>
          )}
          {menu.isActive && paused && item('Reconnect', onReconnect, { disabled: busy || !onReconnect })}
          {onSetDefault && menu.isDefault != null && item(
            menu.isDefault ? 'Clear launch default' : 'Set as launch default (skip Contexts)',
            () => onSetDefault(menu.name),
            { disabled: busy },
          )}
          {menu.isActive && !paused && item('Disconnect', onDisconnect, {
            disabled: !onDisconnect,
            danger: true,
            hint: 'Stop live watches and polling for this cluster. Nothing on the cluster changes.',
          })}
        </>
      )}
    </div>,
    document.body,
  )
}

/**
 * Connection is sticky per context: once connected it stays Connected when you
 * switch away. Live Connecting / Disconnected / Paused only apply to the active
 * context. Periodic rechecks can clear sticky state later.
 */
function rowStatus({
  isActive,
  knownConnected = false,
  attempting,
  disconnected,
  syncError,
  paused,
  switching = false,
}) {
  if (isActive) {
    if (paused) return { label: 'Paused', tone: 'unknown' }
    if (switching || attempting) return { label: switching ? 'Switching…' : 'Connecting', tone: 'warning' }
    if (disconnected || syncError) return { label: 'Disconnected', tone: 'critical' }
    return { label: 'Connected', tone: 'ready' }
  }
  if (knownConnected) return { label: 'Connected', tone: 'ready' }
  return null
}

function statusTip(status) {
  switch (status?.label) {
    case 'Connected':
      return {
        label: 'Connected',
        detail: 'Reached successfully. Stays Connected when you switch to another context.',
      }
    case 'Switching…':
      return { label: 'Switching context', detail: 'Loading this cluster’s kubeconfig and API.' }
    case 'Connecting':
      return { label: 'Connecting', detail: 'Checking reachability for this cluster.' }
    case 'Disconnected':
      return { label: 'Disconnected', detail: 'API unreachable right now. Reconnect from the menu or top bar.' }
    case 'Paused':
      return { label: 'Monitoring paused', detail: 'Live watches are stopped until you reconnect.' }
    default:
      return { label: status?.label || '', detail: '' }
  }
}
