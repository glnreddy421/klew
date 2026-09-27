import { ClusterCloudIcon } from './shell/ClusterCloudIcon.jsx'
import { ClusterFetchPanel } from './ClusterFetchPanel.jsx'
import { LoadingState } from './LoadingSpinner.jsx'
import { connectionErrorMessage } from '../lib/clusterConnection.js'
import { isFavoriteContext, orderContextsForDisplay } from '../lib/clusterContexts.js'
import { FavoriteStarIcon, LaunchDefaultIcon } from './shell/ContextActionIcons.jsx'

/**
 * Local kubeconfig context list for Home.
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
  onSelectCluster,
  onSetDefaultContext,
  onToggleFavoriteContext,
  onReconnect,
  onOpenProxySettings,
}) {
  const contexts = cluster?.contexts || []
  const displayContexts = orderContextsForDisplay(contexts, { favoriteContexts })
  const favoriteCount = favoriteContexts?.length || 0
  const active = cluster?.selectedContext || cluster?.currentContext || ''
  const syncError = String(cluster?.syncError || '').trim()
  const attempting = !monitoringPaused && (syncing || connecting || statusLoading)
  const phase = connection?.phase
  const disconnected = phase === 'disconnected' || phase === 'retrying' || Boolean(syncError)
  const errorMessage = syncError || (disconnected ? connectionErrorMessage(cluster, null) : '')

  return (
    <div className="cluster-home-list">
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
          <p>Monitoring is paused. Reconnect to sync this cluster.</p>
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
          <p>{errorMessage}</p>
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
      ) : (
        <>
          {favoriteCount > 0 && (
            <p className="cluster-home-section-label muted">Favorites first — open any context anytime.</p>
          )}
          <ul className="cluster-home-rows">
            {displayContexts.map((c) => {
              const isActive = c.name === active
              const isDefault = c.name === defaultContext
              const isFavorite = isFavoriteContext(favoriteContexts, c.name)
              const tone = isActive
                ? (disconnected || syncError ? 'crit' : (attempting ? 'warn' : 'ok'))
                : 'muted'
              return (
                <li
                  key={c.name}
                  className={['cluster-home-row', isActive ? 'is-active' : ''].filter(Boolean).join(' ')}
                >
                  <button
                    type="button"
                    className="cluster-home-row-open"
                    onClick={() => onSelectCluster?.(c.name)}
                    disabled={connecting || reconnectBusy}
                  >
                    <ClusterCloudIcon tone={tone} size={18} className="cluster-home-row-cloud" />
                    <span className="cluster-home-row-copy">
                      <span className="cluster-home-row-name mono">
                        {isFavorite && (
                          <FavoriteStarIcon filled size={12} className="cluster-home-fav-inline" />
                        )}
                        {c.name}
                        {isDefault && (
                          <span className="cluster-home-default-badge" title="Launch default on Home">
                            <LaunchDefaultIcon active size={11} />
                            <span className="sr-only">Launch default</span>
                          </span>
                        )}
                        {isActive && !attempting && !disconnected && !syncError && (
                          <span className="cluster-home-active-badge">Connected</span>
                        )}
                      </span>
                      {(c.cluster || c.user || c.namespace) && (
                        <span className="cluster-home-row-meta muted">
                          {[c.cluster, c.user, c.namespace && `ns: ${c.namespace}`]
                            .filter(Boolean)
                            .join(' · ')}
                        </span>
                      )}
                    </span>
                    <span className="cluster-home-row-cta">
                      {connecting && isActive ? '…' : 'Open'}
                    </span>
                  </button>
                  <div className="cluster-home-row-actions">
                    {onToggleFavoriteContext && (
                      <button
                        type="button"
                        className={['cluster-home-pin', isFavorite ? 'is-favorite' : ''].filter(Boolean).join(' ')}
                        title={isFavorite ? 'Remove from favorites' : 'Add to favorites'}
                        aria-label={isFavorite ? `Unfavorite ${c.name}` : `Favorite ${c.name}`}
                        aria-pressed={isFavorite}
                        onClick={(e) => {
                          e.stopPropagation()
                          onToggleFavoriteContext(c.name)
                        }}
                        disabled={connecting || reconnectBusy}
                      >
                        <FavoriteStarIcon filled={isFavorite} size={15} />
                      </button>
                    )}
                    {onSetDefaultContext && (
                      <button
                        type="button"
                        className={['cluster-home-default-btn', isDefault ? 'is-default' : ''].filter(Boolean).join(' ')}
                        title={isDefault ? 'Clear launch default' : 'Default context on Home (does not lock switching)'}
                        aria-label={isDefault ? `Clear ${c.name} as launch default` : `Set ${c.name} as launch default`}
                        aria-pressed={isDefault}
                        onClick={(e) => {
                          e.stopPropagation()
                          onSetDefaultContext(c.name)
                        }}
                        disabled={connecting || reconnectBusy}
                      >
                        <LaunchDefaultIcon active={isDefault} size={15} />
                      </button>
                    )}
                  </div>
                </li>
              )
            })}
          </ul>
        </>
      )}
    </div>
  )
}
