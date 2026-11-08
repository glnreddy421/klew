/**
 * Which resource types Investigate can target. The engine owns the list
 * (InvestigableTypes): every namespaced type a collector, built-in or user,
 * reads. The Resources view only looks types up here; it keeps no list of
 * its own, so adding a collector extension makes its type investigable.
 */

/** Group and resource from a catalog id ("apps/v1/deployments", "v1/pods"). */
export function groupResourceFromId(resourceId) {
  const parts = String(resourceId || '').split('/').filter(Boolean)
  if (parts.length < 2) return null
  return { group: parts.length === 2 ? '' : parts[0], resource: parts[parts.length - 1] }
}

/** Index of the engine's investigable types by group/resource. */
export function investigableIndex(types) {
  const index = new Map()
  for (const t of Array.isArray(types) ? types : []) {
    if (t?.resource) index.set(`${t.group || ''}/${t.resource}`, t)
  }
  return index
}

/** The investigable type for a catalog resource id, or null. */
export function investigableType(index, resourceId, kind = '') {
  if (!index) return null
  const gr = groupResourceFromId(resourceId)
  if (gr) return index.get(`${gr.group}/${gr.resource}`) || null
  if (!kind) return null
  for (const t of index.values()) if (t.kind === kind) return t
  return null
}

/**
 * Whether an object can be investigated, and why not when it cannot:
 * { ok: true, title } or { ok: false, title }.
 */
export function investigateState(index, { resourceId, kind, name, namespace } = {}, { loaded = true } = {}) {
  const label = kind && name ? `${kind}/${name}` : (kind || 'this resource')
  if (!loaded) return { ok: false, title: 'Checking what Klew can investigate…' }
  if (!namespace) return { ok: false, title: `${label} is cluster-scoped. Investigations cover one namespace.` }
  const type = investigableType(index, resourceId, kind)
  if (!type) {
    return {
      ok: false,
      title: `Klew has no collector for ${kind || 'this type'}, so it cannot investigate ${label}. Add a Collector extension to read it.`,
    }
  }
  const via = type.custom ? ` (custom collector ${type.collector})` : ''
  return { ok: true, title: `Investigate ${label} in ${namespace}${via}` }
}

/**
 * investigateState for a Resources row. The selected type's id applies only
 * when the row is of that type (chains mix kinds); otherwise the row's kind
 * is looked up.
 */
export function rowInvestigateState(index, row, kindGroup, opts) {
  const sameKind = kindGroup?.resourceId && (!kindGroup.kind || kindGroup.kind === row?.kind)
  return investigateState(index, {
    resourceId: sameKind ? kindGroup.resourceId : '',
    kind: row?.kind,
    name: row?.name,
    namespace: row?.namespace,
  }, opts)
}
