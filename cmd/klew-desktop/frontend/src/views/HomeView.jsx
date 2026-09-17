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
  onSelectCluster,
  onSetDefaultContext,
  onReconnect,
  onOpenProxySettings,
  onOpenSettings,
}) {
  return (
    <div className="klew-home">
      <section className="klew-home-hero welcome welcome-orbit" aria-label="Klew">
        <CollectingMatchesSplash variant="home" />
      </section>

      <section className="klew-home-clusters" aria-labelledby="klew-home-clusters-title">
        <header className="klew-home-clusters-head">
          <h2 id="klew-home-clusters-title">Local clusters</h2>
          <p className="klew-home-clusters-lead muted">
            Contexts from your kubeconfig on this machine. Open one to connect and browse
            resources, or pin a default cluster for next time.
          </p>
        </header>

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
          onSelectCluster={onSelectCluster}
          onSetDefaultContext={onSetDefaultContext}
          onReconnect={onReconnect}
          onOpenProxySettings={onOpenProxySettings}
        />

        <footer className="klew-home-footer">
          <button type="button" className="text-link-btn" onClick={onOpenSettings}>
            Kubernetes settings
          </button>
        </footer>
      </section>
    </div>
  )
}
