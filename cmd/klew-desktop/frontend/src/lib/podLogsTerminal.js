import { getSnapshot } from './investigationViews'

/** @typedef {{ name: string, init?: boolean }} PodLogContainer */
/** @typedef {{ podName: string, namespace: string, containers: PodLogContainer[] }} PodLogsTarget */

function podKind(inspectRow) {
  return inspectRow?.kind || inspectRow?.ref?.kind || ''
}

function podName(inspectRow) {
  return inspectRow?.name || inspectRow?.ref?.name || ''
}

function podNamespace(inspectRow) {
  return inspectRow?.namespace || inspectRow?.ref?.namespace || ''
}

function normalizeContainers(raw = []) {
  return raw
    .map((c) => ({
      name: String(c?.name || '').trim(),
      init: Boolean(c?.init),
    }))
    .filter((c) => c.name)
}

function containersFromInspectRow(inspectRow, view) {
  const fromTable = normalizeContainers(inspectRow?.table?.containers)
  if (fromTable.length) return fromTable

  const snap = getSnapshot(view)
  const pod = (snap.pods || []).find((p) => p.name === podName(inspectRow))
  const fromSnap = normalizeContainers(pod?.containers)
  if (fromSnap.length) return fromSnap

  const fromCatalog = normalizeContainers(inspectRow?.containerStatuses)
  if (fromCatalog.length) return fromCatalog

  const names = (inspectRow?.containerNames || []).map((n) => String(n || '').trim()).filter(Boolean)
  if (names.length) return names.map((name) => ({ name }))

  return []
}

/** Resolve a pod log tail target from the current inspector selection. */
export function resolvePodLogsTarget(inspectRow, view) {
  if (!inspectRow || podKind(inspectRow) !== 'Pod') return null
  const name = podName(inspectRow)
  if (!name) return null
  return {
    podName: name,
    namespace: podNamespace(inspectRow),
    containers: containersFromInspectRow(inspectRow, view),
  }
}

