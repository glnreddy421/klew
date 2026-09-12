import { resolveInspectRef, linkableTableColumn } from './inspectEnrich'
import { buildInspectKey } from './matches'

const ROLE_ORDER = [
  'Owner',
  'ReplicaSet',
  'Service',
  'Ingress',
  'Target pod',
  'Pod',
  'Node',
  'ConfigMap',
  'Secret',
  'Volume',
  'Autoscaling',
  'Mounted by',
  'Used by',
  'Related',
]

const ROLE_TONES = {
  Owner: 'owner',
  ReplicaSet: 'workload',
  Service: 'network',
  Ingress: 'network',
  'Target pod': 'workload',
  Pod: 'workload',
  Node: 'infra',
  ConfigMap: 'config',
  Secret: 'config',
  Volume: 'storage',
  Autoscaling: 'scale',
  'Mounted by': 'workload',
  'Used by': 'workload',
  Related: 'neutral',
}

/** Orb radii + label footprint used for autoscale layout. */
export const REL_ORB = {
  normal: { centerR: 26, satelliteR: 21, labelW: 88, labelH: 28 },
  compact: { centerR: 22, satelliteR: 18, labelW: 76, labelH: 24 },
}

/**
 * Build a hub-and-spoke graph from snapshot relationship items and/or detail sections.
 */
export function buildRelationshipGraph({
  center,
  items = [],
  sections = [],
  inspectNamespace = '',
}) {
  if (!center?.kind || !center?.name) return emptyGraph()

  const centerKey = center.key || buildInspectKey(center.kind, center.name, center.namespace || inspectNamespace)
  const nodeMap = new Map()
  const edgeMap = new Map()

  nodeMap.set(centerKey, {
    id: centerKey,
    key: centerKey,
    kind: center.kind,
    name: center.name,
    namespace: center.namespace || inspectNamespace,
    role: 'Current',
    isCenter: true,
  })

  const addNode = (ref, role) => {
    if (!ref?.key || ref.key === centerKey) return
    const id = ref.key
    if (!nodeMap.has(id)) {
      nodeMap.set(id, {
        id,
        key: ref.key,
        kind: ref.kind || 'Unknown',
        name: ref.name || id,
        namespace: ref.namespace || inspectNamespace,
        role: role || 'Related',
        isCenter: false,
      })
    } else {
      const existing = nodeMap.get(id)
      if (role && existing.role === 'Related') existing.role = role
    }
    const edgeId = `${centerKey}|${id}|${role || 'Related'}`
    if (!edgeMap.has(edgeId)) {
      edgeMap.set(edgeId, {
        id: edgeId,
        from: centerKey,
        to: id,
        role: role || 'Related',
        tone: roleTone(role),
      })
    }
  }

  for (const item of items || []) {
    addNode(
      { key: item.key, kind: item.kind, name: item.name, namespace: item.namespace },
      item.role,
    )
  }

  for (const section of sections || []) {
    const sectionRole = section.title || section.id || 'Related'
    for (const field of section.fields || []) {
      if (!isRelationshipFieldLink(field.key)) continue
      const ref = resolveInspectRef(field.value, {
        fieldKey: field.key,
        section,
        groupId: 'relationships',
        inspectNamespace,
      })
      if (ref) addNode(ref, relationshipFieldRole(field.key) || sectionRole)
    }
    const cols = section.table?.columns || []
    for (const row of section.table?.rows || []) {
      const ref = pickRelationshipRefFromRow(row, cols, section, inspectNamespace)
      if (ref) addNode(ref.node, ref.role)
    }
  }

  const nodes = [...nodeMap.values()]
  const edges = [...edgeMap.values()]
  if (nodes.length <= 1) return emptyGraph()

  const roles = [...new Set(edges.map((e) => e.role))].sort(roleSort)
  return { centerKey, nodes, edges, roles }
}

