import { describe, expect, it } from 'vitest'
import {
  showsNamespaceColumn,
  enrichEntityForTable,
  formatDeploymentConditions,
  parseSortDuration,
  parseSortFraction,
  parseSortNumber,
  parseWorkloadReplicaSignal,
  resolveDeploymentConditions,
  serviceStatusLabel,
  sortEntitiesForTable,
  tableCellValue,
  truncateSelector,
} from './entityTable.js'
import {
  availableColumnIds,
  columnPickerOrder,
  resolveVisibleColumnIds,
  schemaForKind,
} from './entityTableColumns.js'
import { singleBrowseScope, allBrowseScope } from './browseScope.js'

describe('Pod table columns', () => {
  it('exposes lens-style pod columns in the picker schema', () => {
    const ids = availableColumnIds('Pod', { namespaced: true }, allBrowseScope())
    expect(ids).toEqual([
      'name',
      'namespace',
      'status',
      'cpu',
      'memory',
      'containers',
      'restarts',
      'controlledBy',
      'node',
      'nodeSelector',
      'tolerations',
      'affinity',
      'qos',
      'age',
    ])
  })

  it('defaults to name, namespace, status, restarts, scheduling columns, age', () => {
    const visible = resolveVisibleColumnIds({
      kind: 'Pod',
      kindGroup: { namespaced: true },
      browseScope: allBrowseScope(),
    })
    expect(visible).toEqual([
      'name', 'namespace', 'status', 'cpu', 'memory', 'restarts', 'nodeSelector', 'tolerations', 'affinity', 'age',
    ])
  })

  it('hides namespace in single-namespace scope', () => {
    const visible = resolveVisibleColumnIds({
      kind: 'Pod',
      kindGroup: { namespaced: true },
      browseScope: singleBrowseScope('klew-lab'),
    })
    expect(visible).not.toContain('namespace')
    expect(visible).toContain('restarts')
  })
})

describe('showsNamespaceColumn', () => {
  it('returns false for cluster-scoped kinds', () => {
    expect(showsNamespaceColumn({ namespaced: false }, 'ValidatingPolicy', allBrowseScope())).toBe(false)
  })
})

describe('enrichEntityForTable', () => {
  it('fills pod-specific fields from catalog entity metadata', () => {
    const row = enrichEntityForTable({
      kind: 'Pod',
      name: 'redis-abc',
      namespace: 'klew-lab',
      containerNames: ['redis'],
      restartCount: 2,
      ownerKind: 'ReplicaSet',
      ownerName: 'redis-58f46b4685',
      qosClass: 'Burstable',
      nodeName: 'docker-desktop',
      creationTimestamp: new Date(Date.now() - 3600000).toISOString(),
      signal: 'Running',
      status: 'healthy',
    })
    expect(row.table.containers).toEqual([{
      name: 'redis',
      state: 'unknown',
      ready: false,
      reason: '',
      message: '',
      init: false,
    }])
    expect(row.table.containersSummary).toBe('redis')
    expect(row.table.restarts).toBe(2)
    expect(row.table.controlledBy).toBe('ReplicaSet')
    expect(row.table.controlledByKey).toBe('ReplicaSet/klew-lab/redis-58f46b4685')
    expect(row.table.controlledByTitle).toBe('ReplicaSet/redis-58f46b4685')
    expect(row.table.qos).toBe('Burstable')
    expect(tableCellValue(row, 'node')).toBe('docker-desktop')
  })

  it('parses deployment ready from signal hint', () => {
    const row = enrichEntityForTable({
      kind: 'Deployment',
      name: 'api',
      signal: '2/2 ready',
      status: 'healthy',
    })
    expect(tableCellValue(row, 'ready')).toBe('2/2')
  })
})

