/**
 * Dynamic component inspect — discovers whatever the snapshot/evidence has
 * for the hovered object. No fixed per-kind template: fields, labels,
 * events, resources, and anomalies are derived from that object only.
 */

const WORKLOAD = new Set(['Deployment', 'StatefulSet', 'DaemonSet', 'Job', 'CronJob', 'ReplicaSet'])

export function componentCategory(kind) {
  if (!kind) return 'component'
  if (kind === 'Pod') return 'runtime'
  if (WORKLOAD.has(kind)) return 'workload'
  if (['Service', 'Ingress', 'Endpoints', 'EndpointSlice', 'NetworkPolicy'].includes(kind)) return 'network'
  if (kind === 'ConfigMap' || kind === 'Secret') return 'config'
  if (kind === 'PersistentVolumeClaim' || kind === 'PVC') return 'storage'
  if (['ServiceAccount', 'Role', 'RoleBinding', 'ClusterRole', 'ClusterRoleBinding'].includes(kind)) return 'access'
  if (kind === 'HorizontalPodAutoscaler' || kind === 'HPA') return 'autoscaling'
  if (kind === 'Node') return 'node'
  return 'component'
}

export function categoryLabel(cat) {
  const map = {
    workload: 'Workload',
    runtime: 'Runtime',
    network: 'Network',
    config: 'Config',
    storage: 'Storage',
    access: 'Access control',
    autoscaling: 'Autoscaling',
    node: 'Node',
    component: 'Component',
  }
  return map[cat] || 'Component'
}

/**
 * Build inspect model for whatever object is hovered / selected.
 * Anomalies (signals) are scoped to this object only — never overall verdict.
 */
export function buildComponentInspect(view, row) {
  if (!row) return null

  const kind = row.kind || row.ref?.kind || 'Unknown'
  const name = row.name || row.ref?.name || '—'
  const category = componentCategory(kind)
  const snap = view?.state?.snapshot || {}

  const resolved = resolveSnapshotObject(snap, kind, name)
  const relatedPods = findRelatedPods(kind, name, snap)
  const status = {
    tone: deriveTone(row, resolved, relatedPods),
    label: deriveLabel(row, resolved, relatedPods),
    fields: collectStatusFields(kind, name, row, resolved, relatedPods, snap),
  }
  const meta = collectMeta(resolved, relatedPods)
  const events = collectObjectEvents(view, kind, name).slice(0, 24)
  const resourceBars = buildResourceBars(relatedPods, snap.metrics)
  const anomalies = collectAnomalies(view, kind, name, row, resolved, relatedPods, events)
  const relationships = collectRelationships(kind, name, snap, relatedPods, resolved)

  return {
    key: row.key,
    kind,
    name,
    category,
    categoryLabel: categoryLabel(category),
    status,
    meta,
    events,
    resourceBars,
    anomalies,
    relatedPods: relatedPodsForInspect(relatedPods),
    relationships,
    adhoc: Boolean(row.adhoc),
    notes: buildNotes(events.length, anomalies.length, resourceBars, snap.metrics),
  }
}

