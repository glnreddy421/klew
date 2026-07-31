import { kindBadge, matchKey, podsForMatch } from './matches'

const WORKLOAD_KINDS = new Set([
  'Deployment', 'StatefulSet', 'DaemonSet', 'Job', 'CronJob', 'ReplicaSet',
])

/**
 * Expand a focused match into its workload realm: owner chain, related pods,
 * services, config/secrets referenced by those pods, and label-linked objects
 * available on the live snapshot.
 */
export function buildFocusScope(view, focusRow) {
  if (!focusRow?.ref) return emptyFocusScope()

  const snap = view?.state?.snapshot || {}
  const pods = snap.pods || []
  const services = snap.services || []
  const workloads = snap.workloads || []
  const replicaSets = snap.replicaSets || []
  const ingresses = snap.ingresses || []

  const focusRef = { kind: focusRow.ref.kind, name: focusRow.ref.name, namespace: focusRow.ref.namespace }
  const relatedKeys = new Set([matchKey(focusRef)])
  const relatedPodNames = new Set()
  const relations = [] // { key, via }

  const addKey = (kind, name, namespace, via) => {
    if (!kind || !name) return
    const key = matchKey({ kind, name, namespace: namespace || focusRef.namespace })
    if (!relatedKeys.has(key)) {
      relatedKeys.add(key)
      if (via) relations.push({ key, via })
    }
    return key
  }

  // ── 1) Pods in this realm ──
  let realmPods = collectRealmPods(focusRef, pods, replicaSets, workloads)
  for (const p of realmPods) {
    relatedPodNames.add(p.name)
    addKey('Pod', p.name, p.namespace, 'owns')
    for (const o of p.ownerRefs || []) {
      addKey(o.kind, o.name, focusRef.namespace, 'owner')
      if (o.kind === 'ReplicaSet') {
        climbReplicaSet(o.name, replicaSets, workloads, focusRef.namespace, addKey)
      }
      if (o.kind === 'Job') {
        // CronJob name is often the job name prefix
        const cron = o.name.replace(/-[0-9]+$/, '')
        if (cron && cron !== o.name) addKey('CronJob', cron, focusRef.namespace, 'owner')
      }
    }
    // Config / secret / PVC mounts from pod
    for (const cm of p.configMapRefs || []) addKey('ConfigMap', cm, p.namespace, 'mounts')
    for (const sec of p.secretRefs || []) addKey('Secret', sec, p.namespace, 'mounts')
    for (const pvc of p.pvcRefs || []) addKey('PersistentVolumeClaim', pvc, p.namespace, 'mounts')
  }

  // Ensure focused workload itself is present
  if (WORKLOAD_KINDS.has(focusRef.kind) || focusRef.kind === 'Service') {
    addKey(focusRef.kind, focusRef.name, focusRef.namespace, 'focus')
  }

  // ── 2) Workloads that own realm pods / match focus ──
  for (const w of workloads) {
    const key = matchKey({ kind: w.kind, name: w.name, namespace: w.namespace || focusRef.namespace })
    if (w.kind === focusRef.kind && w.name === focusRef.name) {
      relatedKeys.add(key)
      continue
    }
    if (relatedKeys.has(key)) continue
    const owned = podsForMatch({ kind: w.kind, name: w.name }, realmPods.length ? realmPods : pods)
    if (owned.some((p) => relatedPodNames.has(p.name))) {
      addKey(w.kind, w.name, w.namespace, 'owns')
    }
  }

  // ── 3) Services by selector / name / selecting realm pods ──
  const labelBag = aggregatePodLabels(realmPods)
  for (const svc of services) {
    const nameHit = namesRelated(svc.name, focusRef.name)
    const selectorHit = selectorMatchesLabels(svc.selector, labelBag)
    if (focusRef.kind === 'Service' && svc.name === focusRef.name) {
      addKey('Service', svc.name, svc.namespace, 'focus')
      for (const p of pods) {
        if (namesRelated(p.name, svc.name) || labelsTouchName(p.labels, svc.name)) {
          relatedPodNames.add(p.name)
          addKey('Pod', p.name, p.namespace, 'selects')
        }
      }
      continue
    }
    if (nameHit || selectorHit) {
      addKey('Service', svc.name, svc.namespace, selectorHit ? 'selects' : 'name')
    }
  }

  // Refresh realm pod list after service expansion
  realmPods = pods.filter((p) => relatedPodNames.has(p.name))
  if (!realmPods.length) {
    realmPods = collectRealmPods(focusRef, pods, replicaSets, workloads)
    for (const p of realmPods) {
      relatedPodNames.add(p.name)
      addKey('Pod', p.name, p.namespace, 'owns')
    }
  }
  for (const p of realmPods) {
    for (const cm of p.configMapRefs || []) addKey('ConfigMap', cm, p.namespace, 'mounts')
    for (const sec of p.secretRefs || []) addKey('Secret', sec, p.namespace, 'mounts')
    for (const pvc of p.pvcRefs || []) addKey('PersistentVolumeClaim', pvc, p.namespace, 'mounts')
  }

  // ── 4) Ingresses targeting related services ──
  for (const ing of ingresses) {
    const backends = ing.backends || []
    const hit = backends.some((b) => {
      const name = typeof b === 'string' ? b : b?.name || b?.service || ''
      return [...relatedKeys].some((k) => k === `Service/${name}` || k.endsWith(`/${name}`))
        || namesRelated(name, focusRef.name)
    })
    if (hit || namesRelated(ing.name, focusRef.name)) {
      addKey('Ingress', ing.name, ing.namespace, 'routes')
    }
  }

  // Fallback: prefix match on focus name for pods (payment-api → payment-api-*)
  if (relatedPodNames.size === 0 && focusRef.name) {
    const prefix = focusRef.name.toLowerCase()
    for (const p of pods) {
      const n = (p.name || '').toLowerCase()
      if (n === prefix || n.startsWith(`${prefix}-`)) {
        relatedPodNames.add(p.name)
        addKey('Pod', p.name, p.namespace, 'prefix')
        for (const cm of p.configMapRefs || []) addKey('ConfigMap', cm, p.namespace, 'mounts')
        for (const sec of p.secretRefs || []) addKey('Secret', sec, p.namespace, 'mounts')
        for (const pvc of p.pvcRefs || []) addKey('PersistentVolumeClaim', pvc, p.namespace, 'mounts')
      }
    }
  }

  const rootRef = resolveRoot(focusRef, relatedKeys, workloads)
  const relatedKeysArr = [...relatedKeys]
  const relatedPodNamesArr = [...relatedPodNames].sort()

  return {
    active: true,
    focusKey: focusRow.key,
    focusRef,
    rootRef,
    relatedKeys,
    relatedKeysArr,
    relatedPodNames,
    relatedPodNamesArr,
    relatedPodCount: relatedPodNamesArr.length,
    label: `${focusRef.kind}/${focusRef.name}`,
    relations,
  }
}