describe('ReplicaSet table columns', () => {
  it('exposes status and replica columns in the picker schema', () => {
    const ids = availableColumnIds('ReplicaSet', { namespaced: true }, allBrowseScope())
    expect(ids).toEqual([
      'name', 'namespace', 'status', 'desired', 'current', 'ready', 'nodeSelector', 'tolerations', 'affinity', 'age',
    ])
  })

  it('defaults to name, namespace, desired, current, ready, age', () => {
    const visible = resolveVisibleColumnIds({
      kind: 'ReplicaSet',
      kindGroup: { namespaced: true },
      browseScope: allBrowseScope(),
    })
    expect(visible).toEqual([
      'name', 'namespace', 'desired', 'current', 'ready', 'nodeSelector', 'tolerations', 'affinity', 'age',
    ])
  })

  it('fills replica counts from catalog entity metadata', () => {
    const row = enrichEntityForTable({
      kind: 'ReplicaSet',
      name: 'payment-api-75495887df',
      namespace: 'klew-lab',
      desiredReplicas: 2,
      currentReplicas: 2,
      readyReplicas: 2,
      creationTimestamp: new Date(Date.now() - 3600000).toISOString(),
    })
    expect(tableCellValue(row, 'desired')).toBe('2')
    expect(tableCellValue(row, 'current')).toBe('2')
    expect(tableCellValue(row, 'ready')).toBe('2')
  })

  it('shows ready 0 when current is 0 and readyReplicas is missing', () => {
    const row = enrichEntityForTable({
      kind: 'ReplicaSet',
      name: 'scaled-down',
      currentReplicas: 0,
      signal: '0/0 ready',
    })
    expect(tableCellValue(row, 'current')).toBe('0')
    expect(tableCellValue(row, 'ready')).toBe('0')
  })

  it('parses ready/current from status hint when counts are absent', () => {
    expect(parseWorkloadReplicaSignal('0/2 ready')).toEqual({ ready: 0, current: 2 })
    const row = enrichEntityForTable({
      kind: 'ReplicaSet',
      name: 'api',
      signal: '0/2 ready',
    })
    expect(tableCellValue(row, 'ready')).toBe('0')
    expect(tableCellValue(row, 'current')).toBe('2')
  })

  it('preserves custom column order from preferences', () => {
    const visible = resolveVisibleColumnIds({
      kind: 'ReplicaSet',
      kindGroup: { namespaced: true },
      browseScope: allBrowseScope(),
      selectedIds: ['name', 'ready', 'desired', 'current', 'age'],
    })
    expect(visible).toEqual(['name', 'ready', 'desired', 'current', 'age'])
  })

  it('orders picker with visible columns first', () => {
    const selectable = [
      { id: 'name', label: 'Name' },
      { id: 'namespace', label: 'Namespace' },
      { id: 'status', label: 'Status' },
      { id: 'ready', label: 'Ready' },
    ]
    expect(columnPickerOrder(selectable, ['name', 'ready']).map((c) => c.id))
      .toEqual(['name', 'ready', 'namespace', 'status'])
  })
})

describe('Deployment table columns', () => {
  it('exposes lens-style deployment columns in the picker schema', () => {
    const ids = availableColumnIds('Deployment', { namespaced: true }, allBrowseScope())
    expect(ids).toEqual([
      'name', 'namespace', 'pods', 'replicas', 'nodeSelector', 'tolerations', 'affinity', 'age', 'conditions',
    ])
  })

  it('defaults to name, namespace, pods, replicas, age, conditions', () => {
    const visible = resolveVisibleColumnIds({
      kind: 'Deployment',
      kindGroup: { namespaced: true },
      browseScope: allBrowseScope(),
    })
    expect(visible).toEqual([
      'name', 'namespace', 'pods', 'replicas', 'nodeSelector', 'tolerations', 'affinity', 'age', 'conditions',
    ])
  })

  it('fills deployment pods, replicas, and conditions', () => {
    const row = enrichEntityForTable({
      kind: 'Deployment',
      name: 'payment-api',
      namespace: 'klew-lab',
      desiredReplicas: 2,
      currentReplicas: 2,
      readyReplicas: 2,
      conditions: [
        { type: 'Available', status: 'True', reason: 'MinimumReplicasAvailable' },
        { type: 'Progressing', status: 'True', reason: 'NewReplicaSetAvailable' },
      ],
    })
    expect(tableCellValue(row, 'pods')).toBe('2/2')
    expect(tableCellValue(row, 'replicas')).toBe('2')
    expect(tableCellValue(row, 'conditions')).toBe('Available, Progressing')
    expect(row.table.conditionsDetail).toHaveLength(2)
    expect(formatDeploymentConditions([])).toBe('—')
  })

  it('infers Available and Progressing from replica counts when conditions absent', () => {
    const healthy = resolveDeploymentConditions({
      kind: 'Deployment',
      desiredReplicas: 2,
      currentReplicas: 2,
      readyReplicas: 2,
      availableReplicas: 2,
      updatedReplicas: 2,
      unavailableReplicas: 0,
    })
    expect(healthy.map((c) => `${c.type}:${c.status}`)).toEqual(['Available:True', 'Progressing:True'])

    const stuck = resolveDeploymentConditions({
      kind: 'Deployment',
      desiredReplicas: 3,
      currentReplicas: 4,
      readyReplicas: 3,
      availableReplicas: 3,
      updatedReplicas: 1,
      unavailableReplicas: 1,
    })
    expect(stuck.map((c) => `${c.type}:${c.status}`)).toEqual(['Available:True', 'Progressing:False'])
  })
})