function collectRelationships(kind, name, snap, relatedPods, resolved) {
  const rels = []
  const seen = new Set()
  const add = (k, n, role, ns) => {
    if (!n || !k) return
    const key = `${k}/${n}`
    const id = `${role}|${key}`
    if (seen.has(id)) return
    seen.add(id)
    rels.push({
      kind: k,
      name: n,
      key,
      role,
      namespace: ns || resolved?.namespace || '',
    })
  }

  for (const p of relatedPods) {
    add('Pod', p.name, 'Pod', p.namespace)
  }

  if (kind === 'Pod' && resolved) {
    for (const o of resolved.ownerRefs || []) {
      add(o.kind, o.name, 'Owner', o.namespace)
    }
    if (resolved.node) add('Node', resolved.node, 'Node')
    for (const cm of resolved.configMapRefs || []) add('ConfigMap', cm, 'ConfigMap')
    for (const sec of resolved.secretRefs || []) add('Secret', sec, 'Secret')
    for (const pvc of resolved.pvcRefs || []) add('PersistentVolumeClaim', pvc, 'Volume')
  }

  if (resolved?.rootOwner) {
    add(resolved.rootOwner.kind, resolved.rootOwner.name, 'Owner', resolved.rootOwner.namespace)
  }

  if (kind === 'Deployment') {
    for (const rs of snap.replicaSets || []) {
      if (rs.deploymentOwner === name) add('ReplicaSet', rs.name, 'ReplicaSet', rs.namespace)
    }
  }

  const podLabels = relatedPods.map((p) => p.labels || {})
  for (const svc of snap.services || []) {
    if (serviceMatchesPodLabels(svc, podLabels)) {
      add('Service', svc.name, 'Service', svc.namespace)
    }
    if (kind === 'Service' && svc.name === name) {
      for (const p of relatedPods) add('Pod', p.name, 'Target pod', p.namespace)
    }
  }

  for (const ing of snap.ingresses || []) {
    const backends = ing.backends || []
    if (kind === 'Service' && backends.includes(name)) {
      add('Ingress', ing.name, 'Ingress', ing.namespace)
    }
    if (WORKLOAD.has(kind)) {
      for (const svc of snap.services || []) {
        if (backends.includes(svc.name) && serviceMatchesPodLabels(svc, podLabels)) {
          add('Ingress', ing.name, 'Ingress', ing.namespace)
        }
      }
    }
  }

  for (const h of snap.hpas || []) {
    if (h.targetName === name && (!h.targetKind || h.targetKind === kind || kindAliases(h.targetKind, kind))) {
      add('HorizontalPodAutoscaler', h.name, 'Autoscaling', h.namespace)
    }
  }

  if (['ConfigMap', 'Secret', 'PersistentVolumeClaim', 'PVC'].includes(kind)) {
    for (const p of snap.pods || []) {
      const refs = kind === 'ConfigMap' ? p.configMapRefs
        : kind === 'Secret' ? p.secretRefs : p.pvcRefs
      if (refs?.includes(name)) add('Pod', p.name, 'Mounted by', p.namespace)
    }
    for (const ref of [...(snap.configRefs || []), ...(snap.secretRefs || []), ...(snap.pvcRefs || [])]) {
      if (ref.name === name && ref.usedBy) add('Pod', ref.usedBy, 'Used by', ref.namespace)
    }
  }

  return rels.sort((a, b) => a.role.localeCompare(b.role) || a.name.localeCompare(b.name))
}

function serviceMatchesPodLabels(svc, podLabelSets) {
  const sel = parseSelectorString(svc?.selector)
  if (!sel.length || !podLabelSets.length) return false
  return podLabelSets.some((labels) => sel.every(([k, v]) => labels[k] === v))
}

function parseSelectorString(raw) {
  if (!raw) return []
  return String(raw)
    .split(',')
    .map((part) => part.trim())
    .filter(Boolean)
    .map((part) => {
      const i = part.indexOf('=')
      if (i < 0) return [part, '']
      return [part.slice(0, i).trim(), part.slice(i + 1).trim()]
    })
}

function relatedPodsForInspect(pods) {
  return (pods || []).map((p) => {
    let status = p.ready ? 'healthy' : 'degraded'
    for (const c of p.containers || []) {
      const reason = (c.reason || c.lastReason || '').toLowerCase()
      if (reason.includes('crash') || reason.includes('oom') || reason.includes('backoff')) {
        status = 'critical'
        break
      }
    }
    return {
      key: `Pod/${p.name}`,
      name: p.name,
      namespace: p.namespace,
      ready: p.ready ? 1 : 0,
      total: 1,
      restarts: p.restartCount || 0,
      phase: p.phase || '—',
      status,
    }
  }).sort((a, b) => a.name.localeCompare(b.name))
}

