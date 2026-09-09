import { buildInspectKey, CLUSTER_SCOPED_KINDS, normalizeInspectKind } from './matches.js'
import { DETAIL_TAB_ORDER } from './objectDetails.js'

/** Key summary metrics for the inspect header strip (skip identity noise). */
export function summaryMetrics(inspect) {
  if (!inspect) return []
  const raw = inspect.summary?.length
    ? inspect.summary.map((f) => ({ key: f.key, value: f.value }))
    : (inspect.status?.fields || []).map((f) => ({ key: f.k, value: f.v }))

  const skip = new Set(['kind', 'name'])
  const seen = new Set()
  const out = []
  for (const f of raw) {
    const key = String(f.key || '').trim()
    const value = String(f.value ?? '').trim()
    if (!key || !value || skip.has(key.toLowerCase())) continue
    const id = `${key}|${value}`
    if (seen.has(id)) continue
    seen.add(id)
    out.push({ key, value })
    if (out.length >= 6) break
  }
  return out
}

/** Merge snapshot-only sections into live detail groups when tabs would be sparse. */
export function enrichInspectGroups(groups, inspect) {
  if (!inspect) return groups || []
  const base = (groups || []).map((g) => ({
    ...g,
    sections: dedupeSections(g.sections || []),
  }))
  const byId = new Map(base.map((g) => [g.id, g]))

  const ensure = (id, label) => {
    if (!byId.has(id)) {
      const g = { id, label, sections: [] }
      byId.set(id, g)
      base.push(g)
    }
    return byId.get(id)
  }

  if (inspect.resourceBars?.length) {
    const runtime = ensure('runtime', 'Runtime')
    if (!runtime.sections.some((s) => s._resourceBars)) {
      runtime.sections.push({
        id: 'snapshot-resources',
        title: 'Resources',
        _resourceBars: inspect.resourceBars,
      })
    }
  }

  if (inspect.events?.length) {
    const events = ensure('events', 'Events')
    if (!events.sections.some((s) => s._events) && !events.sections.some((s) => /event/i.test(s.title || s.id))) {
      events.sections.push({
        id: 'snapshot-events',
        title: 'Recent events',
        _events: inspect.events,
      })
    }
  }

  const labels = inspect.meta?.labels || []
  const annotations = inspect.meta?.annotations || []
  if (labels.length || annotations.length) {
    const meta = ensure('metadata', 'Metadata')
    const hasLiveLabels = meta.sections.some((s) => sectionHasLabels(s))
    const hasLiveAnnotations = meta.sections.some((s) => sectionHasAnnotations(s))
    if (!hasLiveLabels && !hasLiveAnnotations && !meta.sections.some((s) => s._labels || s._annotations)) {
      meta.sections.push({
        id: 'snapshot-labels',
        title: 'Labels & annotations',
        _labels: labels,
        _annotations: annotations,
      })
    }
  }

  const tabOrder = new Map(DETAIL_TAB_ORDER.map((t, i) => [t.id, i]))
  return base
    .map((g) => ({
      ...g,
      label: DETAIL_TAB_ORDER.find((t) => t.id === g.id)?.label || g.label,
      sections: dedupeSections(g.sections),
    }))
    .filter((g) => g.sections.length > 0)
    .sort((a, b) => (tabOrder.get(a.id) ?? 99) - (tabOrder.get(b.id) ?? 99))
}

function sectionHasLabels(s) {
  if (s._labels?.length) return true
  const t = `${s.title || ''} ${s.id || ''}`.toLowerCase()
  return /label/.test(t) && (s.fields?.length || s.keyValues?.length)
}

function sectionHasAnnotations(s) {
  if (s._annotations?.length) return true
  const t = `${s.title || ''} ${s.id || ''}`.toLowerCase()
  return /annotation/.test(t) && (s.fields?.length || s.keyValues?.length)
}

function dedupeSections(sections) {
  const seen = new Set()
  const out = []
  for (const s of sections || []) {
    const sig = sectionSignature(s)
    if (seen.has(sig)) continue
    seen.add(sig)
    out.push(s)
  }
  return out
}

function sectionSignature(s) {
  if (s._labels || s._annotations) return `snap-meta:${s.title}`
  if (s._events) return `snap-events:${s.title}`
  if (s._resourceBars) return `snap-resources:${s.title}`
  const fields = (s.fields || []).map((f) => `${f.key}|${f.value}`).join(';')
  if (fields) return `fields:${s.title}:${fields}`
  const kv = (s.keyValues || []).map((f) => `${f.key}|${f.value}`).join(';')
  if (kv) return `kv:${s.title}:${kv}`
  return `id:${s.id || s.title}`
}

