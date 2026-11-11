/**
 * Kind name normalisation for icons and labels: short names and plurals
 * (deploy, pods, netpol) map to the canonical Kubernetes kind. The icons
 * themselves live in components/icons/kindGlyphs.jsx.
 */
const KIND_ALIASES = {
  deploy: 'Deployment',
  deployment: 'Deployment',
  deployments: 'Deployment',
  svc: 'Service',
  service: 'Service',
  services: 'Service',
  ing: 'Ingress',
  ingress: 'Ingress',
  ingresses: 'Ingress',
  ingressclass: 'IngressClass',
  ingressclasses: 'IngressClass',
  pod: 'Pod',
  pods: 'Pod',
  po: 'Pod',
  rs: 'ReplicaSet',
  replicaset: 'ReplicaSet',
  replicasets: 'ReplicaSet',
  rc: 'ReplicationController',
  sts: 'StatefulSet',
  statefulset: 'StatefulSet',
  statefulsets: 'StatefulSet',
  ds: 'DaemonSet',
  daemonset: 'DaemonSet',
  daemonsets: 'DaemonSet',
  cm: 'ConfigMap',
  configmap: 'ConfigMap',
  configmaps: 'ConfigMap',
  secret: 'Secret',
  secrets: 'Secret',
  hpa: 'HorizontalPodAutoscaler',
  pdb: 'PodDisruptionBudget',
  pvc: 'PersistentVolumeClaim',
  pv: 'PersistentVolume',
  sc: 'StorageClass',
  sa: 'ServiceAccount',
  crd: 'CustomResourceDefinition',
  netpol: 'NetworkPolicy',
  ep: 'Endpoints',
  eps: 'EndpointSlice',
  endpointslice: 'EndpointSlice',
  endpointslices: 'EndpointSlice',
  cronjob: 'CronJob',
  cronjobs: 'CronJob',
  cj: 'CronJob',
  job: 'Job',
  jobs: 'Job',
  ns: 'Namespace',
  no: 'Node',
  node: 'Node',
  nodes: 'Node',
  namespace: 'Namespace',
  namespaces: 'Namespace',
  ev: 'Event',
  event: 'Event',
  events: 'Event',
  quota: 'ResourceQuota',
  limits: 'LimitRange',
  pc: 'PriorityClass',
}

export function normalizeKind(kind) {
  if (!kind) return 'Unknown'
  const k = String(kind).trim()
  if (KIND_ALIASES[k.toLowerCase()]) return KIND_ALIASES[k.toLowerCase()]
  if (KIND_ALIASES[k]) return KIND_ALIASES[k]
  return k
}