/** Find the richest snapshot record for this kind/name. */
function resolveSnapshotObject(snap, kind, name) {
  const pools = [
    ...(snap.workloads || []).map((o) => ({ ...o, _pool: 'workload' })),
    ...(snap.pods || []).map((o) => ({ ...o, kind: 'Pod', _pool: 'pod' })),
    ...(snap.services || []).map((o) => ({ ...o, kind: 'Service', _pool: 'service' })),
    ...(snap.ingresses || []).map((o) => ({ ...o, kind: 'Ingress', _pool: 'ingress' })),
    ...(snap.replicaSets || []).map((o) => ({ ...o, kind: 'ReplicaSet', _pool: 'rs' })),
    ...(snap.hpas || []).map((o) => ({ ...o, kind: o.kind || 'HorizontalPodAutoscaler', _pool: 'hpa' })),
    ...(snap.nodes || []).map((o) => ({ ...o, kind: 'Node', _pool: 'node' })),
    ...(snap.configRefs || []).map((o) => ({ ...o, _pool: 'configRef' })),
    ...(snap.secretRefs || []).map((o) => ({ ...o, _pool: 'secretRef' })),
    ...(snap.pvcRefs || []).map((o) => ({ ...o, _pool: 'pvcRef' })),
  ]
  return pools.find((o) => o.name === name && (!o.kind || o.kind === kind || kindAliases(o.kind, kind))) || null
}

function findRelatedPods(kind, name, snap) {
  const pods = snap.pods || []
  if (kind === 'Pod') return pods.filter((p) => p.name === name)
  if (WORKLOAD.has(kind)) {
    return pods.filter((p) => {
      if ((p.name || '').startsWith(`${name}-`)) return true
      for (const o of p.ownerRefs || []) {
        if (o.name === name) return true
        if (o.kind === 'ReplicaSet' && String(o.name || '').startsWith(`${name}-`)) return true
      }
      return labelValueTouches(p.labels, name)
    })
  }
  if (kind === 'Service') {
    return pods.filter((p) => labelValueTouches(p.labels, name) || (p.name || '').startsWith(`${name}-`))
  }
  // Config/Secret/PVC: pods that mount them
  return pods.filter((p) => {
    if (kind === 'ConfigMap' && p.configMapRefs?.includes(name)) return true
    if (kind === 'Secret' && p.secretRefs?.includes(name)) return true
    if ((kind === 'PersistentVolumeClaim' || kind === 'PVC') && p.pvcRefs?.includes(name)) return true
    return false
  })
}

function labelValueTouches(labels, name) {
  if (!labels || !name) return false
  const n = name.toLowerCase()
  return Object.values(labels).some((v) => {
    const val = String(v).toLowerCase()
    return val === n || val.startsWith(`${n}-`)
  })
}

