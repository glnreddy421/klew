/**
 * Presentation-only canonical schema for built-in Kubernetes resources.
 * This is NOT discovery — it defines where known resources appear in the UI.
 */

/**
 * @typedef {Object} PresentationResource
 * @property {string} group
 * @property {string} resource
 * @property {string} kind
 * @property {string} displayName
 * @property {number} sortOrder
 * @property {boolean} [discoveredOnly] — hide row unless cluster exposes this API (e.g. VPA CRD)
 */

/** @type {Array<{ id: string, label: string, resources: PresentationResource[] }>} */
export const BUILTIN_PRESENTATION = [
  {
    id: 'workloads',
    label: 'Workloads',
    resources: [
      { group: '', resource: 'pods', kind: 'Pod', displayName: 'Pods', sortOrder: 1 },
      { group: 'apps', resource: 'deployments', kind: 'Deployment', displayName: 'Deployments', sortOrder: 2 },
      { group: 'apps', resource: 'replicasets', kind: 'ReplicaSet', displayName: 'ReplicaSets', sortOrder: 3 },
      { group: '', resource: 'replicationcontrollers', kind: 'ReplicationController', displayName: 'ReplicationControllers', sortOrder: 4 },
      { group: 'apps', resource: 'statefulsets', kind: 'StatefulSet', displayName: 'StatefulSets', sortOrder: 5 },
      { group: 'apps', resource: 'daemonsets', kind: 'DaemonSet', displayName: 'DaemonSets', sortOrder: 6 },
      { group: 'batch', resource: 'jobs', kind: 'Job', displayName: 'Jobs', sortOrder: 7 },
      { group: 'batch', resource: 'cronjobs', kind: 'CronJob', displayName: 'CronJobs', sortOrder: 8 },
    ],
  },
  {
    id: 'network',
    label: 'Network',
    resources: [
      { group: '', resource: 'services', kind: 'Service', displayName: 'Services', sortOrder: 1 },
      { group: '', resource: 'endpoints', kind: 'Endpoints', displayName: 'Endpoints', sortOrder: 2 },
      { group: 'discovery.k8s.io', resource: 'endpointslices', kind: 'EndpointSlice', displayName: 'EndpointSlices', sortOrder: 3 },
      { group: 'networking.k8s.io', resource: 'ingresses', kind: 'Ingress', displayName: 'Ingresses', sortOrder: 4 },
      { group: 'networking.k8s.io', resource: 'ingressclasses', kind: 'IngressClass', displayName: 'IngressClasses', sortOrder: 5 },
      { group: 'networking.k8s.io', resource: 'networkpolicies', kind: 'NetworkPolicy', displayName: 'NetworkPolicies', sortOrder: 6 },
    ],
  },
  {
    id: 'storage',
    label: 'Storage',
    resources: [
      { group: '', resource: 'persistentvolumeclaims', kind: 'PersistentVolumeClaim', displayName: 'PersistentVolumeClaims', sortOrder: 1 },
      { group: '', resource: 'persistentvolumes', kind: 'PersistentVolume', displayName: 'PersistentVolumes', sortOrder: 2 },
      { group: 'storage.k8s.io', resource: 'storageclasses', kind: 'StorageClass', displayName: 'StorageClasses', sortOrder: 3 },
      { group: 'storage.k8s.io', resource: 'volumeattachments', kind: 'VolumeAttachment', displayName: 'VolumeAttachments', sortOrder: 4 },
      { group: 'storage.k8s.io', resource: 'csidrivers', kind: 'CSIDriver', displayName: 'CSIDrivers', sortOrder: 5 },
      { group: 'storage.k8s.io', resource: 'csinodes', kind: 'CSINode', displayName: 'CSINodes', sortOrder: 6 },
      { group: 'storage.k8s.io', resource: 'csistoragecapacities', kind: 'CSIStorageCapacity', displayName: 'CSIStorageCapacities', sortOrder: 7 },
      { group: 'storage.k8s.io', resource: 'volumeattributesclasses', kind: 'VolumeAttributesClass', displayName: 'VolumeAttributesClasses', sortOrder: 8 },
    ],
  },
  {
    id: 'config',
    label: 'Config',
    resources: [
      { group: '', resource: 'configmaps', kind: 'ConfigMap', displayName: 'ConfigMaps', sortOrder: 1 },
      { group: '', resource: 'secrets', kind: 'Secret', displayName: 'Secrets', sortOrder: 2 },
      { group: '', resource: 'resourcequotas', kind: 'ResourceQuota', displayName: 'Resource Quotas', sortOrder: 3 },
      { group: '', resource: 'limitranges', kind: 'LimitRange', displayName: 'Limit Ranges', sortOrder: 4 },
      { group: 'autoscaling', resource: 'horizontalpodautoscalers', kind: 'HorizontalPodAutoscaler', displayName: 'HPA', sortOrder: 5 },
      { group: 'autoscaling.k8s.io', resource: 'verticalpodautoscalers', kind: 'VerticalPodAutoscaler', displayName: 'VPA', sortOrder: 6, discoveredOnly: true },
      { group: 'policy', resource: 'poddisruptionbudgets', kind: 'PodDisruptionBudget', displayName: 'Pod Disruption Budgets', sortOrder: 7 },
      { group: 'scheduling.k8s.io', resource: 'priorityclasses', kind: 'PriorityClass', displayName: 'Priority Classes', sortOrder: 8 },
      { group: 'node.k8s.io', resource: 'runtimeclasses', kind: 'RuntimeClass', displayName: 'Runtime Classes', sortOrder: 9 },
      { group: 'coordination.k8s.io', resource: 'leases', kind: 'Lease', displayName: 'Leases', sortOrder: 10 },
      { group: 'admissionregistration.k8s.io', resource: 'mutatingwebhookconfigurations', kind: 'MutatingWebhookConfiguration', displayName: 'Mutating Webhook Configs', sortOrder: 11 },
      { group: 'admissionregistration.k8s.io', resource: 'validatingwebhookconfigurations', kind: 'ValidatingWebhookConfiguration', displayName: 'Validating Webhook Configs', sortOrder: 12 },
    ],
  },
  {
    id: 'security',
    label: 'Security',
    resources: [
      { group: '', resource: 'serviceaccounts', kind: 'ServiceAccount', displayName: 'ServiceAccounts', sortOrder: 1 },
      { group: 'rbac.authorization.k8s.io', resource: 'roles', kind: 'Role', displayName: 'Roles', sortOrder: 2 },
      { group: 'rbac.authorization.k8s.io', resource: 'rolebindings', kind: 'RoleBinding', displayName: 'RoleBindings', sortOrder: 3 },
      { group: 'rbac.authorization.k8s.io', resource: 'clusterroles', kind: 'ClusterRole', displayName: 'ClusterRoles', sortOrder: 4 },
      { group: 'rbac.authorization.k8s.io', resource: 'clusterrolebindings', kind: 'ClusterRoleBinding', displayName: 'ClusterRoleBindings', sortOrder: 5 },
    ],
  },
  {
    id: 'cluster',
    label: 'Cluster',
    resources: [
      { group: '', resource: 'nodes', kind: 'Node', displayName: 'Nodes', sortOrder: 1 },
      { group: '', resource: 'namespaces', kind: 'Namespace', displayName: 'Namespaces', sortOrder: 2 },
      { group: '', resource: 'events', kind: 'Event', displayName: 'Events', sortOrder: 3 },
      { group: 'apiextensions.k8s.io', resource: 'customresourcedefinitions', kind: 'CustomResourceDefinition', displayName: 'CustomResourceDefinitions', sortOrder: 4 },
      { group: 'apiregistration.k8s.io', resource: 'apiservices', kind: 'APIService', displayName: 'APIServices', sortOrder: 5 },
      { group: 'admissionregistration.k8s.io', resource: 'validatingadmissionpolicies', kind: 'ValidatingAdmissionPolicy', displayName: 'ValidatingAdmissionPolicies', sortOrder: 6 },
      { group: 'admissionregistration.k8s.io', resource: 'validatingadmissionpolicybindings', kind: 'ValidatingAdmissionPolicyBinding', displayName: 'ValidatingAdmissionPolicyBindings', sortOrder: 7 },
    ],
  },
]

