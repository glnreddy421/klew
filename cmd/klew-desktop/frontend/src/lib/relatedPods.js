import { buildInspectKey } from './matches.js'

const WORKLOAD_KINDS = new Set([
  'Deployment', 'ReplicaSet', 'StatefulSet', 'DaemonSet', 'Job', 'CronJob',
])

function isPodListSection(section) {
  const id = String(section?.id || '').toLowerCase()
  const title = String(section?.title || '').toLowerCase()
  return id === 'pods'
    || id === 'targetpods'
    || title === 'pods'
    || title === 'target pods'
}

function readyFromCell(value) {
  const raw = String(value ?? '').trim().toLowerCase()
  if (raw === 'true') return 1
  if (raw === 'false') return 0
  return 0
}

function statusFromPhase(phase, ready) {
  const p = String(phase || '').toLowerCase()
  if (p === 'failed' || p === 'error') return 'critical'
  if (p === 'pending' || ready < 1) return 'degraded'
  if (p === 'running' || p === 'succeeded') return 'healthy'
  return 'unknown'
}

function podFromTableRow(row, namespace) {
  const name = String(row?.[0] || '').trim()
  if (!name) return null
  const phase = String(row?.[1] || '—').trim() || '—'
  const readyCell = row?.[2]
  const ready = readyFromCell(readyCell)
  return {
    key: buildInspectKey('Pod', name, namespace),
    name,
    namespace,
    ready,
    total: 1,
    restarts: 0,
    phase,
    status: statusFromPhase(phase, ready),
  }
}

/** Merge snapshot and live-detail pod lists for workload inspectors. */
export function collectRelatedPods(inspect) {
  if (!inspect || inspect.kind === 'Pod') return []

  const namespace = inspect.namespace || ''
  const seen = new Set()
  const out = []

  const add = (pod) => {
    if (!pod?.name) return
    const key = pod.key || buildInspectKey('Pod', pod.name, pod.namespace || namespace)
    if (seen.has(key)) return
    seen.add(key)
    out.push({
      ...pod,
      key,
      namespace: pod.namespace || namespace,
    })
  }

  for (const pod of inspect.relatedPods || []) add(pod)

  for (const section of inspect.sections || []) {
    if (!isPodListSection(section)) continue
    for (const row of section.table?.rows || []) {
      const pod = podFromTableRow(row, namespace)
      if (pod) add(pod)
    }
  }

  return out.sort((a, b) => a.name.localeCompare(b.name))
}

export function isWorkloadWithPods(kind) {
  return WORKLOAD_KINDS.has(kind)
}