/** Flatten whatever useful scalar/list fields exist on the resolved object. */
function collectStatusFields(kind, name, row, resolved, relatedPods, snap) {
  const fields = []
  const push = (k, v) => {
    if (v == null || v === '' || v === 'undefined') return
    const s = Array.isArray(v) ? v.filter(Boolean).join(', ') : String(v)
    if (!s) return
    fields.push({ k, v: s })
  }

  push('Kind', kind)
  push('Name', name)

  if (resolved) {
    // Dynamic: walk known useful keys present on any snapshot type
    const prefer = [
      'namespace', 'phase', 'ready', 'replicas', 'available', 'updated',
      'generation', 'observedGeneration', 'selector', 'type', 'clusterIP',
      'readyEndpoints', 'totalEndpoints', 'ports', 'hosts', 'backends',
      'node', 'restartCount', 'deploymentOwner', 'targetKind', 'targetName',
      'minReplicas', 'maxReplicas', 'currentReplicas', 'desiredReplicas',
      'usedBy', 'memoryPressure', 'diskPressure', 'pidPressure',
      'kubeletVersion', 'conditions',
    ]
    for (const key of prefer) {
      if (!(key in resolved) || resolved[key] == null) continue
      if (key === 'ready' && typeof resolved.ready === 'boolean') {
        push('Ready', resolved.ready ? 'true' : 'false')
        continue
      }
      if (key === 'ready' && typeof resolved.ready === 'number') {
        push('Ready', `${resolved.ready} / ${resolved.replicas ?? '—'}`)
        continue
      }
      if (key === 'conditions' && Array.isArray(resolved.conditions)) {
        for (const c of resolved.conditions.slice(0, 6)) push('Condition', c)
        continue
      }
      if (key === 'ports' || key === 'hosts' || key === 'backends') {
        push(titleCase(key), Array.isArray(resolved[key]) ? resolved[key].join(', ') : resolved[key])
        continue
      }
      if (key === 'readyEndpoints' || key === 'totalEndpoints') continue // combined below
      push(titleCase(key), resolved[key])
    }
    if (resolved.readyEndpoints != null || resolved.totalEndpoints != null) {
      push('Endpoints', `${resolved.readyEndpoints ?? '—'} / ${resolved.totalEndpoints ?? '—'}`)
    }
    if (resolved.atMax) push('At max', 'true')
  } else {
    // Row-derived fallback when object not in a typed pool (e.g. synthesized ConfigMap)
    push('Ready', row.ready != null ? `${row.ready} / ${row.total ?? '—'}` : null)
    if (row.restarts != null) push('Restarts', row.restarts)
    if (row.signal) push('Signal', row.signal)
  }

  // Related realm facts (dynamic from pods that belong to this object)
  if (relatedPods.length) {
    push('Related pods', relatedPods.length)
    const restarts = relatedPods.reduce((n, p) => n + (p.restartCount || 0), 0)
    push('Pod restarts', restarts)
    const notReady = relatedPods.filter((p) => !p.ready).map((p) => p.name)
    if (notReady.length) push('Not ready', notReady.join(', '))
    const images = [...new Set(relatedPods.flatMap((p) => (p.containers || []).map((c) => c.image).filter(Boolean)))]
    if (images.length) push('Images', images.slice(0, 5).join(', '))
    // Container runtime states
    for (const p of relatedPods.slice(0, 3)) {
      for (const c of (p.containers || []).slice(0, 3)) {
        const state = [c.state, c.reason].filter(Boolean).join(':')
        if (state && state !== 'running') {
          push(`${p.name}/${c.name}`, state)
        }
      }
    }
  }

  // Mount / claim reverse links from snapshot refs
  for (const ref of [...(snap.configRefs || []), ...(snap.secretRefs || []), ...(snap.pvcRefs || [])]) {
    if (ref.name === name && ref.usedBy) push('Used by', ref.usedBy)
  }

  // HPA targeting this workload
  for (const h of snap.hpas || []) {
    if (h.targetName === name) {
      push('HPA', `${h.name}: ${h.currentReplicas}→${h.desiredReplicas} (${h.minReplicas}–${h.maxReplicas})`)
    }
  }

  // Dedupe by key+value
  const seen = new Set()
  return fields.filter((f) => {
    const id = `${f.k}|${f.v}`
    if (seen.has(id)) return false
    seen.add(id)
    return true
  }).slice(0, 28)
}

function collectMeta(resolved, relatedPods) {
  const labels = { ...(resolved?.labels || {}) }
  const annotations = { ...(resolved?.annotations || {}) }
  if (!Object.keys(labels).length) {
    for (const p of relatedPods) Object.assign(labels, p.labels || {})
  }
  if (!Object.keys(annotations).length) {
    for (const p of relatedPods) Object.assign(annotations, p.annotations || {})
  }

  return {
    labels: Object.entries(labels).slice(0, 30).map(([k, v]) => ({ k, v: String(v) })),
    annotations: Object.entries(annotations)
      .filter(([k]) => !k.startsWith('kubectl.kubernetes.io/last-applied-configuration'))
      .filter(([, v]) => String(v).length < 240)
      .slice(0, 20)
      .map(([k, v]) => ({ k, v: String(v) })),
  }
}

/**
 * Anomalies / signals for THIS object only.
 * Combines view.signals matched to the object + derived health issues.
 */
