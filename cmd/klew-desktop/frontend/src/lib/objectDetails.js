import { categoryLabel, componentCategory, filterSpuriousAnomalies } from './componentInspect'
import { buildInspectKey } from './matches'

/** Group IDs that become inspector tabs (order matters). */
export const DETAIL_TAB_ORDER = [
  { id: 'summary', label: 'Summary' },
  { id: 'containers', label: 'Containers' },
  { id: 'relationships', label: 'Relationships' },
  { id: 'spec', label: 'Spec' },
  { id: 'runtime', label: 'Runtime' },
  { id: 'events', label: 'Events' },
  { id: 'metadata', label: 'Metadata' },
]

/**
 * Normalize a Go ObjectDetail into a UI model.
 * Empty sections are already pruned server-side.
 */
export function normalizeObjectDetail(detail, row) {
  if (!detail && !row) return null
  const kind = detail?.kind || row?.kind || row?.ref?.kind || 'Unknown'
  const name = detail?.ref?.name || detail?.title?.split('/')?.pop() || row?.name || row?.ref?.name || '—'
  const namespace = detail?.ref?.namespace || row?.namespace || row?.ref?.namespace || ''
  const category = detail?.category || componentCategory(kind)
  const sections = (detail?.sections || []).filter((s) => sectionHasContent(s))
  const summary = (detail?.summary || []).filter((f) => f?.value != null && String(f.value).trim() !== '')

  return {
    key: row?.key || buildInspectKey(kind, name, namespace),
    kind,
    name,
    namespace,
    category,
    categoryLabel: categoryLabel(category),
    adhoc: Boolean(row?.adhoc),
    status: {
      tone: detail?.status?.tone || row?.status || 'unknown',
      label: detail?.status?.label || row?.status || 'Unknown',
      fields: summary.map((f) => ({ k: f.key, v: f.value })),
    },
    summary,
    sections,
    groups: groupSections(sections, summary),
    title: detail?.title || `${kind}/${name}`,
  }
}

function sectionHasContent(s) {
  if (!s) return false
  if (s.fields?.length) return true
  if (s.keyValues?.length) return true
  if (s.notes?.length) return true
  if (s.table?.rows?.length) return true
  if (s.code?.trim()) return true
  if (s.altCode?.trim()) return true
  return false
}

function normalizeSectionGroup(group, section) {
  const g = group || inferGroup(section?.id, section?.title)
  if (g === 'status' || g === 'scheduling') return 'summary'
  return g
}

function groupSections(sections, summary) {
  const byGroup = new Map()
  for (const tab of DETAIL_TAB_ORDER) {
    byGroup.set(tab.id, [])
  }
  if (summary?.length) {
    byGroup.get('summary').push({
      id: 'summary',
      title: 'Summary',
      group: 'summary',
      fields: summary.map((f) => ({ key: f.key, value: f.value })),
    })
  }
  for (const s of sections) {
    const g = normalizeSectionGroup(s.group, s)
    if (!byGroup.has(g)) byGroup.set(g, [])
    byGroup.get(g).push(s)
  }
  return DETAIL_TAB_ORDER
    .map((tab) => ({
      ...tab,
      sections: byGroup.get(tab.id) || [],
    }))
    .filter((tab) => tab.sections.length > 0)
}

function inferGroup(id, title) {
  const t = `${id || ''} ${title || ''}`.toLowerCase()
  if (/event/.test(t)) return 'events'
  if (/label|annotation|managed|metadata/.test(t)) return 'metadata'
  if (/owner|subject|role.?ref|selector|mounted|consumer|target|endpoint|claim|used by|referenced|parent|affected|pod.?schedul|active.?job|^jobs$|^pods$/.test(t)) {
    return 'relationships'
  }
  if (/condition|phase|ready|replica|address|capacity|status/.test(t)) return 'summary'
  if (/container|init.?container|sidecar|probe|volume.?mount|environment|secret.?env|image.?pull|restart.?history|security.?context|allocated/.test(t)) {
    return 'containers'
  }
  if (/resource|qos|runtime|restart/.test(t)) return 'runtime'
  if (/schedul|affinity|toleration|node selector|anti-affinity|^taints$/.test(t)) return 'summary'
  if (/spec|strategy|port|rule|volume|environ|template|type|policy|taint|data.?key|secret.?key/.test(t)) {
    return 'spec'
  }
  return 'spec'
}

/** Merge live detail with snapshot-derived signals (anomalies stay frontend). */
export function mergeInspect(detailModel, snapshotInspect) {
  if (!detailModel && !snapshotInspect) return null
  if (!detailModel) {
    return {
      ...snapshotInspect,
      anomalies: filterSpuriousAnomalies(snapshotInspect?.anomalies),
    }
  }
  const snap = snapshotInspect || {}
  const anomalies = filterSpuriousAnomalies(snap.anomalies).filter((a) => {
    if (a.source !== 'status') return true
    if (/\bis unknown$/i.test(String(a.text || ''))) return false
    if (detailModel.status?.tone === 'healthy') return false
    return true
  })

  return {
    ...snap,
    ...detailModel,
    anomalies,
    resourceBars: snap.resourceBars || [],
    notes: snap.notes || [],
    events: snap.events || [],
    meta: snap.meta || { labels: [], annotations: [] },
    relatedPods: snap.relatedPods || [],
    relationships: snap.relationships || [],
    adhoc: snap.adhoc || detailModel.adhoc,
    status: {
      ...(snap.status || {}),
      tone: detailModel.status?.tone || snap.status?.tone,
      label: detailModel.status?.label || snap.status?.label,
      fields: detailModel.status?.fields?.length
        ? detailModel.status.fields
        : (snap.status?.fields || []),
    },
  }
}