describe('StatefulSet table columns', () => {
  it('exposes lens-style statefulset columns in the picker schema', () => {
    const ids = availableColumnIds('StatefulSet', { namespaced: true }, allBrowseScope())
    expect(ids).toEqual([
      'name', 'namespace', 'pods', 'replicas', 'nodeSelector', 'tolerations', 'affinity', 'age',
    ])
  })

  it('defaults to name, namespace, pods, replicas, age', () => {
    const visible = resolveVisibleColumnIds({
      kind: 'StatefulSet',
      kindGroup: { namespaced: true },
      browseScope: allBrowseScope(),
    })
    expect(visible).toEqual([
      'name', 'namespace', 'pods', 'replicas', 'nodeSelector', 'tolerations', 'affinity', 'age',
    ])
  })

  it('fills statefulset pods and replicas from catalog metadata', () => {
    const row = enrichEntityForTable({
      kind: 'StatefulSet',
      name: 'redis',
      namespace: 'klew-lab',
      desiredReplicas: 1,
      currentReplicas: 1,
      readyReplicas: 1,
    })
    expect(tableCellValue(row, 'pods')).toBe('1/1')
    expect(tableCellValue(row, 'replicas')).toBe('1')
  })
})

describe('DaemonSet table columns', () => {
  it('exposes lens-style daemonset columns in the picker schema', () => {
    const ids = availableColumnIds('DaemonSet', { namespaced: true }, allBrowseScope())
    expect(ids).toEqual([
      'name', 'namespace', 'desired', 'current', 'ready', 'updated', 'available', 'misscheduled',
      'nodeSelector', 'tolerations', 'affinity', 'age',
    ])
  })

  it('fills daemonset counts including misscheduled 0', () => {
    const row = enrichEntityForTable({
      kind: 'DaemonSet',
      name: 'kube-proxy',
      namespace: 'kube-system',
      desiredReplicas: 2,
      currentReplicas: 2,
      readyReplicas: 2,
      updatedReplicas: 2,
      availableReplicas: 2,
      misscheduled: 0,
    })
    expect(tableCellValue(row, 'desired')).toBe('2')
    expect(tableCellValue(row, 'current')).toBe('2')
    expect(tableCellValue(row, 'ready')).toBe('2')
    expect(tableCellValue(row, 'updated')).toBe('2')
    expect(tableCellValue(row, 'available')).toBe('2')
    expect(tableCellValue(row, 'misscheduled')).toBe('0')
  })

  it('defaults misscheduled to 0 when other daemonset counts exist', () => {
    const row = enrichEntityForTable({
      kind: 'DaemonSet',
      name: 'kube-proxy',
      desiredReplicas: 2,
      currentReplicas: 2,
      readyReplicas: 2,
    })
    expect(tableCellValue(row, 'misscheduled')).toBe('0')
  })

  it('falls back to status hint when catalog counts are missing', () => {
    const row = enrichEntityForTable({
      kind: 'DaemonSet',
      name: 'kube-proxy',
      signal: '2/2 ready',
    })
    expect(tableCellValue(row, 'desired')).toBe('2')
    expect(tableCellValue(row, 'current')).toBe('2')
    expect(tableCellValue(row, 'ready')).toBe('2')
    expect(tableCellValue(row, 'updated')).toBe('2')
    expect(tableCellValue(row, 'available')).toBe('2')
    expect(tableCellValue(row, 'misscheduled')).toBe('0')
  })
})

