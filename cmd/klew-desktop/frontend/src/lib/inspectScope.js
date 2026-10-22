import { CLUSTER_SCOPED_KINDS } from './matches.js'

/** Discovery's namespaced flag for a kind from the catalog scope tree (null if unknown). */
export function discoveryNamespaced(tree, kind) {
  if (!kind) return null
  for (const cat of tree?.categories || []) {
    for (const k of cat?.kinds || []) {
      if (k?.kind === kind && typeof k.namespaced === 'boolean') return k.namespaced
    }
  }
  return null
}

/**
 * Inspector header scope: namespaced kinds show their namespace, never
 * "Cluster-scoped". Uses discovery first, then the built-in cluster-scoped list.
 */
export function inspectScope({ kind, rowNamespace, detailNamespace, fallbackNamespace, tree, kindGroup } = {}) {
  let namespaced = null
  if (kindGroup?.kind === kind && typeof kindGroup?.namespaced === 'boolean') namespaced = kindGroup.namespaced
  if (namespaced == null) namespaced = discoveryNamespaced(tree, kind)
  if (namespaced == null) namespaced = !CLUSTER_SCOPED_KINDS.has(kind)
  if (!namespaced) return { namespaced: false, label: 'Cluster-scoped' }
  const ns = rowNamespace || detailNamespace || fallbackNamespace || ''
  return { namespaced: true, namespace: ns, label: ns }
}
