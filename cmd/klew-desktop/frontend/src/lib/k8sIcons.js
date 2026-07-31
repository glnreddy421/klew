/**
 * Official Kubernetes community icon set (unlabeled).
 * @see https://github.com/kubernetes/community/tree/master/icons
 */
import deploy from 'kubernetes-icons/svg/resources/unlabeled/deploy.svg?url'
import sts from 'kubernetes-icons/svg/resources/unlabeled/sts.svg?url'
import ds from 'kubernetes-icons/svg/resources/unlabeled/ds.svg?url'
import rs from 'kubernetes-icons/svg/resources/unlabeled/rs.svg?url'
import job from 'kubernetes-icons/svg/resources/unlabeled/job.svg?url'
import cronjob from 'kubernetes-icons/svg/resources/unlabeled/cronjob.svg?url'
import pod from 'kubernetes-icons/svg/resources/unlabeled/pod.svg?url'
import svc from 'kubernetes-icons/svg/resources/unlabeled/svc.svg?url'
import ing from 'kubernetes-icons/svg/resources/unlabeled/ing.svg?url'
import cm from 'kubernetes-icons/svg/resources/unlabeled/cm.svg?url'
import secret from 'kubernetes-icons/svg/resources/unlabeled/secret.svg?url'
import pvc from 'kubernetes-icons/svg/resources/unlabeled/pvc.svg?url'
import pv from 'kubernetes-icons/svg/resources/unlabeled/pv.svg?url'
import sc from 'kubernetes-icons/svg/resources/unlabeled/sc.svg?url'
import hpa from 'kubernetes-icons/svg/resources/unlabeled/hpa.svg?url'
import psp from 'kubernetes-icons/svg/resources/unlabeled/psp.svg?url'
import netpol from 'kubernetes-icons/svg/resources/unlabeled/netpol.svg?url'
import ep from 'kubernetes-icons/svg/resources/unlabeled/ep.svg?url'
import sa from 'kubernetes-icons/svg/resources/unlabeled/sa.svg?url'
import role from 'kubernetes-icons/svg/resources/unlabeled/role.svg?url'
import rb from 'kubernetes-icons/svg/resources/unlabeled/rb.svg?url'
import cRole from 'kubernetes-icons/svg/resources/unlabeled/c-role.svg?url'
import crb from 'kubernetes-icons/svg/resources/unlabeled/crb.svg?url'
import ns from 'kubernetes-icons/svg/resources/unlabeled/ns.svg?url'
import node from 'kubernetes-icons/svg/infrastructure_components/unlabeled/node.svg?url'
import quota from 'kubernetes-icons/svg/resources/unlabeled/quota.svg?url'
import limits from 'kubernetes-icons/svg/resources/unlabeled/limits.svg?url'
import crd from 'kubernetes-icons/svg/resources/unlabeled/crd.svg?url'
import group from 'kubernetes-icons/svg/resources/unlabeled/group.svg?url'
import vol from 'kubernetes-icons/svg/resources/unlabeled/vol.svg?url'
import user from 'kubernetes-icons/svg/resources/unlabeled/user.svg?url'
import api from 'kubernetes-icons/svg/control_plane_components/labeled/api.svg?url'

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
  pod: 'Pod',
  pods: 'Pod',
  rs: 'ReplicaSet',
  replicaset: 'ReplicaSet',
  replicasets: 'ReplicaSet',
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
  sa: 'ServiceAccount',
  crd: 'CustomResourceDefinition',
  netpol: 'NetworkPolicy',
  ep: 'Endpoints',
  eps: 'EndpointSlice',
  endpointslice: 'EndpointSlice',
  endpointslices: 'EndpointSlice',
  cronjob: 'CronJob',
  cronjobs: 'CronJob',
  job: 'Job',
  jobs: 'Job',
}

/** Maps normalized Kubernetes kind → official icon asset URL. */
const ICON_BY_KIND = {
  Deployment: deploy,
  StatefulSet: sts,
  DaemonSet: ds,
  ReplicaSet: rs,
  Job: job,
  CronJob: cronjob,
  Pod: pod,
  Service: svc,
  Ingress: ing,
  ConfigMap: cm,
  Secret: secret,
  PersistentVolumeClaim: pvc,
  PersistentVolume: pv,
  StorageClass: sc,
  HorizontalPodAutoscaler: hpa,
  PodDisruptionBudget: group,
  NetworkPolicy: netpol,
  Endpoints: ep,
  EndpointSlice: ep,
  ServiceAccount: sa,
  Role: role,
  RoleBinding: rb,
  ClusterRole: cRole,
  ClusterRoleBinding: crb,
  Namespace: ns,
  Node: node,
  ResourceQuota: quota,
  LimitRange: limits,
  CustomResourceDefinition: crd,
  PodSecurityPolicy: psp,
  Event: group,
  Lease: group,
  PriorityClass: group,
  ValidatingWebhookConfiguration: api,
  MutatingWebhookConfiguration: api,
  User: user,
  Volume: vol,
}

const STREAM_ICON = {
  LOG: pod,
  EVENT: group,
  KLEW: api,
  METRIC: hpa,
  OBJ: crd,
  SYS: group,
}

export function normalizeKind(kind) {
  if (!kind) return 'Unknown'
  const k = String(kind).trim()
  if (KIND_ALIASES[k.toLowerCase()]) return KIND_ALIASES[k.toLowerCase()]
  if (KIND_ALIASES[k]) return KIND_ALIASES[k]
  return k
}

export function k8sIconUrl(kind) {
  const normalized = normalizeKind(kind)
  return ICON_BY_KIND[normalized] || group
}

export function streamIconUrl(source) {
  const key = String(source || '').toUpperCase()
  return STREAM_ICON[key] || group
}