describe('Job table columns', () => {
  it('exposes lens-style job columns in the picker schema', () => {
    const ids = availableColumnIds('Job', { namespaced: true }, allBrowseScope())
    expect(ids).toEqual([
      'name', 'completions', 'age', 'conditions', 'namespace', 'duration', 'controlledBy',
      'nodeSelector', 'tolerations', 'affinity',
    ])
  })

  it('defaults to name, completion, age, conditions, namespace', () => {
    const visible = resolveVisibleColumnIds({
      kind: 'Job',
      kindGroup: { namespaced: true },
      browseScope: allBrowseScope(),
    })
    expect(visible).toEqual([
      'name', 'completions', 'age', 'conditions', 'namespace', 'nodeSelector', 'tolerations', 'affinity',
    ])
  })

  it('keeps namespace visible in single-namespace browse', () => {
    const ids = availableColumnIds('Job', { namespaced: true }, { mode: 'single', namespace: 'dtool' })
    expect(ids).toContain('namespace')
  })

  it('fills job completions and conditions', () => {
    const row = enrichEntityForTable({
      kind: 'Job',
      name: 'hello',
      namespace: 'default',
      succeeded: 1,
      completions: 1,
      conditions: [{ type: 'Complete', status: 'True', reason: 'JobCompleted' }],
    })
    expect(tableCellValue(row, 'completions')).toBe('1/1')
    expect(tableCellValue(row, 'conditions')).toBe('Complete')
    expect(row.table.conditionsDetail).toHaveLength(1)
  })

  it('falls back to status hint when catalog counts are missing', () => {
    const row = enrichEntityForTable({
      kind: 'Job',
      name: 'hello',
      signal: '1/1',
    })
    expect(tableCellValue(row, 'completions')).toBe('1/1')
    expect(row.table.conditionsDetail[0]).toMatchObject({ type: 'Complete', status: 'True' })
  })

  it('shows job duration from catalog field', () => {
    const row = enrichEntityForTable({
      kind: 'Job',
      name: 'hello',
      jobDuration: '3m',
    })
    expect(tableCellValue(row, 'duration')).toBe('3m')
  })
})

describe('CronJob table columns', () => {
  it('exposes lens-style cronjob columns in the picker schema', () => {
    const ids = availableColumnIds('CronJob', { namespaced: true }, allBrowseScope())
    expect(ids).toEqual([
      'name', 'namespace', 'age', 'schedule', 'suspend', 'active', 'lastSchedule',
      'nodeSelector', 'tolerations', 'affinity',
    ])
  })

  it('defaults to name, namespace, age, schedule, suspend, active, last schedule', () => {
    const visible = resolveVisibleColumnIds({
      kind: 'CronJob',
      kindGroup: { namespaced: true },
      browseScope: allBrowseScope(),
    })
    expect(visible).toEqual([
      'name', 'namespace', 'age', 'schedule', 'suspend', 'active', 'lastSchedule',
      'nodeSelector', 'tolerations', 'affinity',
    ])
  })

  it('fills cronjob schedule, suspend, active, and last schedule', () => {
    const row = enrichEntityForTable({
      kind: 'CronJob',
      name: 'nightly',
      namespace: 'default',
      schedule: '0 2 * * *',
      suspend: true,
      activeJobs: 1,
      lastScheduleTime: new Date(Date.now() - 3600_000).toISOString(),
    })
    expect(tableCellValue(row, 'schedule')).toBe('0 2 * * *')
    expect(tableCellValue(row, 'suspend')).toBe('True')
    expect(tableCellValue(row, 'active')).toBe('1')
    expect(tableCellValue(row, 'lastSchedule')).toMatch(/^\d+[smhd]$/)
  })

  it('falls back to status hint for schedule when catalog field is missing', () => {
    const row = enrichEntityForTable({
      kind: 'CronJob',
      name: 'nightly',
      signal: '*/5 * * * *',
    })
    expect(tableCellValue(row, 'schedule')).toBe('*/5 * * * *')
    expect(tableCellValue(row, 'suspend')).toBe('False')
    expect(tableCellValue(row, 'active')).toBe('0')
  })
})