/** Presentation identity: group/resource (no version). */
export function presentationKey(group, resource) {
  if (!group) return `/${resource}`
  return `${group}/${resource}`
}

const BUILTIN_KEYS = new Set(
  BUILTIN_PRESENTATION.flatMap((cat) =>
    cat.resources.map((r) => presentationKey(r.group, r.resource)),
  ),
)

const BUILTIN_LOOKUP = new Map()
for (const cat of BUILTIN_PRESENTATION) {
  for (const entry of cat.resources) {
    BUILTIN_LOOKUP.set(presentationKey(entry.group, entry.resource), {
      categoryId: cat.id,
      entry,
    })
  }
}

export function isBuiltinPresentationKey(key) {
  return BUILTIN_KEYS.has(key)
}

export function builtinCategoryForKey(key) {
  return BUILTIN_LOOKUP.get(key)?.categoryId || null
}

const CLUSTER_SCOPED_RESOURCES = new Set([
  'nodes', 'namespaces', 'persistentvolumes', 'storageclasses',
  'ingressclasses',
  'clusterroles', 'clusterrolebindings', 'customresourcedefinitions',
  'apiservices', 'mutatingwebhookconfigurations', 'validatingwebhookconfigurations',
  'validatingadmissionpolicies', 'validatingadmissionpolicybindings',
  'priorityclasses', 'runtimeclasses', 'csidrivers', 'csinodes',
  'volumeattachments', 'csistoragecapacities', 'volumeattributesclasses',
])

export function defaultNamespaced(entry) {
  return !CLUSTER_SCOPED_RESOURCES.has(entry.resource)
}

export function isExtensionGroup(group) {
  if (!group) return false
  return !group.endsWith('.k8s.io')
}

export function isDiscoveredOnlyEntry(entry) {
  return Boolean(entry?.discoveredOnly)
}
