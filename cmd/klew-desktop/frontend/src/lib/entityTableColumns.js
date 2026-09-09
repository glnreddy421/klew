import { normalizeBrowseScope } from './browseScope.js'
import { isClusterScopedKindGroup, showsNamespaceColumn } from './entityTable.js'
import { BUILTIN_KIND_TABLE_SCHEMAS } from './builtinKindTableSchemas.js'

export const ENTITY_TABLE_COLUMNS_KEY = 'klew.entityTable.columns'

/** Shared column defs reused across kind schemas. */
const COL = {
  name: { id: 'name', label: 'Name', className: 'col-name', required: true },
  namespace: { id: 'namespace', label: 'Namespace', className: 'col-ns' },
  status: { id: 'status', label: 'Status', className: 'col-status' },
  age: { id: 'age', label: 'Age', className: 'col-age' },
  ready: { id: 'ready', label: 'Ready', className: 'col-num' },
  desired: { id: 'desired', label: 'Desired', className: 'col-num' },
  current: { id: 'current', label: 'Current', className: 'col-num' },
  node: { id: 'node', label: 'Node', className: 'col-node' },
  containers: { id: 'containers', label: 'Containers', className: 'col-containers' },
  restarts: { id: 'restarts', label: 'Restarts', className: 'col-num' },
  controlledBy: { id: 'controlledBy', label: 'Owner', className: 'col-controlled' },
  qos: { id: 'qos', label: 'QoS', className: 'col-qos' },
  pods: { id: 'pods', label: 'Pods', className: 'col-pods' },
  replicas: { id: 'replicas', label: 'Replicas', className: 'col-num' },
  updated: { id: 'updated', label: 'Updated', className: 'col-num' },
  available: { id: 'available', label: 'Available', className: 'col-num' },
  misscheduled: { id: 'misscheduled', label: 'Misscheduled', className: 'col-num' },
  completions: { id: 'completions', label: 'Completions', className: 'col-num' },
  conditions: { id: 'conditions', label: 'Conditions', className: 'col-conditions' },
  schedule: { id: 'schedule', label: 'Schedule', className: 'col-schedule' },
  suspend: { id: 'suspend', label: 'Suspend', className: 'col-suspend' },
  active: { id: 'active', label: 'Active', className: 'col-num' },
  lastSchedule: { id: 'lastSchedule', label: 'Last schedule', className: 'col-age' },
  type: { id: 'type', label: 'Type', className: 'col-type' },
  clusterIP: { id: 'clusterIP', label: 'Cluster IP', className: 'col-cluster-ip' },
  ports: { id: 'ports', label: 'Ports', className: 'col-ports' },
  externalIP: { id: 'externalIP', label: 'External IP', className: 'col-external-ip' },
  selector: { id: 'selector', label: 'Selector', className: 'col-selector' },
  endpoints: { id: 'endpoints', label: 'Endpoints', className: 'col-endpoints' },
  addressType: { id: 'addressType', label: 'Address type', className: 'col-address-type' },
  service: { id: 'service', label: 'Service', className: 'col-service' },
  loadBalancers: { id: 'loadBalancers', label: 'LoadBalancers', className: 'col-load-balancers' },
  rules: { id: 'rules', label: 'Rules', className: 'col-rules' },
  controller: { id: 'controller', label: 'Controller', className: 'col-controller' },
  apiGroup: { id: 'apiGroup', label: 'API Group', className: 'col-api-group' },
  scope: { id: 'scope', label: 'Scope', className: 'col-scope' },
  parameterKind: { id: 'parameterKind', label: 'Kind', className: 'col-parameter-kind' },
  policyTypes: { id: 'policyTypes', label: 'Policy Types', className: 'col-policy-types' },
  provisioner: { id: 'provisioner', label: 'Provisioner', className: 'col-provisioner' },
  reclaimPolicy: { id: 'reclaimPolicy', label: 'Reclaim Policy', className: 'col-reclaim-policy' },
  volumeBindingMode: { id: 'volumeBindingMode', label: 'Volume Binding Mode', className: 'col-volume-binding-mode' },
  defaultClass: { id: 'defaultClass', label: 'Default', className: 'col-default-class' },
}