describe('truncateSelector', () => {
  it('clips long selectors for compact table cells', () => {
    expect(truncateSelector('app=payment-api,version=v2')).toBe('app=payment…')
  })

  it('leaves short selectors unchanged', () => {
    expect(truncateSelector('app=web')).toBe('app=web')
  })
})

describe('serviceStatusLabel', () => {
  it('uses endpoint readiness, not service type', () => {
    expect(serviceStatusLabel({
      serviceType: 'ClusterIP',
      readyEndpoints: 2,
      totalEndpoints: 3,
    })).toBe('2/3 ready')
    expect(serviceStatusLabel({
      serviceType: 'ClusterIP',
      readyEndpoints: 0,
      totalEndpoints: 0,
    })).toBe('No endpoints')
    expect(serviceStatusLabel({ serviceType: 'ExternalName' })).toBe('External')
    expect(serviceStatusLabel({ serviceType: 'ClusterIP', signal: 'ClusterIP' })).toBe('—')
  })
})

describe('Endpoints table columns', () => {
  it('exposes name, namespace, endpoints, age for Endpoints and EndpointSlice', () => {
    for (const kind of ['Endpoints', 'EndpointSlice']) {
      const ids = availableColumnIds(kind, { namespaced: true }, allBrowseScope())
      expect(ids).toEqual(expect.arrayContaining(['name', 'namespace', 'endpoints', 'age']))
      const visible = resolveVisibleColumnIds({ kind, kindGroup: { namespaced: true }, browseScope: allBrowseScope() })
      expect(visible).toEqual(['name', 'namespace', 'endpoints', 'age'])
    }
  })

  it('formats kubectl-style endpoint summary', () => {
    const row = enrichEntityForTable({
      kind: 'Endpoints',
      name: 'kubernetes',
      namespace: 'default',
      endpointSummary: '172.20.0.2:6443',
    })
    expect(tableCellValue(row, 'endpoints')).toBe('172.20.0.2:6443')
  })
})

describe('Ingress table columns', () => {
  it('exposes lens-style ingress columns', () => {
    const ids = availableColumnIds('Ingress', { namespaced: true }, allBrowseScope())
    expect(ids).toEqual(['name', 'namespace', 'loadBalancers', 'rules', 'age'])
    const visible = resolveVisibleColumnIds({
      kind: 'Ingress',
      kindGroup: { namespaced: true },
      browseScope: allBrowseScope(),
    })
    expect(visible).toEqual(['name', 'namespace', 'loadBalancers', 'rules', 'age'])
  })

  it('formats load balancers and rules summary', () => {
    const row = enrichEntityForTable({
      kind: 'Ingress',
      name: 'my-ing',
      namespace: 'default',
      loadBalancers: ['203.0.113.10'],
      ingressRulesSummary: 'app.example.com/api +1',
    })
    expect(tableCellValue(row, 'loadBalancers')).toBe('203.0.113.10')
    expect(tableCellValue(row, 'rules')).toBe('app.example.com/api +1')
  })
})

describe('IngressClass table columns', () => {
  it('exposes lens-style ingress class columns', () => {
    const kindGroup = { namespaced: false, kind: 'IngressClass' }
    const ids = availableColumnIds('IngressClass', kindGroup, allBrowseScope())
    expect(ids).toEqual(['name', 'namespace', 'controller', 'apiGroup', 'scope', 'parameterKind'])
    const visible = resolveVisibleColumnIds({
      kind: 'IngressClass',
      kindGroup,
      browseScope: allBrowseScope(),
    })
    expect(visible).toEqual(['name', 'namespace', 'controller', 'apiGroup', 'scope', 'parameterKind'])
  })

  it('formats controller and parameter reference fields', () => {
    const row = enrichEntityForTable({
      kind: 'IngressClass',
      name: 'nginx',
      ingressController: 'k8s.io/ingress-nginx',
      parameterAPIGroup: 'k8s.example.com',
      parameterScope: 'Cluster',
      parameterKind: 'ClusterIngressParameter',
      parameterNamespace: 'kube-system',
    })
    expect(tableCellValue(row, 'controller')).toBe('k8s.io/ingress-nginx')
    expect(tableCellValue(row, 'apiGroup')).toBe('k8s.example.com')
    expect(tableCellValue(row, 'scope')).toBe('Cluster')
    expect(tableCellValue(row, 'parameterKind')).toBe('ClusterIngressParameter')
    expect(tableCellValue(row, 'namespace')).toBe('kube-system')
  })
})

