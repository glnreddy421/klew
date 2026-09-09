import { normalizeBrowseScope } from './browseScope.js'
import { buildInspectKey } from './matches.js'

/** Format pod creation timestamp as a short relative age. */
export function formatEntityAge(createdAt) {
  if (!createdAt) return '—'
  const raw = typeof createdAt === 'object' ? createdAt.time || createdAt.Time : createdAt
  const ms = Date.parse(raw)
  if (!Number.isFinite(ms)) return '—'
  const sec = Math.max(0, Math.floor((Date.now() - ms) / 1000))
  if (sec < 60) return `${sec}s`
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min}m`
  const hr = Math.floor(min / 60)
  if (hr < 48) return `${hr}h`
  const day = Math.floor(hr / 24)
  return `${day}d`
}

const WORKLOAD_READY_KINDS = new Set([
  'Deployment',
  'StatefulSet',
  'ReplicaSet',
  'DaemonSet',
])

/** Kinds that render separate Desired / Current / Ready columns (not a combined ready fraction). */
const REPLICA_TABLE_KINDS = new Set(['ReplicaSet'])

function formatReplicaCount(value) {
  if (value == null) return '—'
  return String(value)
}

/** Parse catalog hints like `0/2 ready` (ready / current replicas). */
export function parseWorkloadReplicaSignal(signal) {
  const m = String(signal || '').match(/(\d+)\s*\/\s*(\d+)\s*ready/i)
  if (!m) return null
  return { ready: Number(m[1]), current: Number(m[2]) }
}

function resolveReplicaCounts(row) {
  if (!REPLICA_TABLE_KINDS.has(row.kind)) return null

  const parsed = parseWorkloadReplicaSignal(row.signal)
  let desired = row.desiredReplicas
  let current = row.currentReplicas
  let ready = row.readyReplicas

  if (current == null && parsed) current = parsed.current
  if (ready == null && parsed) ready = parsed.ready
  if (ready == null && current === 0) ready = 0

  if (desired == null && current == null && ready == null) return null

  return {
    desired: formatReplicaCount(desired),
    current: formatReplicaCount(current),
    ready: formatReplicaCount(ready),
  }
}

function replicaTableFields(row, pod) {
  const replicas = resolveReplicaCounts(row)
  if (replicas) return replicas
  return { ready: readyLabelForRow(row, pod) }
}

const DEPLOYMENT_CONDITION_TYPES = ['Available', 'Progressing']

/** Resolve Available + Progressing from API conditions or replica counts. */
export function resolveDeploymentConditions(row) {
  if (row?.kind !== 'Deployment') return []

  const fromApi = row.conditions || []
  const byType = {}
  for (const cond of fromApi) {
    if (cond?.type) byType[cond.type] = cond
  }

  const desired = row.desiredReplicas
  const ready = row.readyReplicas
  const current = row.currentReplicas
  const available = row.availableReplicas ?? ready
  const updated = row.updatedReplicas
  const unavailable = row.unavailableReplicas

  const resolved = []

  if (byType.Available) {
    resolved.push(byType.Available)
  } else if (desired != null && available != null) {
    const ok = desired === 0 ? available === 0 : available >= desired
    resolved.push({
      type: 'Available',
      status: ok ? 'True' : 'False',
      reason: ok ? 'MinimumReplicasAvailable' : 'MinimumReplicasUnavailable',
    })
  }

  if (byType.Progressing) {
    resolved.push(byType.Progressing)
  } else if (desired != null) {
    let ok = false
    if (updated != null && ready != null) {
      ok = updated >= desired && ready >= desired && (unavailable ?? 0) === 0
    } else if (ready != null) {
      ok = ready >= desired && (current ?? ready) >= desired
    }
    resolved.push({
      type: 'Progressing',
      status: ok ? 'True' : 'False',
      reason: ok ? 'NewReplicaSetAvailable' : 'ReplicaSetNotAvailable',
    })
  }

  return resolved.length
    ? resolved
    : DEPLOYMENT_CONDITION_TYPES.map((type) => byType[type]).filter(Boolean)
}

export function formatDeploymentConditions(conditions = []) {
  if (!conditions?.length) return '—'
  return conditions.map((c) => c.type).filter(Boolean).join(', ')
}

const JOB_CONDITION_TYPES = ['Complete', 'Failed']

/** Parse catalog hints like `1/1` or `1 succeeded`. */
export function parseJobCompletionSignal(signal) {
  const fraction = String(signal || '').match(/(\d+)\s*\/\s*(\d+)/)
  if (fraction) {
    return { succeeded: Number(fraction[1]), completions: Number(fraction[2]) }
  }
  const succeededOnly = String(signal || '').match(/(\d+)\s*succeeded/i)
  if (succeededOnly) {
    return { succeeded: Number(succeededOnly[1]), completions: 1 }
  }
  return null
}

/** Resolve Complete + Failed from API conditions or completion counts. */
export function resolveJobConditions(row) {
  if (row?.kind !== 'Job') return []

  const fromApi = row.conditions || []
  const byType = {}
  for (const cond of fromApi) {
    if (cond?.type) byType[cond.type] = cond
  }

  const parsed = parseJobCompletionSignal(row.signal)
  const succeeded = row.succeeded ?? parsed?.succeeded
  const completions = row.completions ?? parsed?.completions ?? 1
  const resolved = []

  if (byType.Complete) {
    resolved.push(byType.Complete)
  } else if (succeeded != null && succeeded >= completions) {
    resolved.push({
      type: 'Complete',
      status: 'True',
      reason: 'JobCompleted',
    })
  }

  if (byType.Failed) {
    resolved.push(byType.Failed)
  }

  return resolved.length
    ? resolved
    : JOB_CONDITION_TYPES.map((type) => byType[type]).filter(Boolean)
}

export function formatJobCompletions(row) {
  const parsed = parseJobCompletionSignal(row.signal)
  const succeeded = row.succeeded ?? parsed?.succeeded
  const completions = row.completions ?? parsed?.completions ?? 1
  if (succeeded == null) return '—'
  return `${succeeded}/${completions}`
}

export function formatJobConditions(conditions = []) {
  return formatDeploymentConditions(conditions)
}

export function formatCronJobSuspend(suspend) {
  if (suspend == null) return 'False'
  return suspend ? 'True' : 'False'
}

export function formatCronJobActive(activeJobs) {
  if (activeJobs == null) return '0'
  return String(activeJobs)
}

export function formatCronJobSchedule(row) {
  return row.schedule || row.signal || '—'
}

export function formatCronJobLastSchedule(lastScheduleTime) {
  if (!lastScheduleTime) return '—'
  return formatEntityAge(lastScheduleTime)
}

function cronJobTableFields(row) {
  if (row.kind !== 'CronJob') return {}
  return {
    schedule: formatCronJobSchedule(row),
    suspend: formatCronJobSuspend(row.suspend),
    active: formatCronJobActive(row.activeJobs),
    lastSchedule: formatCronJobLastSchedule(row.lastScheduleTime),
  }
}

export function formatServicePorts(ports) {
  if (!ports?.length) return '—'
  return ports.join(', ')
}

export function formatServiceExternalIP(externalIPs) {
  if (!externalIPs?.length) return '—'
  return externalIPs.join(', ')
}

export function formatServiceSelector(selector) {
  if (!selector) return '—'
  return selector
}

/** Short label for selector cells — full value stays in title/tooltip. */
export function truncateSelector(selector, max = 12) {
  const value = String(selector || '').trim()
  if (!value) return '—'
  if (value.length <= max) return value
  return `${value.slice(0, Math.max(1, max - 1))}…`
}

export function formatServiceType(row) {
  return row.serviceType || row.type || '—'
}

export function formatServiceClusterIP(clusterIP) {
  if (!clusterIP) return '—'
  return clusterIP
}

const SERVICE_TYPE_LABELS = new Set(['ClusterIP', 'NodePort', 'LoadBalancer', 'ExternalName'])

/** Service status = endpoint readiness, not spec.type (that belongs in the Type column). */
export function serviceStatusLabel(row) {
  const type = row.serviceType || row.type || ''
  const ready = row.readyEndpoints
  const total = row.totalEndpoints

  if (type === 'ExternalName') return 'External'

  if (ready != null && total != null) {
    if (total === 0) return 'No endpoints'
    return `${ready}/${total} ready`
  }

  const signal = String(row.signal || '').trim()
  if (signal && !SERVICE_TYPE_LABELS.has(signal)) return signal

  return '—'
}

export function formatEndpointSummary(row) {
  if (row.endpointSummary) return row.endpointSummary
  const ready = row.readyEndpoints
  const total = row.totalEndpoints
  if (ready != null && total != null) {
    if (total === 0) return '<none>'
    return `${ready}/${total} ready`
  }
  if (row.signal && row.signal !== '<none>') return row.signal
  return '—'
}

export function formatLoadBalancers(loadBalancers) {
  if (!loadBalancers?.length) return '—'
  return loadBalancers.join(', ')
}

export function formatIngressRules(summary) {
  if (!summary) return '—'
  return summary
}

function ingressTableFields(row) {
  if (row.kind !== 'Ingress') return {}
  return {
    loadBalancers: formatLoadBalancers(row.loadBalancers),
    rules: formatIngressRules(row.ingressRulesSummary),
  }
}

function displayOrDash(value) {
  if (value == null || value === '') return '—'
  return value
}

function ingressClassTableFields(row) {
  if (row.kind !== 'IngressClass') return {}
  return {
    namespace: displayOrDash(row.parameterNamespace),
    controller: displayOrDash(row.ingressController),
    apiGroup: displayOrDash(row.parameterAPIGroup),
    scope: displayOrDash(row.parameterScope),
    parameterKind: displayOrDash(row.parameterKind),
  }
}

export function formatPolicyTypes(policyTypes) {
  if (!policyTypes?.length) return '—'
  return policyTypes.join(', ')
}

function networkPolicyTableFields(row) {
  if (row.kind !== 'NetworkPolicy') return {}
  return {
    policyTypes: formatPolicyTypes(row.policyTypes),
  }
}

export function formatDefaultClass(isDefault) {
  if (isDefault === true) return 'true'
  if (isDefault === false) return 'false'
  return '—'
}

function isStorageClassRow(row, tableKind) {
  return row.kind === 'StorageClass'
    || tableKind === 'StorageClass'
    || /(^|\/)storageclasses$/i.test(row.resourceId || '')
}

function storageClassTableFields(row, tableKind) {
  if (!isStorageClassRow(row, tableKind)) return {}
  return {
    provisioner: displayOrDash(row.provisioner),
    reclaimPolicy: displayOrDash(row.reclaimPolicy),
    volumeBindingMode: displayOrDash(row.volumeBindingMode),
    defaultClass: formatDefaultClass(row.isDefault),
  }
}

export function formatConfigMapData(entries) {
  if (!entries?.length) return '—'
  const n = entries.length
  return `${n} key${n === 1 ? '' : 's'}`
}

export function formatConfigMapDataLines(entries) {
  if (!entries?.length) return []
  return entries.map((e) => `${e.key}\t${e.sizeBytes} bytes`)
}

export function formatAccessModesList(modes) {
  if (!modes?.length) return '—'
  return modes.join(', ')
}

function formatOptionalCount(value) {
  if (value == null || value === '') return '—'
  return String(value)
}

function formatHPARReplicas(row) {
  const cur = row.currentReplicas
  const des = row.desiredReplicas
  if (cur != null && des != null) return `${cur}/${des}`
  if (cur != null) return String(cur)
  if (des != null) return String(des)
  return '—'
}

function rowKindMatches(row, tableKind, kind) {
  return row.kind === kind || tableKind === kind
}

/** Backend-derived column values (spec/status paths from catalog_table_fields.go). */
function catalogTableFields(row) {
  const out = {}
  for (const [key, value] of Object.entries(row.tableFields || {})) {
    out[key] = displayOrDash(value)
  }
  return out
}

function nodeTableFields(row, tableKind) {
  if (!rowKindMatches(row, tableKind, 'Node')) return {}
  const nr = row.nodeResources || {}
  const readyLabel = nr.ready == null ? '—' : (nr.ready ? 'Ready' : 'NotReady')
  return {
    roles: displayOrDash(nr.roles || row.tableFields?.roles),
    version: displayOrDash(nr.kubeletVersion || row.tableFields?.version),
    taints: nr.taintCount ?? row.tableFields?.taints ?? '—',
    nodeReady: nr.ready,
    conditions: readyLabel,
  }
}

function formatHelmReleaseStatus(value) {
  const raw = String(value || '').trim()
  if (!raw || raw === '—') return '—'
  return raw.charAt(0).toUpperCase() + raw.slice(1)
}

function helmReleaseTableFields(row, tableKind) {
  if (!rowKindMatches(row, tableKind, 'HelmRelease')) return {}
  const tf = row.tableFields || {}
  const statusRaw = row.signal || tf.status || row.status || ''
  return {
    chart: displayOrDash(tf.chart),
    chartVersion: displayOrDash(tf.chartVersion),
    appVersion: displayOrDash(tf.appVersion),
    revision: displayOrDash(tf.revision),
    updated: formatEntityAge(tf.updated || row.creationTimestamp),
    statusLabel: formatHelmReleaseStatus(statusRaw),
  }
}

function builtinKindTableFields(row, tableKind) {
  if (rowKindMatches(row, tableKind, 'HelmRelease')) {
    return helmReleaseTableFields(row, tableKind)
  }
  if (rowKindMatches(row, tableKind, 'PersistentVolumeClaim')) {
    return {
      volume: displayOrDash(row.volumeName),
      capacity: displayOrDash(row.capacity),
      accessModes: formatAccessModesList(row.accessModes),
      storageClass: displayOrDash(row.storageClassName),
    }
  }
  if (rowKindMatches(row, tableKind, 'PersistentVolume')) {
    return {
      capacity: displayOrDash(row.capacity),
      accessModes: formatAccessModesList(row.accessModes),
      reclaimPolicy: displayOrDash(row.reclaimPolicy),
      claim: displayOrDash(row.claimRef),
      storageClass: displayOrDash(row.storageClassName),
    }
  }
  if (rowKindMatches(row, tableKind, 'ConfigMap')) {
    return {
      dataKeys: formatConfigMapData(row.configMapData),
      dataKeysDetail: row.configMapData || [],
    }
  }
  if (rowKindMatches(row, tableKind, 'Secret')) {
    return {
      secretType: displayOrDash(row.secretType),
      keys: formatOptionalCount(row.dataKeys),
    }
  }
  if (rowKindMatches(row, tableKind, 'HorizontalPodAutoscaler')) {
    return {
      scaleTarget: displayOrDash(row.scaleTarget),
      targets: displayOrDash(row.metricsSummary),
      minPods: formatOptionalCount(row.minReplicas),
      maxPods: formatOptionalCount(row.maxReplicas),
      hpaReplicas: formatHPARReplicas(row),
    }
  }
  if (rowKindMatches(row, tableKind, 'PodDisruptionBudget')) {
    return {
      minAvailable: displayOrDash(row.pdbMinAvailable),
      maxUnavailable: displayOrDash(row.pdbMaxUnavailable),
      allowedDisruptions: formatOptionalCount(row.pdbDisruptionsAllowed),
    }
  }
  if (rowKindMatches(row, tableKind, 'Lease')) {
    return { holder: displayOrDash(row.leaseHolder) }
  }
  return {}
}

function endpointResourceTableFields(row) {
  if (row.kind !== 'Endpoints' && row.kind !== 'EndpointSlice') return {}
  return {
    endpoints: formatEndpointSummary(row),
    service: row.serviceName || '—',
    addressType: row.addressType || '—',
  }
}

function serviceTableFields(row) {
  if (row.kind !== 'Service') return {}
  return {
    type: formatServiceType(row),
    clusterIP: formatServiceClusterIP(row.clusterIP),
    ports: formatServicePorts(row.ports),
    externalIP: formatServiceExternalIP(row.externalIPs),
    selector: formatServiceSelector(row.selector),
    statusLabel: serviceStatusLabel(row),
  }
}

const WORKLOAD_PODS_KINDS = new Set(['Deployment', 'StatefulSet'])

function formatWorkloadPods(row) {
  const parsed = parseWorkloadReplicaSignal(row.signal)
  const ready = row.readyReplicas ?? parsed?.ready
  const total = row.currentReplicas ?? parsed?.current
  if (ready == null && total == null) return '—'
  return `${ready ?? 0}/${total ?? 0}`
}

function resolveDaemonSetCounts(row) {
  const parsed = parseWorkloadReplicaSignal(row.signal)
  let desired = row.desiredReplicas
  let current = row.currentReplicas
  let ready = row.readyReplicas
  if (current == null && parsed) current = parsed.current
  if (ready == null && parsed) ready = parsed.ready
  if (desired == null && parsed) desired = parsed.current
  if (ready == null && current === 0) ready = 0

  let updated = row.updatedReplicas
  let available = row.availableReplicas
  if (updated == null && ready != null) updated = ready
  if (available == null && ready != null) available = ready

  const hasCounts = [desired, current, ready, updated, available].some((v) => v != null)

  return {
    desired: formatReplicaCount(desired),
    current: formatReplicaCount(current),
    ready: formatReplicaCount(ready),
    updated: formatReplicaCount(updated),
    available: formatReplicaCount(available),
    misscheduled: formatReplicaCount(row.misscheduled ?? (hasCounts ? 0 : null)),
  }
}

function daemonSetTableFields(row) {
  if (row.kind !== 'DaemonSet') return {}
  return resolveDaemonSetCounts(row)
}

function jobTableFields(row) {
  if (row.kind !== 'Job') return {}
  const conditions = resolveJobConditions(row)
  return {
    completions: formatJobCompletions(row),
    conditions: formatJobConditions(conditions),
    conditionsDetail: conditions,
  }
}

function workloadPodsTableFields(row) {
  if (!WORKLOAD_PODS_KINDS.has(row.kind)) return {}
  const base = {
    pods: formatWorkloadPods(row),
    replicas: formatReplicaCount(row.desiredReplicas),
  }
  if (row.kind !== 'Deployment') return base
  const conditions = resolveDeploymentConditions(row)
  return {
    ...base,
    conditions: formatDeploymentConditions(conditions),
    conditionsDetail: conditions,
  }
}

export function isClusterScopedKindGroup(kindGroup, kind) {
  if (kindGroup?.namespaced === false) return true
  if (kindGroup?.namespaced === true) return false
  return kind === 'Node' || kind === 'Namespace'
}

export function showsNamespaceColumn(kindGroup, kind, browseScope) {
  if (isClusterScopedKindGroup(kindGroup, kind)) return false
  const scope = normalizeBrowseScope(browseScope)
  return scope.mode === 'all' || scope.mode === 'multi'
}

function podForRow(row, pods) {
  if (row?.kind !== 'Pod' || !row?.name || !pods?.length) return null
  const ns = row.namespace || row.ref?.namespace || ''
  return pods.find((p) => p.name === row.name && (!ns || !p.namespace || p.namespace === ns)) || null
}

function aggregateContainerResource(containers, field) {
  if (!containers?.length) return '—'
  const values = containers.map((c) => c[field]).filter(Boolean)
  if (!values.length) return '—'
  if (values.length === 1) return values[0]
  return values.join(', ')
}

export function resolveContainerStatuses(row, pod) {
  if (pod?.containers?.length) {
    return pod.containers.map((c) => ({
      name: c.name,
      state: c.state || 'unknown',
      ready: Boolean(c.ready),
      reason: c.reason || c.lastReason || '',
      message: c.message || '',
      init: false,
    }))
  }
  if (row.containerStatuses?.length) {
    return row.containerStatuses.map((c) => ({
      name: c.name,
      state: c.state || 'unknown',
      ready: Boolean(c.ready),
      reason: c.reason || '',
      message: c.message || '',
      init: Boolean(c.init),
    }))
  }
  const names = row.containerNames || []
  if (!names.length) return []
  return names.map((name) => ({
    name,
    state: 'unknown',
    ready: false,
    reason: '',
    message: '',
    init: false,
  }))
}

function containersSummary(containers) {
  if (!containers?.length) return '—'
  return containers.map((c) => c.name).join(', ')
}

function resolveControlledBy(row, pod) {
  const owners = pod?.ownerRefs || []
  let kind = ''
  let name = ''
  if (owners.length) {
    kind = owners[0]?.kind || ''
    name = owners[0]?.name || ''
  } else {
    kind = row.ownerKind || ''
    name = row.ownerName || ''
  }
  if (!kind) {
    return { label: '—', inspectKey: null, title: '' }
  }
  const ns = row.namespace || row.ref?.namespace || pod?.namespace || ''
  const inspectKey = name ? buildInspectKey(kind, name, ns) : null
  return {
    label: kind,
    inspectKey,
    title: name ? `${kind}/${name}` : kind,
  }
}

function formatControlledBy(row, pod) {
  return resolveControlledBy(row, pod).label
}

export function statusLabelForRow(row, pod) {
  if (row.kind === 'HelmRelease') {
    return formatHelmReleaseStatus(row.signal || row.tableFields?.status || row.status)
  }
  if (row.kind === 'Service') return serviceStatusLabel(row)
  if (pod?.phase) return pod.phase
  if (WORKLOAD_READY_KINDS.has(row.kind) && row.signal) return row.signal
  if (row.kind === 'Job' && row.signal) return row.signal
  if (row.signal) return row.signal
  const map = {
    healthy: 'Running',
    degraded: 'Degraded',
    critical: 'Failed',
    unknown: 'Unknown',
  }
  return map[row.status] || '—'
}

function readyLabelForRow(row, pod) {
  if (WORKLOAD_READY_KINDS.has(row.kind)) {
    const match = String(row.signal || '').match(/(\d+\/\d+)/)
    if (match) return match[1]
    if (row.ready != null && row.total != null) return `${row.ready}/${row.total}`
    return '—'
  }
  if (row.kind === 'Job') {
    const match = String(row.signal || '').match(/(\d+)\s*succeeded/i)
    if (match) return match[1]
    return row.signal || '—'
  }
  if (row.kind === 'Pod') {
    if (row.ready != null && row.total != null) return `${row.ready}/${row.total}`
    if (pod?.ready != null && pod?.total != null) return `${pod.ready}/${pod.total}`
  }
  return '—'
}

/** Merge snapshot pod fields into entity rows for table columns. */
export function enrichEntityForTable(row, pods = [], tableKind = '') {
  const pod = podForRow(row, pods)
  const namespace = row.namespace || row.ref?.namespace || pod?.namespace || '—'
  const ageSource = pod?.createdAt || row.creationTimestamp
  const restarts = row.restarts ?? pod?.restartCount ?? row.restartCount ?? '—'
  const owner = resolveControlledBy(row, pod)
  const containers = resolveContainerStatuses(row, pod)
  const replicaCounts = replicaTableFields(row, pod)
  const workloadPodsFields = workloadPodsTableFields(row)
  const daemonSetFields = daemonSetTableFields(row)
  const jobFields = jobTableFields(row)
  const cronJobFields = cronJobTableFields(row)
  const serviceFields = serviceTableFields(row)
  const endpointFields = endpointResourceTableFields(row)
  const ingressFields = ingressTableFields(row)
  const ingressClassFields = ingressClassTableFields(row)
  const networkPolicyFields = networkPolicyTableFields(row)
  const storageClassFields = storageClassTableFields(row, tableKind)
  const builtinFields = builtinKindTableFields(row, tableKind)
  const catalogFields = catalogTableFields(row)
  const nodeFields = nodeTableFields(row, tableKind)
  const isNodeRow = rowKindMatches(row, tableKind, 'Node')
  return {
    ...row,
    table: {
      ...catalogFields,
      ...nodeFields,
      namespace: ingressClassFields.namespace ?? namespace,
      node: pod?.node || row.node || row.nodeName || '—',
      age: formatEntityAge(ageSource),
      ...replicaCounts,
      ...workloadPodsFields,
      ...daemonSetFields,
      ...jobFields,
      ...cronJobFields,
      ...serviceFields,
      ...endpointFields,
      ...ingressFields,
      ...ingressClassFields,
      ...networkPolicyFields,
      ...storageClassFields,
      ...builtinFields,
      restarts,
      containers,
      containersSummary: containersSummary(containers),
      controlledBy: owner.label,
      controlledByKey: owner.inspectKey,
      controlledByTitle: owner.title,
      qos: row.qosClass || '—',
      cpu: isNodeRow
        ? '—'
        : (aggregateContainerResource(pod?.containers, 'limitsCPU')
          || aggregateContainerResource(pod?.containers, 'requestsCPU')),
      memory: isNodeRow
        ? '—'
        : (aggregateContainerResource(pod?.containers, 'limitsMem')
          || aggregateContainerResource(pod?.containers, 'requestsMem')),
      statusLabel: isNodeRow
        ? (nodeFields.conditions !== '—' ? nodeFields.conditions : statusLabelForRow(row, pod))
        : statusLabelForRow(row, pod),
    },
  }
}

export function enrichEntitiesForTable(entities, pods = [], tableKind = '') {
  return (entities || []).map((row) => enrichEntityForTable(row, pods, tableKind))
}

export function tableCellValue(row, columnId) {
  const t = row.table || {}
  switch (columnId) {
    case 'name':
      return row.name || '—'
    case 'status':
      return t.statusLabel || '—'
    case 'namespace':
      return t.namespace || '—'
    case 'node':
      return t.node || '—'
    case 'desired':
      return t.desired ?? '—'
    case 'current':
      return t.current ?? '—'
    case 'ready':
      return t.ready ?? '—'
    case 'restarts':
      return t.restarts ?? '—'
    case 'containers':
      return t.containersSummary || '—'
    case 'controlledBy':
      return t.controlledBy || '—'
    case 'qos':
      return t.qos || '—'
    case 'cpu':
      return t.cpu || '—'
    case 'memory':
      return t.memory || '—'
    case 'age':
      return t.age || '—'
    case 'pods':
      return t.pods ?? '—'
    case 'replicas':
      return t.replicas ?? '—'
    case 'conditions':
      return t.conditions ?? '—'
    case 'updated':
      return t.updated ?? '—'
    case 'available':
      return t.available ?? '—'
    case 'misscheduled':
      return t.misscheduled ?? '—'
    case 'completions':
      return t.completions ?? '—'
    case 'schedule':
      return t.schedule ?? '—'
    case 'suspend':
      return t.suspend ?? '—'
    case 'active':
      return t.active ?? '—'
    case 'lastSchedule':
      return t.lastSchedule ?? '—'
    case 'type':
      return t.type ?? '—'
    case 'clusterIP':
      return t.clusterIP ?? '—'
    case 'ports':
      return t.ports ?? '—'
    case 'externalIP':
      return t.externalIP ?? '—'
    case 'selector':
      return t.selector ?? '—'
    case 'endpoints':
      return t.endpoints ?? '—'
    case 'service':
      return t.service ?? '—'
    case 'addressType':
      return t.addressType ?? '—'
    case 'loadBalancers':
      return t.loadBalancers ?? '—'
    case 'rules':
      return t.rules ?? '—'
    case 'controller':
      return t.controller ?? '—'
    case 'apiGroup':
      return t.apiGroup ?? '—'
    case 'scope':
      return t.scope ?? '—'
    case 'parameterKind':
      return t.parameterKind ?? '—'
    case 'policyTypes':
      return t.policyTypes ?? '—'
    case 'provisioner':
      return t.provisioner ?? '—'
    case 'reclaimPolicy':
      return t.reclaimPolicy ?? '—'
    case 'volumeBindingMode':
      return t.volumeBindingMode ?? '—'
    case 'defaultClass':
      return t.defaultClass ?? '—'
    case 'volume':
      return t.volume ?? '—'
    case 'capacity':
      return t.capacity ?? '—'
    case 'accessModes':
      return t.accessModes ?? '—'
    case 'storageClass':
      return t.storageClass ?? '—'
    case 'claim':
      return t.claim ?? '—'
    case 'keys':
      return t.keys ?? '—'
    case 'dataKeys':
      return t.dataKeys ?? '—'
    case 'secretType':
      return t.secretType ?? '—'
    case 'scaleTarget':
      return t.scaleTarget ?? '—'
    case 'targets':
      return t.targets ?? '—'
    case 'minPods':
      return t.minPods ?? '—'
    case 'maxPods':
      return t.maxPods ?? '—'
    case 'hpaReplicas':
      return t.hpaReplicas ?? '—'
    case 'minAvailable':
      return t.minAvailable ?? '—'
    case 'maxUnavailable':
      return t.maxUnavailable ?? '—'
    case 'allowedDisruptions':
      return t.allowedDisruptions ?? '—'
    case 'holder':
      return t.holder ?? '—'
    default:
      return t[columnId] ?? '—'
  }
}
