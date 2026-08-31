/** Format pod creation timestamp as a short relative age. */
export function formatEntityAge(createdAt) {
  if (!createdAt) return '—'
  const raw = typeof createdAt === 'object' ? createdAt.time || createdAt.Time : createdAt
  const ms = Date.parse(raw)
  if (!Number.isFinite(ms)) return '—'
  const sec = Math.max(0, Math.floor((Date.now() - ms) / 1000))
  if (sec < 60) return `${sec}s`
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min}m`
  const hr = Math.floor(min / 60)
  if (hr < 48) return `${hr}h`
  const day = Math.floor(hr / 24)
  return `${day}d`
}

function firstNonEmpty(...values) {
  for (const v of values) {
    if (v != null && v !== '' && v !== '—') return v
  }
  return '—'
}

function podForRow(row, pods) {
  if (!row?.name || !pods?.length) return null
  if (row.kind === 'Pod') {
    return pods.find((p) => p.name === row.name) || null
  }
  return null
}

function aggregateContainerResource(containers, field) {
  if (!containers?.length) return '—'
  const values = containers.map((c) => c[field]).filter(Boolean)
  if (!values.length) return '—'
  if (values.length === 1) return values[0]
  return values.join(', ')
}

export function statusLabelForRow(row, pod) {
  if (pod?.phase) return pod.phase
  if (row.ready != null && row.total != null) {
    return row.ready === row.total ? 'Ready' : `${row.ready}/${row.total}`
  }
  const map = {
    healthy: 'Running',
    degraded: 'Degraded',
    critical: 'Failed',
    unknown: 'Unknown',
  }
  return map[row.status] || row.signal || '—'
}

/** Merge snapshot pod fields into entity rows for table columns. */
export function enrichEntityForTable(row, pods = []) {
  const pod = podForRow(row, pods)
  const namespace = row.namespace || row.ref?.namespace || pod?.namespace || '—'
  return {
    ...row,
    table: {
      namespace,
      node: pod?.node || row.node || '—',
      age: formatEntityAge(pod?.createdAt),
      restarts: row.restarts ?? pod?.restartCount ?? '—',
      cpu: aggregateContainerResource(pod?.containers, 'limitsCPU')
        || aggregateContainerResource(pod?.containers, 'requestsCPU'),
      memory: aggregateContainerResource(pod?.containers, 'limitsMem')
        || aggregateContainerResource(pod?.containers, 'requestsMem'),
      statusLabel: statusLabelForRow(row, pod),
    },
  }
}

export function enrichEntitiesForTable(entities, pods = []) {
  return (entities || []).map((row) => enrichEntityForTable(row, pods))
}

export const TABLE_COLUMNS = {
  standard: [
    { id: 'name', label: 'Name', className: 'col-name' },
    { id: 'status', label: 'Status', className: 'col-status' },
    { id: 'namespace', label: 'Namespace', className: 'col-ns' },
    { id: 'node', label: 'Node', className: 'col-node' },
    { id: 'age', label: 'Age', className: 'col-age' },
  ],
  dense: [
    { id: 'name', label: 'Name', className: 'col-name' },
    { id: 'status', label: 'Status', className: 'col-status' },
    { id: 'namespace', label: 'Namespace', className: 'col-ns' },
    { id: 'node', label: 'Node', className: 'col-node' },
    { id: 'restarts', label: 'Restarts', className: 'col-num' },
    { id: 'cpu', label: 'CPU', className: 'col-metric' },
    { id: 'memory', label: 'Memory', className: 'col-metric' },
    { id: 'age', label: 'Age', className: 'col-age' },
  ],
}

export function tableColumnsForDensity(density) {
  return density === 'dense' ? TABLE_COLUMNS.dense : TABLE_COLUMNS.standard
}