export function emptyFocusScope() {
  return {
    active: false,
    focusKey: null,
    focusRef: null,
    rootRef: null,
    relatedKeys: new Set(),
    relatedKeysArr: [],
    relatedPodNames: new Set(),
    relatedPodNamesArr: [],
    relatedPodCount: 0,
    label: '',
    relations: [],
  }
}

/** Rows for the matched list while drilled: focus chain only. */
export function buildChainRows(view, focusScope, existingRows = []) {
  if (!focusScope?.active) return existingRows

  const byKey = new Map(existingRows.map((r) => [r.key, r]))
  const snap = view?.state?.snapshot || {}
  const keys = focusScope.relatedKeysArr?.length
    ? focusScope.relatedKeysArr
    : [...(focusScope.relatedKeys || [])]

  // Prefer stable order: focus root, other workloads, services, pods, config…
  const ordered = sortChainKeys(keys, focusScope.focusKey)

  const rows = []
  for (const key of ordered) {
    if (byKey.has(key)) {
      rows.push({ ...byKey.get(key), inFocusChain: true })
      continue
    }
    const synthesized = synthesizeRow(key, snap, focusScope)
    if (synthesized) rows.push(synthesized)
  }

  // Always include focus row even if missing from snapshot enrichment
  if (focusScope.focusKey && !rows.some((r) => r.key === focusScope.focusKey)) {
    const focusExisting = byKey.get(focusScope.focusKey)
    if (focusExisting) rows.unshift({ ...focusExisting, inFocusChain: true })
  }

  return rows
}