function collectAnomalies(view, kind, name, row, resolved, relatedPods, events) {
  const out = []
  const push = (a) => {
    if (!a?.text) return
    out.push({
      level: a.level || 'warn',
      text: a.text,
      source: a.source || 'derived',
    })
  }

  // Engine signals scoped to this object
  for (const s of view?.signals || []) {
    if (!signalBelongsTo(s, kind, name, relatedPods)) continue
    const sev = String(s.severity || s.strength || '').toLowerCase()
    const level = sev === 'critical' || sev === 'high' || sev === 'strong' ? 'crit'
      : sev === 'medium' || sev === 'warning' ? 'warn' : 'info'
    push({
      level,
      text: s.label || s.evidence || s.id,
      source: s.source || 'signal',
    })
  }

  // Derived from object state
  if (row.status && row.status !== 'healthy') {
    push({ level: row.status === 'critical' ? 'crit' : 'warn', text: row.signal || `${kind}/${name} is ${row.status}`, source: 'status' })
  }
  if (typeof resolved?.ready === 'number' && typeof resolved?.replicas === 'number' && resolved.ready < resolved.replicas) {
    push({ level: 'crit', text: `Only ${resolved.ready}/${resolved.replicas} replicas ready`, source: 'workload' })
  }
  if (resolved?.ready === false) {
    push({ level: 'crit', text: `${name} is not ready`, source: 'pod' })
  }
  if ((resolved?.restartCount || 0) >= 3) {
    push({ level: 'warn', text: `${resolved.restartCount} restarts on this pod`, source: 'pod' })
  }
  const unready = relatedPods.filter((p) => !p.ready)
  if (unready.length) {
    push({ level: 'crit', text: `${unready.length} related pod(s) not ready: ${unready.map((p) => p.name).join(', ')}`, source: 'pods' })
  }
  const highRestarts = relatedPods.filter((p) => (p.restartCount || 0) >= 3)
  if (highRestarts.length) {
    push({
      level: 'warn',
      text: `High restarts: ${highRestarts.map((p) => `${p.name}(${p.restartCount})`).join(', ')}`,
      source: 'pods',
    })
  }
  for (const p of relatedPods) {
    for (const c of p.containers || []) {
      const reason = `${c.reason || ''} ${c.lastReason || ''}`.toLowerCase()
      if (reason.includes('crash') || reason.includes('oom') || reason.includes('backoff')) {
        push({ level: 'crit', text: `${p.name}/${c.name}: ${c.reason || c.lastReason}`, source: 'container' })
      } else if (c.state === 'waiting' || c.state === 'terminated') {
        push({ level: 'warn', text: `${p.name}/${c.name}: ${c.state}${c.reason ? ` (${c.reason})` : ''}`, source: 'container' })
      }
    }
  }
  if (resolved?.readyEndpoints != null && resolved?.totalEndpoints != null
    && resolved.totalEndpoints > 0 && resolved.readyEndpoints < resolved.totalEndpoints) {
    push({ level: 'crit', text: `Endpoints ${resolved.readyEndpoints}/${resolved.totalEndpoints} ready`, source: 'service' })
  }
  if (resolved?.atMax) {
    push({ level: 'warn', text: 'HPA is at maximum replicas', source: 'hpa' })
  }
  if (resolved?.memoryPressure || resolved?.diskPressure || resolved?.pidPressure) {
    push({ level: 'crit', text: 'Node pressure detected', source: 'node' })
  }

  // Warning events → anomalies
  for (const ev of events) {
    if (ev.severity === 'warning' || ev.severity === 'high' || ev.severity === 'critical' || ev.type === 'Warning') {
      push({
        level: ev.severity === 'critical' || ev.severity === 'high' ? 'crit' : 'warn',
        text: `${ev.reason || 'Event'}: ${truncate(ev.message, 120)}`,
        source: 'event',
      })
    }
  }

  // Dedupe by text
  const seen = new Set()
  return out.filter((a) => {
    if (seen.has(a.text)) return false
    seen.add(a.text)
    return true
  }).slice(0, 12)
}

function signalBelongsTo(s, kind, name, relatedPods) {
  const ref = s.objectRef || {}
  if (ref.name) {
    if (ref.name === name && (!ref.kind || ref.kind === kind || kindAliases(ref.kind, kind))) return true
    if (relatedPods.some((p) => p.name === ref.name)) return true
    if (WORKLOAD.has(kind) && String(ref.name).startsWith(`${name}-`)) return true
  }
  const hay = `${s.label || ''} ${s.evidence || ''}`.toLowerCase()
  const n = name.toLowerCase()
  if (hay.includes(n)) return true
  return relatedPods.some((p) => hay.includes(p.name.toLowerCase()))
}

function deriveTone(row, resolved, relatedPods) {
  if (row.status === 'critical') return 'critical'
  if (row.status === 'degraded' || row.status === 'warning') return 'degraded'
  if (relatedPods.some((p) => !p.ready)) return 'degraded'
  if (typeof resolved?.ready === 'number' && typeof resolved?.replicas === 'number' && resolved.ready < resolved.replicas) {
    return 'degraded'
  }
  if (resolved?.ready === false) return 'degraded'
  if (row.status === 'healthy' || resolved?.ready === true) return 'healthy'
  if (typeof resolved?.ready === 'number' && resolved.ready >= (resolved.replicas ?? resolved.ready)) return 'healthy'
  return row.status || 'unknown'
}