/** Kinds we can navigate to from inspector refs. */
export const LINKABLE_KINDS = new Set([
  'Pod', 'Node', 'Namespace',
  'Deployment', 'ReplicaSet', 'ReplicationController', 'StatefulSet', 'DaemonSet',
  'Job', 'CronJob', 'PodDisruptionBudget',
  'Service', 'Endpoints', 'EndpointSlice', 'Ingress', 'IngressClass', 'NetworkPolicy',
  'ConfigMap', 'Secret', 'ServiceAccount',
  'PersistentVolumeClaim', 'PersistentVolume', 'StorageClass',
  'HorizontalPodAutoscaler', 'Lease', 'ResourceQuota', 'LimitRange',
  'Role', 'ClusterRole', 'RoleBinding', 'ClusterRoleBinding',
  'Gateway', 'HTTPRoute', 'ClusterPolicy', 'Policy',
  'PriorityClass', 'RuntimeClass', 'VolumeAttachment',
  'CSIDriver', 'CSINode', 'CustomResourceDefinition', 'APIService',
  'MutatingWebhookConfiguration', 'ValidatingWebhookConfiguration',
  'ValidatingAdmissionPolicy', 'ValidatingAdmissionPolicyBinding',
])

function isLinkableKind(kind) {
  return LINKABLE_KINDS.has(normalizeInspectKind(kind))
}

function refResult(kind, name, namespace = '') {
  const k = normalizeInspectKind(kind)
  const n = String(name || '').trim()
  if (!k || !n || !isLinkableKind(k)) return null
  if (k === 'Node' && !isPlausibleNodeName(n)) return null
  const ns = CLUSTER_SCOPED_KINDS.has(k) ? '' : String(namespace || '').trim()
  const key = buildInspectKey(k, n, ns)
  if (!key) return null
  return { kind: k, name: n, namespace: ns, key }
}

function isPlausibleNodeName(name) {
  const n = String(name || '').trim()
  if (!n || /^\d+$/.test(n)) return false
  if (/[=,]/.test(n) && !n.includes('/')) return false
  return true
}

/**
 * Parse secret:/configMap:/pvc: prefixed refs for inspector navigation.
 * Backend format is name[/dataKey] — the segment after / is a key inside the object, not namespace.
 */
export function parseObjectRefCell(cell, defaultNamespace = '') {
  const raw = String(cell || '').trim()
  if (!raw) return null
  const m = raw.match(/^(secret|configmap|pvc):([^/]+)(?:\/(.+))?$/i)
  if (!m) return null
  const kindMap = {
    secret: 'Secret',
    configmap: 'ConfigMap',
    pvc: 'PersistentVolumeClaim',
  }
  const kind = kindMap[m[1].toLowerCase()]
  const name = m[2]?.trim()
  if (!kind || !name) return null
  return refResult(kind, name, defaultNamespace)
}

function parseKindNameValue(value, defaultNamespace = '') {
  const raw = String(value || '').trim()
  const slash = raw.indexOf('/')
  if (slash <= 0) return null
  const kind = normalizeInspectKind(raw.slice(0, slash))
  const rest = raw.slice(slash + 1).trim()
  if (!rest || !isLinkableKind(kind)) return null

  if (CLUSTER_SCOPED_KINDS.has(kind)) {
    return refResult(kind, rest)
  }

  const parts = rest.split('/').filter(Boolean)
  if (parts.length >= 2) {
    return refResult(kind, parts.slice(1).join('/'), parts[0])
  }
  return refResult(kind, rest, defaultNamespace)
}

function parseNamespacedName(value, kind, defaultNamespace = '') {
  const raw = String(value || '').trim()
  const m = raw.match(/^([^/\s]+)\/([^/\s]+)$/)
  if (!m) return null
  return refResult(kind, m[2], m[1] || defaultNamespace)
}

