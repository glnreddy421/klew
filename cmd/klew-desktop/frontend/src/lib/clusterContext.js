import { getSnapshot, getState, podHealthLabel } from './investigationViews.js'
import { formatVersionChipLabel, formatVersionTooltip } from './clusterVersion.js'

/**
 * Overview cluster / scope context — GetCluster, GetClusterStatus, investigation snapshot.
 */

export function buildClusterContext(
  cluster,
  view,
  { running = false, syncing = false, clusterStatus = null } = {},
) {
  const snap = getSnapshot(view)
  const state = getState(view)
  const pods = snap.pods || []
  const nodes = snap.nodes || []
  const metrics = snap.metrics || state.metrics || {}
  const perms = state.permissions?.length ? state.permissions : (snap.permissions || [])

  const context = cluster?.selectedContext || cluster?.currentContext || ''
  const clusterName = cluster?.cluster || ''
  const namespace = cluster?.selectedNamespace || ''
  const query = String(view?.summary?.query || state.query || '').trim()

  const sync = buildSyncState(cluster, syncing)
  const podStats = summarizePods(pods)
  const nodeStats = summarizeNodes(nodes)
  const metricsAvailable = metrics.available === true
  const denied = perms.filter((p) => p && p.allowed === false)
  const rbacLimited = denied.length > 0

  const chips = buildChips({
    sync,
    podStats,
    nodeStats,
    metricsAvailable,
    rbacLimited,
    deniedCount: denied.length,
    running,
    hasScope: pods.length > 0 || query.length > 0,
    clusterStatus,
  })

  return {
    context,
    clusterName,
    namespace,
    query,
    running,
    syncing,
    sync,
    clusterStatus,
    pods: podStats,
    nodes: nodeStats,
    metricsAvailable,
    metricsNote: metrics.note || '',
    rbacLimited,
    deniedCount: denied.length,
    chips,
    showBar: Boolean(
      context
      || namespace
      || running
      || pods.length > 0
      || clusterStatus?.available,
    ),
  }
}

function buildSyncState(cluster, syncing) {
  if (syncing) {
    return { status: 'pending', label: 'Syncing…', title: 'Refreshing cluster connection' }
  }
  if (!cluster) {
    return { status: 'unknown', label: 'Not connected', title: '' }
  }
  if (cluster.syncError) {
    return {
      status: 'error',
      label: 'Sync error',
      title: cluster.syncError,
      action: 'settings',
    }
  }
  if (cluster.syncedAt) {
    return {
      status: 'ok',
      label: `Synced ${formatSyncAge(cluster.syncedAt)}`,
      title: `Last synced ${formatSyncClock(cluster.syncedAt)}`,
    }
  }
  return { status: 'unknown', label: 'Not synced', title: '' }
}

function summarizePods(pods) {
  let ready = 0
  let failing = 0
  for (const p of pods) {
    if (p.ready) ready += 1
    if (podHealthLabel(p) !== 'healthy') failing += 1
  }
  return { total: pods.length, ready, failing }
}

function summarizeNodes(nodes) {
  let ready = 0
  let pressured = 0
  for (const n of nodes) {
    if (n.ready) ready += 1
    if (n.memoryPressure || n.diskPressure || n.pidPressure || n.unschedulable) {
      pressured += 1
    }
  }
  return { total: nodes.length, ready, pressured }
}