describe('NetworkPolicy table columns', () => {
  it('exposes lens-style network policy columns', () => {
    const ids = availableColumnIds('NetworkPolicy', { namespaced: true }, allBrowseScope())
    expect(ids).toEqual(['name', 'namespace', 'policyTypes', 'age'])
    const visible = resolveVisibleColumnIds({
      kind: 'NetworkPolicy',
      kindGroup: { namespaced: true },
      browseScope: allBrowseScope(),
    })
    expect(visible).toEqual(['name', 'namespace', 'policyTypes', 'age'])
  })

  it('formats policy types', () => {
    const row = enrichEntityForTable({
      kind: 'NetworkPolicy',
      name: 'deny-all',
      namespace: 'default',
      policyTypes: ['Ingress', 'Egress'],
    })
    expect(tableCellValue(row, 'policyTypes')).toBe('Ingress, Egress')
  })
})

describe('StorageClass table columns', () => {
  it('exposes lens-style storage class columns', () => {
    const kindGroup = { namespaced: false, kind: 'StorageClass' }
    const ids = availableColumnIds('StorageClass', kindGroup, allBrowseScope())
    expect(ids).toEqual(['name', 'provisioner', 'reclaimPolicy', 'volumeBindingMode', 'defaultClass', 'age'])
    const visible = resolveVisibleColumnIds({
      kind: 'StorageClass',
      kindGroup,
      browseScope: allBrowseScope(),
    })
    expect(visible).toEqual(['name', 'provisioner', 'reclaimPolicy', 'volumeBindingMode', 'defaultClass', 'age'])
  })

  it('formats provisioner, reclaim policy, volume binding mode, and default', () => {
    const row = enrichEntityForTable({
      kind: 'StorageClass',
      name: 'standard',
      provisioner: 'kubernetes.io/gce-pd',
      reclaimPolicy: 'Delete',
      volumeBindingMode: 'WaitForFirstConsumer',
      isDefault: true,
    }, [], 'StorageClass')
    expect(tableCellValue(row, 'provisioner')).toBe('kubernetes.io/gce-pd')
    expect(tableCellValue(row, 'reclaimPolicy')).toBe('Delete')
    expect(tableCellValue(row, 'volumeBindingMode')).toBe('WaitForFirstConsumer')
    expect(tableCellValue(row, 'defaultClass')).toBe('true')
  })

  it('shows false when explicitly not default', () => {
    const row = enrichEntityForTable({
      kind: 'StorageClass',
      name: 'hostpath',
      isDefault: false,
    }, [], 'StorageClass')
    expect(tableCellValue(row, 'defaultClass')).toBe('false')
  })

  it('shows dash when default annotation is absent', () => {
    const row = enrichEntityForTable({
      kind: 'StorageClass',
      name: 'custom',
    }, [], 'StorageClass')
    expect(tableCellValue(row, 'defaultClass')).toBe('—')
  })

  it('formats storage class spec when catalog kind is missing', () => {
    const row = enrichEntityForTable({
      kind: 'Resource',
      resourceId: 'storage.k8s.io/v1/storageclasses',
      name: 'standard',
      provisioner: 'kubernetes.io/gce-pd',
      reclaimPolicy: 'Delete',
      volumeBindingMode: 'Immediate',
    }, [], 'StorageClass')
    expect(tableCellValue(row, 'provisioner')).toBe('kubernetes.io/gce-pd')
    expect(tableCellValue(row, 'reclaimPolicy')).toBe('Delete')
    expect(tableCellValue(row, 'volumeBindingMode')).toBe('Immediate')
  })
})