/** Lens-style Endpoints / EndpointSlice tables. */
const ENDPOINT_RESOURCE_COLUMNS = {
  columnOrder: ['name', 'namespace', 'endpoints', 'age'],
  defaultVisible: ['name', 'namespace', 'endpoints', 'age'],
  columns: {
    name: COL.name,
    namespace: COL.namespace,
    endpoints: COL.endpoints,
    age: COL.age,
  },
}

/** Lens-style pods + desired replicas (StatefulSet, DaemonSet-style lists). */
const WORKLOAD_PODS_COLUMNS = {
  columnOrder: ['name', 'namespace', 'pods', 'replicas', 'age'],
  defaultVisible: ['name', 'namespace', 'pods', 'replicas', 'age'],
  columns: {
    name: COL.name,
    namespace: COL.namespace,
    pods: COL.pods,
    replicas: COL.replicas,
    age: COL.age,
  },
}

/** Lens-style deployment columns. */
const DEPLOYMENT_COLUMNS = {
  columnOrder: ['name', 'namespace', 'pods', 'replicas', 'age', 'conditions'],
  defaultVisible: ['name', 'namespace', 'pods', 'replicas', 'age', 'conditions'],
  columns: {
    ...WORKLOAD_PODS_COLUMNS.columns,
    conditions: COL.conditions,
  },
}

/** Lens-style daemonset scheduled pod counts. */
const DAEMONSET_COLUMNS = {
  columnOrder: [
    'name', 'namespace', 'desired', 'current', 'ready', 'updated', 'available', 'misscheduled', 'age',
  ],
  defaultVisible: [
    'name', 'namespace', 'desired', 'current', 'ready', 'updated', 'available', 'misscheduled', 'age',
  ],
  columns: {
    name: COL.name,
    namespace: COL.namespace,
    desired: COL.desired,
    current: COL.current,
    ready: COL.ready,
    updated: COL.updated,
    available: COL.available,
    misscheduled: COL.misscheduled,
    age: COL.age,
  },
}

/** Lens-style replica counts for controllers (ReplicaSet, Deployment, …). */
const REPLICA_COUNT_COLUMNS = {
  columnOrder: ['name', 'namespace', 'status', 'desired', 'current', 'ready', 'age'],
  defaultVisible: ['name', 'namespace', 'desired', 'current', 'ready', 'age'],
  columns: {
    name: COL.name,
    namespace: COL.namespace,
    status: COL.status,
    desired: COL.desired,
    current: COL.current,
    ready: COL.ready,
    age: COL.age,
  },
}

/**
 * Per-kind table schemas. Add an entry here to enable column picker + custom columns.
 * Preferences persist under localStorage key `klew.entityTable.columns.<Kind>`.
 */
