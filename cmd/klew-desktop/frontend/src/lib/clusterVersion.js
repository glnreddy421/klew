/**
 * Cluster version formatting for Overview + Nodes surfaces.
 * Run: node --test src/lib/clusterVersion.test.js
 */

export function formatVersionChipLabel(clusterStatus) {
  const summary = clusterStatus?.versions
  const api = summary?.apiServer || clusterStatus?.kubernetesVersion || ''
  if (!summary) {
    return api ? (api.startsWith('v') ? `K8s ${api}` : api) : ''
  }

  if (!summary.skewed) {
    const unified = summary.controlPlane?.label || summary.workers?.label || api
    return unified ? `K8s ${shortVersion(unified)}` : (api ? `K8s ${shortVersion(api)}` : '')
  }

  const parts = []
  if (api) parts.push(`API ${shortVersion(api)}`)
  if (summary.controlPlane?.count > 0) {
    parts.push(`CP ${shortVersion(summary.controlPlane.label)}`)
  }
  if (summary.workers?.count > 0) {
    parts.push(`workers ${shortVersion(summary.workers.label)}`)
  }
  return parts.join(' · ')
}

export function formatVersionTooltip(clusterStatus) {
  const summary = clusterStatus?.versions
  if (!summary) return formatLegacyVersionTitle(clusterStatus)

  const lines = []
  if (summary.apiServer) {
    lines.push(`API server: ${summary.apiServer}`)
  }
  if (summary.controlPlane?.count > 0) {
    lines.push(`Control plane (${summary.controlPlane.count}): ${formatGroupDetail(summary.controlPlane)}`)
  }
  if (summary.workers?.count > 0) {
    lines.push(`Workers (${summary.workers.count}): ${formatGroupDetail(summary.workers)}`)
  }
  if (clusterStatus?.platform) lines.push(clusterStatus.platform)
  return lines.join('\n') || 'Cluster version'
}

export function formatNodesVersionLead(clusterStatus) {
  const summary = clusterStatus?.versions
  const nodes = clusterStatus?.nodes
  const ready = nodes?.total > 0 ? `${nodes.ready}/${nodes.total} ready` : ''

  const versionPart = formatVersionChipLabel(clusterStatus)
  if (!versionPart && !ready) return ''
  if (!ready) return versionPart
  if (!versionPart) return ready
  return `${versionPart} · ${ready}`
}

function formatGroupDetail(group) {
  if (!group?.versions || Object.keys(group.versions).length <= 1) {
    return group?.label || '—'
  }
  return Object.entries(group.versions)
    .sort((a, b) => compareVersion(a[0], b[0]))
    .map(([version, count]) => `${version} (${count})`)
    .join(', ')
}

function formatLegacyVersionTitle(clusterStatus) {
  const parts = []
  if (clusterStatus?.kubernetesVersion) {
    parts.push(`Kubernetes ${clusterStatus.kubernetesVersion}`)
  }
  if (clusterStatus?.platform) parts.push(clusterStatus.platform)
  return parts.join(' · ') || 'Cluster version'
}

function shortVersion(version) {
  const v = String(version || '').trim()
  if (!v) return v
  return v.startsWith('v') ? v : v
}

function compareVersion(a, b) {
  return String(a).localeCompare(String(b), undefined, { numeric: true })
}
