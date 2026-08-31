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

  return base
    .map((g) => ({ ...g, sections: dedupeSections(g.sections) }))
    .filter((g) => g.sections.length > 0)
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

/** Table columns that hold object names we can navigate to. */
export function linkableTableColumn(columnName, sectionGroup) {
  const col = String(columnName || '').toLowerCase()
  if (col === 'name' || col === 'pod' || col === 'source' || col === 'volume' || col === 'secret key') {
    return true
  }
  if (sectionGroup === 'relationships' && (col === 'target' || col === 'consumer')) return true
  if (sectionGroup === 'spec' && (col === 'value' || col === 'source')) return true
  return false
}

/** Parse secret:/configMap:/pvc: prefixed refs for inspector navigation. */
export function parseObjectRefCell(cell) {
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
  if (!kind) return null
  const name = m[2].trim()
  if (!name) return null
  return { kind, name, key: `${kind}/${name}` }
}

/** Infer kind for a relationship table row from section context. */
export function inferRowKind(section, columnIndex, cell) {
  const parsed = parseObjectRefCell(cell)
  if (parsed) return parsed.kind
  const title = `${section?.title || ''} ${section?.id || ''}`.toLowerCase()
  if (/^node$|scheduled on|node /.test(title) || title.includes('node')) return 'Node'
  if (/namespace/.test(title) && columnIndex === 0) return 'Namespace'
  if (/pod|consumer|mounted|target pod|used by|scheduled/.test(title)) return 'Pod'
  if (/service/.test(title)) return 'Service'
  if (/ingress/.test(title)) return 'Ingress'
  if (/replicaset/.test(title)) return 'ReplicaSet'
  if (/secret/.test(title)) return 'Secret'
  if (/config/.test(title)) return 'ConfigMap'
  if (/storageclass/.test(title)) return 'StorageClass'
  if (/persistentvolume(?!claim)/.test(title)) return 'PersistentVolume'
  if (/clusterrolebinding/.test(title)) return 'ClusterRoleBinding'
  if (/clusterrole/.test(title)) return 'ClusterRole'
  if (columnIndex === 0) return 'Pod'
  return null
}