describe('Service table columns', () => {
  it('exposes lens-style service columns in the picker schema', () => {
    const ids = availableColumnIds('Service', { namespaced: true }, allBrowseScope())
    expect(ids).toEqual([
      'name',
      'namespace',
      'type',
      'clusterIP',
      'ports',
      'externalIP',
      'selector',
      'age',
      'status',
    ])
  })

  it('defaults to all service columns visible', () => {
    const visible = resolveVisibleColumnIds({
      kind: 'Service',
      kindGroup: { namespaced: true },
      browseScope: allBrowseScope(),
    })
    expect(visible).toEqual([
      'name',
      'namespace',
      'type',
      'clusterIP',
      'ports',
      'externalIP',
      'selector',
      'age',
      'status',
    ])
  })

  it('fills service-specific fields from catalog metadata', () => {
    const row = enrichEntityForTable({
      kind: 'Service',
      name: 'payment-api',
      namespace: 'prod',
      serviceType: 'ClusterIP',
      clusterIP: '10.96.0.12',
      ports: ['80/TCP', '443/TCP'],
      externalIPs: ['203.0.113.10'],
      selector: 'app=payment-api',
      readyEndpoints: 2,
      totalEndpoints: 2,
      creationTimestamp: new Date(Date.now() - 7200000).toISOString(),
    })
    expect(tableCellValue(row, 'type')).toBe('ClusterIP')
    expect(tableCellValue(row, 'clusterIP')).toBe('10.96.0.12')
    expect(tableCellValue(row, 'ports')).toBe('80/TCP, 443/TCP')
    expect(tableCellValue(row, 'externalIP')).toBe('203.0.113.10')
    expect(tableCellValue(row, 'selector')).toBe('app=payment-api')
    expect(tableCellValue(row, 'status')).toBe('2/2 ready')
  })
})

describe('schemaForKind', () => {
  it('uses universal default for unknown kinds', () => {
    const schema = schemaForKind('ConfigMap')
    expect(schema.defaultVisible).toContain('name')
    expect(schema.defaultVisible).toContain('age')
  })
})

describe('entity table sorting', () => {
  it('parses numeric, fraction, and duration sort values', () => {
    expect(parseSortNumber('3')).toBe(3)
    expect(parseSortNumber('—')).toBeNull()
    expect(parseSortFraction('2/3')).toEqual({ primary: 2, secondary: 3 })
    expect(parseSortDuration('5m')).toBe(300)
  })

  it('sorts rows by name ascending and descending', () => {
    const rows = [
      enrichEntityForTable({ kind: 'CronJob', name: 'z-job' }),
      enrichEntityForTable({ kind: 'CronJob', name: 'a-job' }),
      enrichEntityForTable({ kind: 'CronJob', name: 'm-job' }),
    ]
    expect(sortEntitiesForTable(rows, 'name', 'asc').map((r) => r.name))
      .toEqual(['a-job', 'm-job', 'z-job'])
    expect(sortEntitiesForTable(rows, 'name', 'desc').map((r) => r.name))
      .toEqual(['z-job', 'm-job', 'a-job'])
  })

  it('sorts cronjob schedule and active columns', () => {
    const rows = [
      enrichEntityForTable({
        kind: 'CronJob',
        name: 'every-minute',
        schedule: '*/1 * * * *',
        activeJobs: 2,
      }),
      enrichEntityForTable({
        kind: 'CronJob',
        name: 'nightly',
        schedule: '0 2 * * *',
        activeJobs: 0,
      }),
    ]
    expect(sortEntitiesForTable(rows, 'active', 'asc').map((r) => r.name))
      .toEqual(['nightly', 'every-minute'])
    expect(sortEntitiesForTable(rows, 'schedule', 'asc').map((r) => r.name))
      .toEqual(['every-minute', 'nightly'])
  })

  it('sorts age by creation timestamp rather than formatted label', () => {
    const now = Date.now()
    const rows = [
      enrichEntityForTable({
        kind: 'Pod',
        name: 'old',
        creationTimestamp: new Date(now - 7200_000).toISOString(),
      }),
      enrichEntityForTable({
        kind: 'Pod',
        name: 'new',
        creationTimestamp: new Date(now - 60_000).toISOString(),
      }),
    ]
    expect(sortEntitiesForTable(rows, 'age', 'asc').map((r) => r.name))
      .toEqual(['new', 'old'])
    expect(sortEntitiesForTable(rows, 'age', 'desc').map((r) => r.name))
      .toEqual(['old', 'new'])
  })
})
