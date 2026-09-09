import { getKindCountDisplay } from './resourceCatalog.js'
import { blastRadiusCounts, fmtCpu, fmtMem, getSnapshot, podHealthLabel, utilPct } from './investigationViews.js'

/** Primary workload kinds shown in the overview grid (Lens order, minus ReplicaSet). */
export const WORKLOAD_OVERVIEW_KINDS = [
  'Pod',
  'Deployment',
  'StatefulSet',
  'DaemonSet',
  'Job',
  'CronJob',
]

function kindCount(kindGroup) {
  const display = getKindCountDisplay(kindGroup)
  if (display.label && display.className !== 'count-unknown') {
    const n = Number(display.label)
    if (Number.isFinite(n)) return n
  }
  return kindGroup?.count ?? 0
}

function summarizePods(pods) {
  let running = 0
  let pending = 0
  let failing = 0
  let healthy = 0
  for (const pod of pods) {
    if (!pod) continue
    const health = podHealthLabel(pod)
    const phase = String(pod.phase || '').toLowerCase()
    if (health === 'critical') failing += 1
    else if (health === 'warning' || phase === 'pending') pending += 1
    else if (health === 'healthy' || phase === 'running' || phase === 'succeeded') {
      running += 1
      healthy += 1
    } else pending += 1
  }
  return {
    total: pods.length,
    running,
    pending,
    failing,
    healthy,
  }
}

function summarizeReplicas(workloads) {
  let ready = 0
  let desired = 0
  for (const w of workloads) {
    if (!w) continue
    ready += w.ready ?? w.readyReplicas ?? 0
    desired += w.replicas ?? w.desired ?? w.desiredReplicas ?? 0
  }
  return { ready, desired }
}

function summarizeResources(metrics) {
  const m = metrics || {}
  const cpuDenom = Math.max(m.cpuLimitMillicores || 0, m.cpuRequestMillicores || 0, 1)
  const memDenom = Math.max(m.memLimitMi || 0, m.memRequestMi || 0, 1)
  return {
    available: Boolean(m.available),
    note: m.note || '',
    cpu: {
      usage: m.cpuUsageMillicores,
      request: m.cpuRequestMillicores,
      limit: m.cpuLimitMillicores,
      pct: utilPct(m.cpuUsageMillicores, cpuDenom),
      label: fmtCpu(m.cpuUsageMillicores),
      denomLabel: fmtCpu(cpuDenom),
    },
    memory: {
      usage: m.memUsageMi,
      request: m.memRequestMi,
      limit: m.memLimitMi,
      pct: utilPct(m.memUsageMi, memDenom),
      label: fmtMem(m.memUsageMi),
      denomLabel: fmtMem(memDenom),
    },
  }
}

function overallHealthTone({ pods, blast, nodes }) {
  if (pods.failing > 0 || blast.critical > 0) return 'crit'
  if (pods.pending > 0 || blast.warning > 0 || (nodes?.notReady ?? 0) > 0) return 'warn'
  if (pods.total > 0 || (nodes?.total ?? 0) > 0) return 'ok'
  return 'muted'
}

/**
 * Build workload overview metrics from catalog tree, investigation snapshot, and cluster status.
 */
export function buildWorkloadMetrics({ tree, view, clusterStatus }) {
  const snap = getSnapshot(view || {})
  const workloadsCat = tree?.categories?.find((c) => c.id === 'workloads')
  if (!workloadsCat?.kinds?.length) {
    return emptyWorkloadMetrics()
  }

  const kindByName = new Map(workloadsCat.kinds.map((k) => [k.kind, k]))

  const kindTiles = WORKLOAD_OVERVIEW_KINDS
    .map((kind) => {
      const kindGroup = kindByName.get(kind)
      if (!kindGroup) return null
      return {
        kind,
        label: kindGroup.label || kind,
        count: kindCount(kindGroup),
        groupId: workloadsCat.id,
        resourceId: kindGroup.resourceId,
        denied: kindGroup.accessState === 'forbidden',
      }
    })
    .filter(Boolean)

  const pods = summarizePods(snap.pods || [])
  const replicas = summarizeReplicas(snap.workloads || [])
  const resources = summarizeResources(snap.metrics)
  const blast = blastRadiusCounts(snap)
  const nodes = clusterStatus?.nodes || { total: 0, ready: 0, notReady: 0, pressured: 0 }

  const podHealthPct = pods.total > 0
    ? Math.round((pods.healthy / pods.total) * 100)
    : null
  const replicaPct = replicas.desired > 0
    ? Math.round((replicas.ready / replicas.desired) * 100)
    : null

  const totalWorkloadCount = kindTiles.reduce((n, t) => n + t.count, 0)
  const hasInvestigation = pods.total > 0 || (snap.workloads || []).length > 0

  return {
    kindTiles,
    totalWorkloadCount,
    pods,
    replicas,
    resources,
    blast,
    nodes,
    podHealthPct,
    replicaPct,
    hasInvestigation,
    healthTone: overallHealthTone({ pods, blast, nodes }),
    healthLabel: healthLabelFor({ pods, blast, nodes, hasInvestigation }),
  }
}

function emptyWorkloadMetrics() {
  return {
    kindTiles: [],
    totalWorkloadCount: 0,
    pods: { total: 0, running: 0, pending: 0, failing: 0, healthy: 0 },
    replicas: { ready: 0, desired: 0 },
    resources: summarizeResources(null),
    blast: { critical: 0, warning: 0, healthy: 0, pods: 0, podsFailing: 0 },
    nodes: { total: 0, ready: 0, notReady: 0, pressured: 0 },
    podHealthPct: null,
    replicaPct: null,
    hasInvestigation: false,
    healthTone: 'muted',
    healthLabel: 'All systems normal',
  }
}

function healthLabelFor({ pods, blast, nodes, hasInvestigation }) {
  if (pods.failing > 0) return `${pods.failing} pod${pods.failing === 1 ? '' : 's'} failing`
  if ((nodes.notReady ?? 0) > 0) return `${nodes.notReady} node${nodes.notReady === 1 ? '' : 's'} not ready`
  if (blast.warning > 0) return `${blast.warning} warning${blast.warning === 1 ? '' : 's'}`
  if (hasInvestigation && pods.total > 0) return `${pods.healthy}/${pods.total} pods healthy`
  if ((nodes.ready ?? 0) > 0) return `${nodes.ready}/${nodes.total} nodes ready`
  return 'All systems normal'
}