function synthesizeRow(key, snap, focusScope) {
  const [kind, ...rest] = key.split('/')
  const name = rest.join('/')
  if (!kind || !name) return null

  const pods = snap.pods || []
  const workloads = snap.workloads || []
  const services = snap.services || []

  let ready = null
  let total = null
  let restarts = 0
  let status = 'healthy'
  let signal = null

  if (kind === 'Pod') {
    const p = pods.find((x) => x.name === name)
    if (p) {
      ready = p.ready ? 1 : 0
      total = 1
      restarts = p.restartCount || 0
      if (!p.ready) {
        status = 'degraded'
        signal = 'not ready'
      }
    }
  } else if (WORKLOAD_KINDS.has(kind)) {
    const w = workloads.find((x) => x.kind === kind && x.name === name)
    if (w) {
      ready = w.ready ?? 0
      total = w.replicas ?? 0
      if (ready < total) {
        status = 'degraded'
        signal = `${ready}/${total} ready`
      }
    }
    const related = pods.filter((p) => (focusScope.relatedPodNamesArr || []).includes(p.name)
      || p.name.startsWith(`${name}-`))
    restarts = related.reduce((n, p) => n + (p.restartCount || 0), 0)
  } else if (kind === 'Service') {
    const svc = services.find((x) => x.name === name)
    if (svc) {
      ready = svc.readyEndpoints ?? null
      total = svc.totalEndpoints ?? null
    }
  }

  return {
    key,
    ref: { kind, name, namespace: focusScope.focusRef?.namespace },
    score: 0,
    matchBy: 'focus-chain',
    kind,
    name,
    kindBadge: kindBadge(kind),
    ready,
    total,
    restarts,
    status,
    signal,
    inFocusChain: true,
  }
}

function sortChainKeys(keys, focusKey) {
  const rank = (key) => {
    if (key === focusKey) return 0
    const kind = key.split('/')[0]
    if (WORKLOAD_KINDS.has(kind)) return 1
    if (kind === 'Service' || kind === 'Ingress') return 2
    if (kind === 'Pod') return 3
    if (kind === 'ConfigMap' || kind === 'Secret' || kind === 'PersistentVolumeClaim') return 4
    return 5
  }
  return [...keys].sort((a, b) => {
    const d = rank(a) - rank(b)
    if (d !== 0) return d
    return a.localeCompare(b)
  })
}

function collectRealmPods(focusRef, pods, replicaSets, workloads) {
  if (!focusRef?.name) return []

  // Direct heuristic from matches.js
  let found = podsForMatch(focusRef, pods)

  // Label-based: app / app.kubernetes.io/name equals focus name
  const byLabel = pods.filter((p) => labelsTouchName(p.labels, focusRef.name))
  found = uniqPods([...found, ...byLabel])

  // If focus is a workload, also take pods whose RS owner maps to it
  if (WORKLOAD_KINDS.has(focusRef.kind) || focusRef.kind === 'Deployment') {
    const fromRS = pods.filter((p) => {
      for (const o of p.ownerRefs || []) {
        if (o.kind === 'ReplicaSet') {
          const rs = replicaSets.find((r) => r.name === o.name)
          if (rs?.deploymentOwner === focusRef.name) return true
          if (o.name.startsWith(`${focusRef.name}-`)) return true
        }
        if (o.kind === focusRef.kind && o.name === focusRef.name) return true
        if ((o.kind === 'StatefulSet' || o.kind === 'DaemonSet' || o.kind === 'Job')
          && o.name === focusRef.name) return true
      }
      return false
    })
    found = uniqPods([...found, ...fromRS])
  }

  // Prefix fallback
  if (!found.length) {
    const prefix = focusRef.name.toLowerCase()
    found = pods.filter((p) => {
      const n = (p.name || '').toLowerCase()
      return n === prefix || n.startsWith(`${prefix}-`)
    })
  }

  return found
}

