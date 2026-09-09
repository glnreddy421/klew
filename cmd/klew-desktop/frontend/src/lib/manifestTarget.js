import { CLUSTER_SCOPED_KINDS } from './matches.js'

/**
 * Resolve kubectl manifest fetch target from an inspected entity row.
 * Nav kind metadata (resourceId, scope) applies only when it matches the inspected kind —
 * e.g. opening a ReplicaSet from the Pods list must not reuse the Pods resourceId.
 */
export function buildManifestTarget(inspectRow, kindGroup) {
  if (!inspectRow) return null
  const kind = inspectRow.kind || inspectRow.ref?.kind
  const name = inspectRow.name || inspectRow.ref?.name
  if (!kind || !name) return null

  const namespace = inspectRow.namespace || inspectRow.ref?.namespace || ''
  const navMatchesKind = !kindGroup?.kind || kindGroup.kind === kind
  const resourceId = inspectRow.resourceId || (navMatchesKind ? kindGroup?.resourceId : '') || ''

  const clusterScoped = CLUSTER_SCOPED_KINDS.has(kind)
    || (navMatchesKind && kindGroup?.namespaced === false)

  return {
    kind,
    name,
    namespace,
    resourceId,
    clusterScoped,
  }
}

export function manifestTargetKey(target) {
  if (!target) return ''
  return [
    target.resourceId || target.kind,
    target.namespace || '*',
    target.name,
  ].join('|')
}
