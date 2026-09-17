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
            onSelectCluster={onSelectCluster}
            onSetDefaultContext={onSetDefaultContext}
            onReconnect={onReconnect}
            onOpenProxySettings={onOpenProxySettings}
          />
        </div>

        <footer className="klew-home-footer">
          <button
            type="button"
            className="klew-home-settings-btn"
            onClick={onOpenSettings}
            title="Kubernetes settings"
            aria-label="Kubernetes settings"
          >
            <KubernetesSettingsIcon />
          </button>
        </footer>
      </section>
    </div>
  )
}

function KubernetesSettingsIcon() {
  return (
    <svg
      viewBox="0 0 24 24"
      width="18"
      height="18"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.75"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M12 3.5 18.5 7v10L12 20.5 5.5 17V7L12 3.5z" />
      <circle cx="12" cy="12" r="2.25" />
      <path d="M12 9.75V6.5M12 17.5v-3.25M14.25 10.5l2.83-1.63M9.75 13.5l-2.83 1.63M14.25 13.5l2.83 1.63M9.75 10.5l-2.83-1.63" />
    </svg>
  )
}