export function layoutRelationshipGraph(graph, width, height = 0, { compact = false } = {}) {
  const orb = compact ? REL_ORB.compact : REL_ORB.normal
  const margin = 20
  const satellites = graph.nodes
    .filter((n) => !n.isCenter)
    .sort((a, b) => a.name.localeCompare(b.name))
  const count = satellites.length

  let w = Math.max(width, 280)
  let h = Math.max(height, compact ? 220 : 260)

  const center = graph.nodes.find((n) => n.isCenter)
  const minOrbit = orb.centerR + orb.satelliteR + (compact ? 56 : 72)
  let radius = minOrbit

  if (count === 1) radius = minOrbit
  else if (count === 2) radius = minOrbit + 12
  else radius = minOrbit + Math.min(48, count * 6)

  const labelReach = radius + orb.satelliteR + orb.labelH + 8
  h = Math.max(h, labelReach * 2 + orb.centerR + 36)
  w = Math.max(w, labelReach * 2 + orb.labelW / 2)

  let cx = w / 2
  let cy = h / 2 + 8

  if (center) {
    center.x = cx
    center.y = cy
    center.r = orb.centerR
  }

  for (let i = 0; i < count; i++) {
    const angle = count === 1
      ? -Math.PI / 2
      : (2 * Math.PI * i / count) - Math.PI / 2
    const node = satellites[i]
    node.x = cx + radius * Math.cos(angle)
    node.y = cy + radius * Math.sin(angle)
    node.r = orb.satelliteR
    node.angle = angle
    node.labelX = node.x + Math.cos(angle) * (orb.satelliteR + 14)
    node.labelY = node.y + Math.sin(angle) * (orb.satelliteR + 14)
    node.labelAlign = labelAlignForAngle(angle)
  }

  const bounds = measureOrbBounds(graph.nodes, orb, margin)
  w = Math.max(w, bounds.maxX - bounds.minX + margin * 2)
  h = Math.max(h, bounds.maxY - bounds.minY + margin * 2 + 28)

  const offsetX = (w - (bounds.maxX - bounds.minX)) / 2 - bounds.minX
  const offsetY = (h - (bounds.maxY - bounds.minY)) / 2 - bounds.minY + 14
  for (const node of graph.nodes) {
    node.x += offsetX
    node.y += offsetY
    if (node.labelX != null) {
      node.labelX += offsetX
      node.labelY += offsetY
    }
  }

  const positionedEdges = graph.edges.map((edge) => {
    const from = graph.nodes.find((n) => n.id === edge.from)
    const to = graph.nodes.find((n) => n.id === edge.to)
    if (!from || !to) return { ...edge, path: '', labelX: 0, labelY: 0 }
    const path = edgePathBetweenOrbs(from, to)
    const labelX = (from.x + to.x) / 2
    const labelY = (from.y + to.y) / 2
    return { ...edge, path, labelX, labelY }
  })

  return { ...graph, width: w, height: h, pad: margin, orb, edges: positionedEdges }
}

export function roleTone(role = '') {
  for (const [key, tone] of Object.entries(ROLE_TONES)) {
    if (role === key || role.startsWith(key)) return tone
  }
  if (/service|ingress|endpoint|route/i.test(role)) return 'network'
  if (/pod|owner|replica|deploy|job|cron/i.test(role)) return 'workload'
  if (/secret|config/i.test(role)) return 'config'
  if (/volume|claim|storage|pv/i.test(role)) return 'storage'
  if (/node/i.test(role)) return 'infra'
  return 'neutral'
}

export function shortenRoleLabel(role = '') {
  const s = String(role || 'Related')
    .replace(/ · .+$/, '')
    .replace(/ References$/, '')
    .replace(/ Assignment$/, '')
  if (s.length <= 14) return s
  return `${s.slice(0, 12)}…`
}

export function shortenNodeName(name = '', max = 22) {
  const s = String(name || '')
  if (s.length <= max) return s
  return `${s.slice(0, max - 1)}…`
}

function emptyGraph() {
  return { centerKey: '', nodes: [], edges: [], roles: [] }
}

function isRelationshipFieldLink(fieldKey = '') {
  const key = String(fieldKey || '').trim().toLowerCase()
  if (!key) return false
  if (/^tolerations?$|^node selector$|^scheduler$|^host network$|^termination grace$|^automount|^priority|^qos/.test(key)) {
    return false
  }
  return true
}

function relationshipFieldRole(fieldKey = '') {
  const key = String(fieldKey || '').trim()
  if (/^node$/i.test(key)) return 'Node'
  if (/nominated node/i.test(key)) return 'Nominated node'
  return ''
}

function pickRelationshipRefFromRow(row, columns, section, inspectNamespace) {
  const sectionId = section?.id || ''
  const sectionRole = relationshipSectionRole(section)
  const priority = relationshipColumnPriority(sectionId, columns)

  for (const colName of priority) {
    const idx = columns.findIndex((c) => String(c).toLowerCase() === colName.toLowerCase())
    if (idx < 0) continue
    const ref = resolveInspectRef(row[idx], {
      columnName: columns[idx],
      section,
      groupId: 'relationships',
      inspectNamespace,
      row,
      columns,
      columnIndex: idx,
    })
    if (ref) {
      return {
        node: ref,
        role: relationshipRowRole(sectionId, columns[idx], sectionRole),
      }
    }
  }

  for (let j = 0; j < row.length; j++) {
    const colName = columns[j]
    if (!linkableTableColumn(colName, 'relationships')) continue
    if (/^addresses?$|^ip$|^node$|^zone$|^ready$|^phase$|^hostname$/i.test(String(colName))) continue
    const ref = resolveInspectRef(row[j], {
      columnName: colName,
      section,
      groupId: 'relationships',
      inspectNamespace,
      row,
      columns,
      columnIndex: j,
    })
    if (ref) {
      return {
        node: ref,
        role: relationshipRowRole(sectionId, colName, sectionRole),
      }
    }
  }
  return null
}

