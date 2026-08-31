import { parseInspectKey } from './matches'

/** Compact breadcrumb segments for the active investigation context. */
export function investigationBreadcrumb({
  cluster,
  inspectKey,
  inspectRow,
  focusPinned,
  drillDown,
}) {
  const namespace = cluster?.selectedNamespace || cluster?.currentContext || ''
  const row = inspectRow || (inspectKey ? parseInspectKey(inspectKey) : null)
  if (!row?.kind && !row?.name && !focusPinned) return null

  const segments = []
  if (focusPinned && drillDown?.label) {
    segments.push(drillDown.label)
  } else if (row?.kind) {
    segments.push(row.kind)
    if (row.name) segments.push(row.name)
  }

  if (!segments.length) return null

  return {
    namespace: namespace || '—',
    segments,
    full: [namespace, ...segments].filter(Boolean).join(' / '),
  }
}

export function inspectRowFromKey(key, view, rows = []) {
  if (!key) return null
  const fromRows = rows.find((r) => r.key === key)
  if (fromRows) return fromRows
  const parsed = parseInspectKey(key)
  if (!parsed) return null
  return {
    key,
    kind: parsed.kind,
    name: parsed.name,
    namespace: parsed.namespace,
    ref: parsed,
  }
}