function inferKindFromFieldKey(fieldKey, sectionContext) {
  const key = String(fieldKey || '').toLowerCase()
  const ctx = String(sectionContext || '').toLowerCase()

  if (/storage class|storageclass/.test(key) || /storageclass/.test(ctx)) return 'StorageClass'
  if (/persistent volume|volume name|^volume$/.test(key) && !/volume mode|volume binding|volume claim template/.test(key)) {
    if (/claim|pvc/.test(ctx)) return null
    return 'PersistentVolume'
  }
  if (/claim|pvc/.test(key) || (/claim/.test(ctx) && !/reclaim/.test(key))) return 'PersistentVolumeClaim'
  if (/^node$|nominated node|scheduled on|node name/.test(key) || (/node/.test(ctx) && key === 'name')) return 'Node'
  if (/^namespace$/.test(key) || (/parameter/.test(ctx) && /namespace/.test(key))) return 'Namespace'
  if (/ingressclass|^class$/.test(key) || /ingressclass/.test(ctx)) return 'IngressClass'
  if (/^service$|backend service|backend/.test(key) || (/service/.test(ctx) && key === 'name')) return 'Service'
  if (/^role$|role ref/.test(key)) {
    if (/clusterrole/.test(ctx)) return 'ClusterRole'
    return 'Role'
  }
  if (/scale target|reference|target ref/.test(key)) return null // usually Kind/name value
  if (/data source/.test(key)) return null // Kind/name value
  if (/^policy$/.test(key) && /admission/.test(ctx)) return 'ValidatingAdmissionPolicy'
  if (/configmap|config map/.test(key)) return 'ConfigMap'
  if (/secret/.test(key) && !/image pull/.test(key)) return 'Secret'
  if (/service account/.test(key)) return 'ServiceAccount'
  if (/endpointslice/.test(key)) return 'EndpointSlice'
  if (/^object$/.test(key) && /event/.test(ctx)) return null // Kind/name in value
  if (/^target$|^target ref$/.test(key)) return null // Kind/name value carries kind
  return null
}

function inferKindFromPlainName(section, columnName, groupId) {
  const title = `${section?.title || ''} ${section?.id || ''}`.toLowerCase()
  const col = String(columnName || '').toLowerCase()
  if (col === 'uid' || col === 'message' || col === 'value' || col === 'size' || col === 'key') return null
  if (/^node$|^nominated node$/.test(col)) return 'Node'
  if (/scheduled on/.test(title) && col === 'name') return 'Node'
  if (/namespace/.test(title) && col === 'name') return 'Namespace'
  if (/pod|consumer|mounted|target pod|used by|scheduled|active job/.test(title)) return 'Pod'
  if (/service|backend service|load balancer/.test(title)) return 'Service'
  if (/ingress(?!class)/.test(title)) return 'Ingress'
  if (/ingressclass/.test(title)) return 'IngressClass'
  if (/replicaset/.test(title)) return 'ReplicaSet'
  if (/replicationcontroller/.test(title)) return 'ReplicationController'
  if (/secret/.test(title)) return 'Secret'
  if (/configmap|config map|data key/.test(title)) return null
  if (/storageclass/.test(title)) return 'StorageClass'
  if (/persistentvolumeclaim|pvc/.test(title)) return 'PersistentVolumeClaim'
  if (/persistentvolume(?!claim)|^volume$/.test(title)) return 'PersistentVolume'
  if (/clusterrolebinding/.test(title)) return 'ClusterRoleBinding'
  if (/rolebinding/.test(title)) return 'RoleBinding'
  if (/clusterrole/.test(title)) return 'ClusterRole'
  if (/^role$|role ref/.test(title)) return 'Role'
  if (/endpointslice/.test(title)) return 'EndpointSlice'
  if (/endpoint(?!slice)/.test(title)) return 'Endpoints'
  if (/image pull secret/.test(title)) return 'Secret'
  if (/environment|secret environment/.test(title)) return null // Source column uses prefixed refs
  if (/horizontalpodautoscaler|hpa/.test(title)) return 'HorizontalPodAutoscaler'
  if (/poddisruptionbudget|pdb/.test(title)) return 'PodDisruptionBudget'
  if (/lease/.test(title)) return 'Lease'
  if (groupId === 'relationships' && (col === 'name' || col === 'pod' || col === 'target')) return 'Pod'
  return null
}

/**
 * Resolve a field/cell value to an inspect key when it references another resource.
 */