function buildChips({
  sync,
  podStats,
  nodeStats,
  metricsAvailable,
  rbacLimited,
  deniedCount,
  running,
  hasScope,
  clusterStatus,
}) {
  const chips = []

  chips.push({
    id: 'sync',
    label: sync.label,
    tone: sync.status === 'error' ? 'crit' : sync.status === 'ok' ? 'ok' : 'muted',
    title: sync.title || sync.label,
    action: sync.action,
  })

  appendClusterStatusChips(chips, clusterStatus)

  if (running && hasScope && podStats.total > 0) {
    chips.push({
      id: 'pods',
      label: podStats.failing > 0
        ? `${podStats.ready}/${podStats.total} pods ready · ${podStats.failing} failing`
        : `${podStats.ready}/${podStats.total} pods in scope`,
      tone: podStats.failing > 0 ? 'warn' : 'ok',
      navTab: podStats.failing > 0 ? 'failures' : 'resources',
      title: 'Pods in investigation scope',
      scope: true,
    })
  } else if (running && hasScope) {
    chips.push({
      id: 'scope',
      label: 'Scope loaded',
      tone: 'muted',
      navTab: 'resources',
      title: 'Browse resources in scope',
      scope: true,
    })
  }

  if (running && nodeStats.total > 0) {
    chips.push({
      id: 'nodes-scope',
      label: nodeStats.pressured > 0
        ? `${nodeStats.ready}/${nodeStats.total} scoped nodes · ${nodeStats.pressured} pressured`
        : `${nodeStats.ready}/${nodeStats.total} scoped nodes`,
      tone: nodeStats.pressured > 0 ? 'warn' : 'muted',
      navTab: 'nodes',
      nodesMode: 'scope',
      title: 'Nodes hosting scoped workloads',
      scope: true,
    })
  }

  if (running) {
    chips.push({
      id: 'metrics',
      label: metricsAvailable ? 'Metrics available' : 'Metrics unavailable',
      tone: metricsAvailable ? 'ok' : 'muted',
      title: metricsAvailable
        ? 'metrics-server data included in scope'
        : 'metrics-server not available for this scope',
      scope: true,
    })
  }

  if (rbacLimited) {
    chips.push({
      id: 'rbac',
      label: deniedCount === 1 ? '1 RBAC gap' : `${deniedCount} RBAC gaps`,
      tone: 'warn',
      title: 'Some resource types are not visible with the current identity',
      action: 'visibility',
    })
  }

  return chips
}

function appendClusterStatusChips(chips, clusterStatus) {
  if (!clusterStatus) return

  if (clusterStatus.available && clusterStatus.apiReachable) {
    const versionLabel = formatVersionChipLabel(clusterStatus)
    if (versionLabel) {
      chips.push({
        id: 'k8s-version',
        label: versionLabel,
        tone: clusterStatus.versions?.skewed ? 'warn' : 'muted',
        title: formatVersionTooltip(clusterStatus),
        clusterWide: true,
      })
    }

    const nodes = clusterStatus.nodes || {}
    if (nodes.total > 0) {
      const label = nodes.notReady > 0
        ? `${nodes.ready}/${nodes.total} cluster nodes · ${nodes.notReady} not ready`
        : `${nodes.ready}/${nodes.total} cluster nodes`
      chips.push({
        id: 'cluster-nodes',
        label,
        tone: nodes.notReady > 0 || nodes.pressured > 0 ? 'warn' : 'ok',
        title: nodes.pressured > 0
          ? `${nodes.pressured} node(s) report pressure or are unschedulable`
          : 'Cluster-wide node inventory',
        navTab: 'nodes',
        nodesMode: 'cluster',
        clusterWide: true,
      })
    } else if (clusterStatus.namespaceCount > 0) {
      chips.push({
        id: 'namespaces',
        label: `${clusterStatus.namespaceCount} namespaces`,
        tone: 'muted',
        title: 'Namespaces visible to the current identity',
        clusterWide: true,
      })
    }
    return
  }

  if (clusterStatus.error) {
    chips.push({
      id: 'api',
      label: 'API unreachable',
      tone: 'crit',
      title: clusterStatus.error,
      action: 'settings',
      clusterWide: true,
    })
  }
}

function formatSyncAge(ts) {
  const t = new Date(ts).getTime()
  if (!Number.isFinite(t)) return ''
  const sec = Math.max(0, Math.floor((Date.now() - t) / 1000))
  if (sec < 60) return 'just now'
  if (sec < 3600) return `${Math.floor(sec / 60)}m ago`
  return `${Math.floor(sec / 3600)}h ago`
}

function formatSyncClock(ts) {
  try {
    return new Date(ts).toLocaleString(undefined, {
      hour: '2-digit',
      minute: '2-digit',
      month: 'short',
      day: 'numeric',
    })
  } catch {
    return String(ts)
  }
}