function deriveLabel(row, resolved, relatedPods) {
  const tone = deriveTone(row, resolved, relatedPods)
  if (tone === 'healthy') return 'HEALTHY'
  if (tone === 'critical') return 'CRITICAL'
  if (tone === 'degraded') return 'DEGRADED'
  return (row.status || 'UNKNOWN').toUpperCase()
}

function buildResourceBars(relatedPods, metrics) {
  const bars = []
  let reqCpu = 0
  let limCpu = 0
  let reqMem = 0
  let limMem = 0
  let any = false

  for (const p of relatedPods) {
    for (const c of p.containers || []) {
      const rCpu = parseCPU(c.requestsCPU)
      const lCpu = parseCPU(c.limitsCPU)
      const rMem = parseMemMi(c.requestsMem)
      const lMem = parseMemMi(c.limitsMem)
      if (rCpu || lCpu || rMem || lMem) any = true
      reqCpu += rCpu
      limCpu += lCpu
      reqMem += rMem
      limMem += lMem
    }
  }

  if (!any) {
    if (relatedPods.length) {
      return [{
        id: 'empty',
        label: 'Resources',
        empty: true,
        detail: 'No CPU/memory requests or limits on related pod specs',
        source: 'pod specs',
      }]
    }
    return []
  }

  const hasUsage = Boolean(metrics?.available)

  if (reqCpu || limCpu) {
    const usage = hasUsage ? (metrics.cpuUsageMillicores || 0) : null
    const scale = Math.max(limCpu || reqCpu, usage || 0, 1)
    bars.push({
      id: 'cpu',
      label: 'CPU',
      request: reqCpu,
      limit: limCpu,
      usage,
      requestPct: Math.min(100, Math.round((reqCpu / scale) * 100)),
      limitPct: limCpu ? Math.min(100, Math.round((limCpu / scale) * 100)) : 0,
      usagePct: usage != null ? Math.min(100, Math.round((usage / scale) * 100)) : null,
      detail: limCpu
        ? `request ${fmtCpu(reqCpu)} · limit ${fmtCpu(limCpu)}${usage != null ? ` · usage ${fmtCpu(usage)}` : ''}`
        : `request ${fmtCpu(reqCpu)} · no limit`,
      source: 'from pod specs',
    })
  }

  if (reqMem || limMem) {
    const usage = hasUsage ? (metrics.memUsageMi || 0) : null
    const scale = Math.max(limMem || reqMem, usage || 0, 1)
    bars.push({
      id: 'mem',
      label: 'Memory',
      request: reqMem,
      limit: limMem,
      usage,
      requestPct: Math.min(100, Math.round((reqMem / scale) * 100)),
      limitPct: limMem ? Math.min(100, Math.round((limMem / scale) * 100)) : 0,
      usagePct: usage != null ? Math.min(100, Math.round((usage / scale) * 100)) : null,
      detail: limMem
        ? `request ${reqMem}Mi · limit ${limMem}Mi${usage != null ? ` · usage ${usage}Mi` : ''}`
        : `request ${reqMem}Mi · no limit`,
      source: 'from pod specs',
    })
  }

  return bars
}

function buildNotes(eventCount, anomalyCount, resourceBars, metrics) {
  const notes = []
  if (!eventCount) notes.push('No Kubernetes events for this object in the current window.')
  if (!anomalyCount) notes.push('No anomalies detected for this object.')
  if (resourceBars.some((b) => !b.empty) && !metrics?.available) {
    notes.push('Usage unavailable (metrics-server). Bars show requests/limits from pod specs.')
  }
  return notes
}

