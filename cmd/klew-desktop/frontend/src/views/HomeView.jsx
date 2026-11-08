import { CollectingMatchesSplash } from '../components/incident/CollectingMatchesSplash.jsx'
import { ClusterHomeList } from '../components/ClusterHomeList.jsx'

/**
 * Klew home — orbital hero plus local kubeconfig contexts.
 * Choosing a cluster opens Resources for that context.
 */
export function HomeView({
  cluster,
  clusterStatus,
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
  onOpenMcpSetup,
}) {
  return (
    <div className="klew-home">
      <section className="klew-home-hero welcome welcome-orbit" aria-label="Klew">
        <CollectingMatchesSplash variant="home" />
        {onOpenMcpSetup && (
          <div className="klew-home-mcp">
            <span className="klew-home-mcp-icon" aria-hidden="true">
              <svg viewBox="0 0 16 16" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                <path d="M6 2.5v3M10 2.5v3" />
                <path d="M4 5.5h8v2.25a4 4 0 0 1-8 0V5.5z" />
                <path d="M8 11.75v1.75" />
              </svg>
            </span>
            <div className="klew-home-mcp-text">
              <strong>Use Klew from AI assistants</strong>
              <span className="muted">Read-only MCP server for Claude, Cursor and others.</span>
            </div>
            <button type="button" className="btn btn-sm" onClick={onOpenMcpSetup}>Set up MCP</button>
          </div>
        )}
      </section>

      <section className="klew-home-clusters" aria-labelledby="klew-home-clusters-title">
        <header className="klew-home-clusters-head">
          <h2 id="klew-home-clusters-title">Local clusters</h2>
          <p className="klew-home-clusters-lead muted">
            Contexts from your kubeconfig. Star favorites, set an optional launch default,
            and open any cluster — switching never locks you to one context.
          </p>
        </header>

        <div className="cluster-home-scroll">
          <ClusterHomeList
            cluster={cluster}
            clusterStatus={clusterStatus}
            connection={connection}
            syncing={syncing}
            connecting={connecting}
            statusLoading={statusLoading}
            reconnectBusy={reconnectBusy}
            monitoringPaused={monitoringPaused}
            defaultContext={defaultContext}
            favoriteContexts={favoriteContexts}
            onSelectCluster={onSelectCluster}
            onSetDefaultContext={onSetDefaultContext}
            onToggleFavoriteContext={onToggleFavoriteContext}
            onReconnect={onReconnect}
            onOpenProxySettings={onOpenProxySettings}
          />
        </div>
      </section>
    </div>
  )
}