function relationshipColumnPriority(sectionId, columns = []) {
  const id = String(sectionId || '').toLowerCase()
  if (id === 'backendaddresses' || id === 'legacyaddresses' || id === 'addresses') {
    return ['Target', 'Name', 'Pod']
  }
  if (id === 'endpointslices') return ['Name']
  if (id === 'targetpods') return ['Name']
  if (id === 'ownerrefs') return ['Kind', 'Name']
  if (id === 'pods' || id === 'targetpods') return ['Name']
  if (id === 'replicasets') return ['Name']
  if (id === 'jobs' || id === 'activejobs') return ['Name', 'Kind']
  const lower = columns.map((c) => String(c).toLowerCase())
  if (lower.includes('target')) return ['Target', 'Name', 'Pod', 'Kind']
  if (lower.includes('name')) return ['Name', 'Target', 'Pod']
  return ['Name', 'Target', 'Pod', 'Kind']
}

function relationshipSectionRole(section) {
  const id = String(section?.id || '').toLowerCase()
  if (id === 'endpointslices') return 'EndpointSlice'
  if (id === 'backendaddresses') return 'Backend'
  if (id === 'legacyaddresses' || id === 'serviceendpoint') return 'Endpoint'
  if (id === 'targetpods') return 'Target pod'
  if (id === 'ownerrefs') return 'Owner'
  if (id === 'pods') return 'Pod'
  if (id === 'replicasets') return 'ReplicaSet'
  if (id === 'jobs' || id === 'activejobs') return 'Job'
  return section?.title || section?.id || 'Related'
}

function relationshipRowRole(sectionId, columnName, fallback = 'Related') {
  const id = String(sectionId || '').toLowerCase()
  const col = String(columnName || '').toLowerCase()
  if (id === 'endpointslices' && col === 'name') return 'EndpointSlice'
  if ((id === 'backendaddresses' || id === 'legacyaddresses') && col === 'target') return 'Backend'
  if (id === 'targetpods' && col === 'name') return 'Target pod'
  if (id === 'pods' && col === 'name') return 'Pod'
  if (id === 'replicasets' && col === 'name') return 'ReplicaSet'
  if ((id === 'jobs' || id === 'activejobs') && (col === 'name' || col === 'kind')) return 'Job'
  return fallback
}

function roleSort(a, b) {
  const ai = ROLE_ORDER.indexOf(a)
  const bi = ROLE_ORDER.indexOf(b)
  const ar = ai >= 0 ? ai : ROLE_ORDER.length
  const br = bi >= 0 ? bi : ROLE_ORDER.length
  if (ar !== br) return ar - br
  return a.localeCompare(b)
}

function labelAlignForAngle(angle) {
  const deg = (angle * 180) / Math.PI
  if (deg > -30 && deg < 30) return 'left'
  if (deg >= 30 && deg < 150) return 'center'
  if (deg >= 150 || deg <= -150) return 'right'
  return 'center'
}

function measureOrbBounds(nodes, orb, pad) {
  let minX = Infinity
  let minY = Infinity
  let maxX = -Infinity
  let maxY = -Infinity
  for (const node of nodes) {
    const r = node.r || (node.isCenter ? orb.centerR : orb.satelliteR)
    minX = Math.min(minX, node.x - r - pad)
    maxX = Math.max(maxX, node.x + r + pad)
    minY = Math.min(minY, node.y - r - pad)
    maxY = Math.max(maxY, node.y + r + pad)
    if (!node.isCenter && node.labelX != null) {
      minX = Math.min(minX, node.labelX - orb.labelW / 2)
      maxX = Math.max(maxX, node.labelX + orb.labelW / 2)
      minY = Math.min(minY, node.labelY - 4)
      maxY = Math.max(maxY, node.labelY + orb.labelH)
    }
  }
  return { minX, minY, maxX, maxY }
}

function edgePathBetweenOrbs(from, to) {
  const fromR = from.r || 24
  const toR = to.r || 20
  const dx = to.x - from.x
  const dy = to.y - from.y
  const len = Math.hypot(dx, dy) || 1
  const x1 = from.x + (dx / len) * fromR
  const y1 = from.y + (dy / len) * fromR
  const x2 = to.x - (dx / len) * toR
  const y2 = to.y - (dy / len) * toR
  const mx = (x1 + x2) / 2
  const my = (y1 + y2) / 2
  const bow = from.isCenter ? 0 : 10
  const cx = mx + (-dy / len) * bow
  const cy = my + (dx / len) * bow
  return `M ${x1} ${y1} Q ${cx} ${cy} ${x2} ${y2}`
}