function climbReplicaSet(rsName, replicaSets, workloads, namespace, addKey) {
  const rs = replicaSets.find((r) => r.name === rsName)
  if (rs?.deploymentOwner) {
    addKey('Deployment', rs.deploymentOwner, namespace, 'owner')
    return
  }
  const deploy = workloads.find((w) =>
    w.kind === 'Deployment' && rsName.startsWith(`${w.name}-`),
  )
  if (deploy) {
    addKey('Deployment', deploy.name, namespace, 'owner')
    return
  }
  const deployName = rsName.replace(/-[a-z0-9]{5,10}$/i, '')
  if (deployName && deployName !== rsName) {
    addKey('Deployment', deployName, namespace, 'owner')
  }
}

function uniqPods(list) {
  const map = new Map()
  for (const p of list) {
    if (p?.name) map.set(p.name, p)
  }
  return [...map.values()]
}

function aggregatePodLabels(pods) {
  const out = {}
  for (const p of pods) {
    for (const [k, v] of Object.entries(p.labels || {})) {
      out[k] = v
    }
  }
  return out
}

function labelsTouchName(labels, name) {
  if (!labels || !name) return false
  const n = name.toLowerCase()
  for (const [k, v] of Object.entries(labels)) {
    const val = String(v).toLowerCase()
    if (val === n || val.startsWith(`${n}-`)) return true
    if ((k === 'app' || k === 'app.kubernetes.io/name' || k.endsWith('/name')) && namesRelated(val, n)) {
      return true
    }
  }
  return false
}

/** Parse "k=v,k2=v2" style selectors from ServiceSummary.selector string. */
function selectorMatchesLabels(selector, labels) {
  if (!selector || !labels || !Object.keys(labels).length) return false
  const parts = String(selector).split(/,/)
  if (!parts.length) return false
  for (const part of parts) {
    const [k, ...rest] = part.split('=')
    const key = k?.trim()
    const val = rest.join('=').trim()
    if (!key) return false
    if (labels[key] !== val) return false
  }
  return true
}

function resolveRoot(focusRef, relatedKeys, workloads) {
  if (WORKLOAD_KINDS.has(focusRef.kind) && focusRef.kind !== 'ReplicaSet') {
    return focusRef
  }
  for (const w of workloads) {
    const key = matchKey({ kind: w.kind, name: w.name })
    if (relatedKeys.has(key) && (w.kind === 'Deployment' || w.kind === 'StatefulSet' || w.kind === 'DaemonSet')) {
      return { kind: w.kind, name: w.name, namespace: w.namespace || focusRef.namespace }
    }
  }
  return focusRef
}

function namesRelated(a, b) {
  if (!a || !b) return false
  const x = String(a).toLowerCase()
  const y = String(b).toLowerCase()
  return x === y || x.startsWith(`${y}-`) || y.startsWith(`${x}-`)
}

export function focusMetrics(view, focusScope) {
  const pods = view?.state?.snapshot?.pods || []
  const names = new Set(focusScope?.relatedPodNamesArr || [...(focusScope?.relatedPodNames || [])])
  if (!names.size) {
    return { ready: null, total: null, restarts: null, endpointsReady: null, endpointsTotal: null }
  }
  const scoped = pods.filter((p) => names.has(p.name))
  const ready = scoped.filter((p) => p.ready).length
  const total = scoped.length
  const restarts = scoped.reduce((n, p) => n + (p.restartCount || 0), 0)

  const services = view?.state?.snapshot?.services || []
  const keys = new Set(focusScope?.relatedKeysArr || [...(focusScope?.relatedKeys || [])])
  let endpointsReady = null
  let endpointsTotal = null
  for (const svc of services) {
    if (!keys.has(`Service/${svc.name}`)) continue
    endpointsReady = (endpointsReady ?? 0) + (svc.readyEndpoints ?? 0)
    endpointsTotal = (endpointsTotal ?? 0) + (svc.totalEndpoints ?? 0)
  }

  return { ready, total, restarts, endpointsReady, endpointsTotal }
}