export const KIND_TABLE_SCHEMAS = {
  Pod: {
    columnOrder: [
      'name', 'namespace', 'status', 'containers', 'restarts', 'controlledBy', 'node', 'qos', 'age',
    ],
    defaultVisible: ['name', 'namespace', 'status', 'restarts', 'age'],
    columns: {
      name: COL.name,
      namespace: COL.namespace,
      status: COL.status,
      containers: COL.containers,
      restarts: COL.restarts,
      controlledBy: COL.controlledBy,
      node: COL.node,
      qos: COL.qos,
      age: COL.age,
    },
  },
  ReplicaSet: { ...REPLICA_COUNT_COLUMNS },
  Deployment: { ...DEPLOYMENT_COLUMNS },
  StatefulSet: { ...WORKLOAD_PODS_COLUMNS },
  DaemonSet: { ...DAEMONSET_COLUMNS },
  Job: {
    columnOrder: ['name', 'namespace', 'completions', 'age', 'conditions'],
    defaultVisible: ['name', 'namespace', 'completions', 'age', 'conditions'],
    columns: {
      name: COL.name,
      namespace: COL.namespace,
      completions: COL.completions,
      age: COL.age,
      conditions: COL.conditions,
    },
  },
  CronJob: {
    columnOrder: ['name', 'namespace', 'schedule', 'suspend', 'active', 'lastSchedule', 'age'],
    defaultVisible: ['name', 'namespace', 'schedule', 'suspend', 'active', 'lastSchedule', 'age'],
    columns: {
      name: COL.name,
      namespace: COL.namespace,
      schedule: COL.schedule,
      suspend: COL.suspend,
      active: COL.active,
      lastSchedule: COL.lastSchedule,
      age: COL.age,
    },
  },
  HelmRelease: {
    columnOrder: [
      'name', 'namespace', 'chart', 'revision', 'chartVersion', 'appVersion', 'status', 'updated',
    ],
    defaultVisible: [
      'name', 'namespace', 'chart', 'revision', 'chartVersion', 'appVersion', 'status', 'updated',
    ],
    columns: {
      name: COL.name,
      namespace: COL.namespace,
      status: COL.status,
      chart: { id: 'chart', label: 'Chart', className: 'col-chart' },
      revision: { id: 'revision', label: 'Revision', className: 'col-num' },
      chartVersion: { id: 'chartVersion', label: 'Version', className: 'col-version' },
      appVersion: { id: 'appVersion', label: 'App Version', className: 'col-version' },
      updated: { id: 'updated', label: 'Updated', className: 'col-age' },
    },
  },
  Endpoints: { ...ENDPOINT_RESOURCE_COLUMNS },
  EndpointSlice: {
    columnOrder: ['name', 'namespace', 'endpoints', 'service', 'addressType', 'age'],
    defaultVisible: ['name', 'namespace', 'endpoints', 'age'],
    columns: {
      ...ENDPOINT_RESOURCE_COLUMNS.columns,
      service: COL.service,
      addressType: COL.addressType,
    },
  },
  Ingress: {
    columnOrder: ['name', 'namespace', 'loadBalancers', 'rules', 'age'],
    defaultVisible: ['name', 'namespace', 'loadBalancers', 'rules', 'age'],
    columns: {
      name: COL.name,
      namespace: COL.namespace,
      loadBalancers: COL.loadBalancers,
      rules: COL.rules,
      age: COL.age,
    },
  },
  IngressClass: {
    columnOrder: ['name', 'namespace', 'controller', 'apiGroup', 'scope', 'parameterKind'],
    defaultVisible: ['name', 'namespace', 'controller', 'apiGroup', 'scope', 'parameterKind'],
    columns: {
      name: COL.name,
      namespace: COL.namespace,
      controller: COL.controller,
      apiGroup: COL.apiGroup,
      scope: COL.scope,
      parameterKind: COL.parameterKind,
    },
  },
  NetworkPolicy: {
    columnOrder: ['name', 'namespace', 'policyTypes', 'age'],
    defaultVisible: ['name', 'namespace', 'policyTypes', 'age'],
    columns: {
      name: COL.name,
      namespace: COL.namespace,
      policyTypes: COL.policyTypes,
      age: COL.age,
    },
  },
  StorageClass: {
    columnOrder: ['name', 'provisioner', 'reclaimPolicy', 'volumeBindingMode', 'defaultClass', 'age'],
    defaultVisible: ['name', 'provisioner', 'reclaimPolicy', 'volumeBindingMode', 'defaultClass', 'age'],
    columns: {
      name: COL.name,
      provisioner: COL.provisioner,
      reclaimPolicy: COL.reclaimPolicy,
      volumeBindingMode: COL.volumeBindingMode,
      defaultClass: COL.defaultClass,
      age: COL.age,
    },
  },
  Service: {
    columnOrder: [
      'name', 'namespace', 'type', 'clusterIP', 'ports', 'externalIP', 'selector', 'age', 'status',
    ],
    defaultVisible: [
      'name', 'namespace', 'type', 'clusterIP', 'ports', 'externalIP', 'selector', 'age', 'status',
    ],
    columns: {
      name: COL.name,
      namespace: COL.namespace,
      type: COL.type,
      clusterIP: COL.clusterIP,
      ports: COL.ports,
      externalIP: COL.externalIP,
      selector: COL.selector,
      age: COL.age,
      status: COL.status,
    },
  },
}