export function resolveInspectRef(value, {
  fieldKey = '',
  columnName = '',
  section = null,
  groupId = '',
  inspectNamespace = '',
  row = null,
  columns = null,
  columnIndex = -1,
} = {}) {
  const raw = String(value ?? '').trim()
  if (!raw || raw === '—' || raw === '-') return null
  if (raw.includes(',') && raw.length > 64) return null

  const prefixed = parseObjectRefCell(raw, inspectNamespace)
  if (prefixed) return prefixed

  const kindName = parseKindNameValue(raw, inspectNamespace)
  if (kindName) return kindName

  if (row && columns?.length) {
    const fromRow = resolveStructuredTableRow(row, columns, columnIndex, inspectNamespace)
    if (fromRow) return fromRow
  }

  const sectionContext = `${section?.title || ''} ${section?.id || ''} ${fieldKey} ${columnName}`
  const keyLower = String(fieldKey || columnName || '').toLowerCase()

  if (/claim ref|^claim$/.test(keyLower)) {
    const claim = parseNamespacedName(raw, 'PersistentVolumeClaim', inspectNamespace)
    if (claim) return claim
  }
  if ((/^service$/.test(keyLower) || String(columnName || '').toLowerCase() === 'service') && raw.includes('/')) {
    const svc = parseNamespacedName(raw, 'Service', inspectNamespace)
    if (svc) return svc
  }
  if (/webhook/.test(`${section?.title || ''} ${section?.id || ''}`.toLowerCase())
    && String(columnName || fieldKey || '').toLowerCase() === 'service'
    && raw.includes('/')) {
    return parseNamespacedName(raw, 'Service', inspectNamespace)
  }

  const fieldKind = inferKindFromFieldKey(fieldKey, sectionContext)
  if (fieldKind) {
    const fromField = refResult(fieldKind, raw, inspectNamespace)
    if (fromField) return fromField
  }

  const plainKind = inferKindFromPlainName(section, columnName || fieldKey, groupId)
  if (plainKind) {
    const fromPlain = refResult(plainKind, raw, inspectNamespace)
    if (fromPlain) return fromPlain
  }

  if (columnName?.toLowerCase() === 'object' || fieldKey?.toLowerCase() === 'object') {
    return parseKindNameValue(raw) || refResult(inferRowKind(section, columnIndex, raw), raw, inspectNamespace)
  }

  if (String(columnName || fieldKey || '').toLowerCase() === 'target') {
    return parseKindNameValue(raw, inspectNamespace)
  }

  return null
}

function resolveStructuredTableRow(row, columns, columnIndex, inspectNamespace) {
  const lowerCols = columns.map((c) => String(c || '').toLowerCase())
  const kindIdx = lowerCols.indexOf('kind')
  const nameIdx = lowerCols.indexOf('name')
  const nsIdx = lowerCols.indexOf('namespace')
  if (kindIdx < 0 || nameIdx < 0) return null
  if (columnIndex >= 0) {
    const col = lowerCols[columnIndex]
    const linkCols = new Set(['kind', 'name', 'namespace', 'role', 'object', 'service', 'claim', 'target', 'pod'])
    if (!linkCols.has(col)) return null
  }
  const kind = row[kindIdx]
  const name = row[nameIdx]
  const ns = nsIdx >= 0 ? row[nsIdx] : inspectNamespace
  return refResult(kind, name, ns)
}

/** @deprecated use resolveInspectRef — kept for callers that check column eligibility first */
export function linkableTableColumn(columnName, sectionGroup) {
  const col = String(columnName || '').toLowerCase()
  if (col === 'uid' || col === 'message' || col === 'size' || col === 'key' || col === 'value') return false
  if (col === 'name' || col === 'pod' || col === 'source' || col === 'volume' || col === 'object') return true
  if (col === 'role' || col === 'policy' || col === 'service' || col === 'claim' || col === 'target') return true
  if (sectionGroup === 'relationships') return true
  if (sectionGroup === 'spec' || sectionGroup === 'status' || sectionGroup === 'containers') {
    if (col === 'source') return true
    return col !== 'type' && col !== 'status' && col !== 'reason'
      && col !== 'state' && col !== 'exit' && col !== 'restarts' && col !== 'ready'
      && col !== 'probe' && col !== 'container' && col !== 'container id' && col !== 'image id'
      && col !== 'image' && col !== 'started' && col !== 'alloc cpu' && col !== 'alloc mem'
      && col !== 'mount path' && col !== 'sub path' && col !== 'read only'
  }
  return false
}

/** Infer kind for a relationship table row from section context. */
export function inferRowKind(section, columnIndex, cell) {
  const parsed = parseObjectRefCell(cell)
  if (parsed) return parsed.kind
  return inferKindFromPlainName(section, '', 'relationships')
    || inferKindFromPlainName(section, 'name', 'relationships')
}
