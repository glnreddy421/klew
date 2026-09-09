/**
 * Catalog browse display — lazy list API has status columns; investigation rows do not.
 */

export function entityMergeKey(row) {
  const kind = row?.kind || row?.ref?.kind || ''
  const name = row?.name || row?.ref?.name || ''
  const ns = row?.namespace || row?.ref?.namespace || ''
  if (!kind || !name) return row?.key || ''
  return ns ? `${kind}|${ns}|${name}` : `${kind}|${name}`
}

export function canLoadCatalogEntities(kindGroup) {
  return Boolean(
    kindGroup?.resourceId
    && kindGroup.accessState !== 'forbidden'
    && kindGroup.countState?.state !== 'forbidden'
    && (kindGroup.discovered || kindGroup.virtual)
  )
}

/** Prefer catalog API rows; merge status onto investigation matches by key. */
export function resolveDisplayEntities({
  catalogAll,
  investigationEntities = [],
  lazyEntities = [],
}) {
  const catalogByKey = new Map(lazyEntities.map((row) => [entityMergeKey(row), row]))
  const mergeCatalog = (rows) => rows.map((row) => {
    const catalog = catalogByKey.get(entityMergeKey(row))
    if (!catalog) return row
    return {
      ...row,
      ...catalog,
      key: catalog.key || row.key,
      ref: { ...row.ref, ...catalog.ref },
    }
  })

  if (catalogAll) return lazyEntities
  if (investigationEntities.length > 0) return mergeCatalog(investigationEntities)
  return lazyEntities
}