const DEFAULT_SCHEMA = {
  columnOrder: ['name', 'namespace', 'status', 'ready', 'node', 'age'],
  defaultVisible: ['name', 'namespace', 'status', 'age'],
  columns: {
    name: COL.name,
    namespace: COL.namespace,
    status: COL.status,
    ready: COL.ready,
    node: COL.node,
    age: COL.age,
  },
}

export function schemaForKind(kind) {
  if (KIND_TABLE_SCHEMAS[kind]) return KIND_TABLE_SCHEMAS[kind]
  if (BUILTIN_KIND_TABLE_SCHEMAS[kind]) return BUILTIN_KIND_TABLE_SCHEMAS[kind]
  if (kind === 'Namespace') {
    return {
      columnOrder: ['name', 'status', 'age'],
      defaultVisible: ['name', 'status', 'age'],
      columns: {
        name: COL.name,
        status: COL.status,
        age: COL.age,
      },
    }
  }
  return DEFAULT_SCHEMA
}

export function availableColumnIds(kind, kindGroup, browseScope) {
  const schema = schemaForKind(kind)
  let ids = [...(schema.columnOrder || Object.keys(schema.columns))]

  const lensNamespaceColumn = kind === 'IngressClass'
  if (!lensNamespaceColumn && !showsNamespaceColumn(kindGroup, kind, browseScope)) {
    ids = ids.filter((id) => id !== 'namespace')
  }

  if (isClusterScopedKindGroup(kindGroup, kind)) {
    if (lensNamespaceColumn) {
      ids = ids.filter((id) => id !== 'node')
    } else {
      ids = ids.filter((id) => id !== 'namespace' && id !== 'node')
    }
  }

  return ids
}

export function columnDefsForIds(kind, ids) {
  const schema = schemaForKind(kind)
  return ids.map((id) => schema.columns[id]).filter(Boolean)
}

export function loadColumnPreferences(kind) {
  if (!kind) return null
  try {
    const raw = localStorage.getItem(`${ENTITY_TABLE_COLUMNS_KEY}.${kind}`)
    if (!raw) return null
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed.filter(Boolean) : null
  } catch {
    return null
  }
}

export function saveColumnPreferences(kind, columnIds) {
  if (!kind || !columnIds?.length) return
  try {
    localStorage.setItem(`${ENTITY_TABLE_COLUMNS_KEY}.${kind}`, JSON.stringify(columnIds))
  } catch {
    // ignore quota errors
  }
}

export function resolveVisibleColumnIds({
  kind,
  kindGroup,
  browseScope,
  selectedIds,
}) {
  const resolvedKind = kind || kindGroup?.kind || ''
  const available = availableColumnIds(resolvedKind, kindGroup, browseScope)
  const schema = schemaForKind(resolvedKind)
  const defaults = schema.defaultVisible.filter((id) => available.includes(id))
  const stored = selectedIds?.length
    ? selectedIds.filter((id) => available.includes(id))
    : loadColumnPreferences(resolvedKind)?.filter((id) => available.includes(id))

  let visible = stored?.length ? [...stored] : [...defaults]

  if (!visible.includes('name')) {
    visible.unshift('name')
  }

  visible = visible.filter((id) => available.includes(id))
  if (!visible.length) visible = [...defaults]
  return visible
}

/** Column menu order: visible columns in table order, then hidden optional columns. */
export function columnPickerOrder(selectable, visibleIds) {
  const visibleSet = new Set(visibleIds)
  const visible = visibleIds
    .map((id) => selectable.find((col) => col.id === id))
    .filter(Boolean)
  const hidden = selectable.filter((col) => !visibleSet.has(col.id))
  return [...visible, ...hidden]
}

export function allSelectableColumns(kind, kindGroup, browseScope) {
  const resolvedKind = kind || kindGroup?.kind || ''
  const ids = availableColumnIds(resolvedKind, kindGroup, browseScope)
  return columnDefsForIds(resolvedKind, ids)
}