/** Events only for this object — never container logs. */
function collectObjectEvents(view, kind, name) {
  const snap = view?.state?.snapshot || {}
  const out = []
  const seen = new Set()
  const push = (ev) => {
    const id = `${ev.time}|${ev.reason}|${ev.message}`
    if (seen.has(id)) return
    seen.add(id)
    out.push(ev)
  }

  for (const e of snap.events || []) {
    const obj = e.involvedObject || {}
    if (!objectMatches(obj.kind, obj.name, kind, name)) continue
    push({
      time: formatTime(e.timestamp),
      type: e.type || 'Normal',
      reason: e.reason || '',
      message: e.message || '',
      severity: eventSeverity(e.type, e.reason),
      count: e.count || 1,
    })
  }

  for (const e of view?.evidence || []) {
    if (e.sourceType === 'log') continue
    if (e.sourceType !== 'k8s_event' && e.sourceType !== 'object_change') continue
    if (!evidenceMatches(e, kind, name)) continue
    push({
      time: formatTime(e.timestamp),
      type: e.sourceType === 'object_change' ? 'Object' : 'Event',
      reason: e.reason || '',
      message: truncate(e.message || '', 180),
      severity: e.severity || eventSeverity(e.sourceType, e.reason),
      count: e.count || 1,
    })
  }

  return out
}

function objectMatches(objKind, objName, kind, name) {
  if (!objName) return false
  if (objName === name) {
    if (!objKind || !kind) return true
    return objKind === kind || kindAliases(objKind, kind)
  }
  if (WORKLOAD.has(kind) && (objKind === 'Pod' || !objKind) && String(objName).startsWith(`${name}-`)) return true
  return false
}

function evidenceMatches(e, kind, name) {
  if (e.sourceKind && e.sourceName && objectMatches(e.sourceKind, e.sourceName, kind, name)) return true
  if (kind === 'Pod' && (e.pod === name || e.sourceName === name)) return true
  if (WORKLOAD.has(kind) && e.pod && String(e.pod).startsWith(`${name}-`)) return true
  for (const ref of e.relatedObjectRefs || []) {
    if (objectMatches(ref.kind, ref.name, kind, name)) return true
  }
  return false
}

function kindAliases(a, b) {
  const x = String(a).toLowerCase()
  const y = String(b).toLowerCase()
  if (x === y) return true
  if ((x === 'pvc' || x === 'persistentvolumeclaim') && (y === 'pvc' || y === 'persistentvolumeclaim')) return true
  if ((x === 'hpa' || x === 'horizontalpodautoscaler') && (y === 'hpa' || y === 'horizontalpodautoscaler')) return true
  return false
}

function eventSeverity(type, reason) {
  const t = String(type || '').toLowerCase()
  const r = String(reason || '').toLowerCase()
  if (t === 'warning' || r.includes('fail') || r.includes('error') || r.includes('backoff') || r.includes('oom')) {
    return 'warning'
  }
  if (r.includes('kill') || r.includes('crash')) return 'high'
  return 'info'
}

function parseCPU(s) {
  if (!s) return 0
  const v = String(s).trim()
  if (v.endsWith('m')) return parseInt(v, 10) || 0
  const n = parseFloat(v)
  return Number.isNaN(n) ? 0 : Math.round(n * 1000)
}

function parseMemMi(s) {
  if (!s) return 0
  const v = String(s).trim()
  const n = parseFloat(v)
  if (Number.isNaN(n)) return 0
  if (/Ki$/i.test(v)) return Math.round(n / 1024)
  if (/Mi$/i.test(v)) return Math.round(n)
  if (/Gi$/i.test(v)) return Math.round(n * 1024)
  if (/Ti$/i.test(v)) return Math.round(n * 1024 * 1024)
  if (/^[0-9.]+$/.test(v)) return Math.round(n / (1024 * 1024))
  return Math.round(n)
}

function fmtCpu(m) {
  if (m >= 1000) return `${(m / 1000).toFixed(m % 1000 === 0 ? 0 : 2)}`
  return `${m}m`
}

function titleCase(key) {
  return String(key)
    .replace(/([A-Z])/g, ' $1')
    .replace(/^./, (c) => c.toUpperCase())
    .replace(/Cpu/g, 'CPU')
    .replace(/Ip/g, 'IP')
    .trim()
}

function formatTime(ts) {
  if (!ts) return '—'
  try {
    return new Date(ts).toLocaleTimeString(undefined, {
      hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false,
    })
  } catch {
    return '—'
  }
}

function truncate(s, n) {
  if (!s || s.length <= n) return s || ''
  return `${s.slice(0, n - 1)}…`
}
