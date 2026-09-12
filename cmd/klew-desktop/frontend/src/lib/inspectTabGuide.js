import { DETAIL_TAB_ORDER } from './objectDetails.js'

/** Browse-mode copy for inspector detail tabs. */
export const INSPECT_TAB_META = {
  summary: {
    description: 'Status, readiness, scheduling, and the first fields to check on this resource.',
    whenAvailable: 'Available for every resource once live details load from the cluster.',
    emptyHint: 'No summary fields yet. Details may still be loading, or the API returned an empty status.',
    scanFor: 'Phase, replicas, conditions, node placement, and scheduling rules.',
    kinds: null,
  },
  containers: {
    description: 'Container images, ports, probes, mounts, and CPU/memory requests.',
    whenAvailable: 'Shown for Pods. Controllers surface their Pods under Summary and Relationships instead.',
    emptyHint: 'No container rows returned for this Pod yet.',
    scanFor: 'Image, restarts, ready state, probes, and resource requests/limits.',
    kinds: ['Pod'],
  },
  relationships: {
    description: 'Owners, selectors, services, volumes, and other linked cluster objects.',
    whenAvailable: 'Appears when the API returns references or related resources you can open.',
    emptyHint: 'No relationships were returned for this resource.',
    scanFor: 'Owner chain, Services, PVCs, ConfigMaps, and click-through links to related objects.',
    kinds: null,
  },
  spec: {
    description: 'Desired configuration — replicas, update strategy, ports, volumes, and templates.',
    whenAvailable: 'Shown for controllers, networking, storage, and other spec-bearing kinds.',
    emptyHint: 'No spec sections were returned for this resource.',
    scanFor: 'Replica counts, selectors, ports, volume claims, and template settings.',
    kinds: null,
  },
  runtime: {
    description: 'Live CPU and memory use when cluster metrics are available.',
    whenAvailable: 'Requires metrics-server (or equivalent). Most useful for Pods and Nodes.',
    emptyHint: 'Runtime metrics are not available for this resource right now.',
    scanFor: 'Current CPU/memory use versus requests and limits.',
    kinds: ['Pod', 'Node'],
  },
  events: {
    description: 'Recent Kubernetes events — warnings, failures, pulls, and scheduling messages.',
    whenAvailable: 'Loaded from the cluster event stream for the selected object.',
    emptyHint: 'No recent events were recorded for this resource.',
    scanFor: 'Warning/Failed reasons, backoff messages, and scheduling failures.',
    kinds: null,
  },
  metadata: {
    description: 'Labels, annotations, and management metadata attached to the object.',
    whenAvailable: 'Shown when the resource carries labels and/or annotations.',
    emptyHint: 'This resource has no labels or annotations.',
    scanFor: 'Selectors, Helm release labels, GitOps annotations, and ownership tags.',
    kinds: null,
  },
}

export function tabAppliesToKind(tabId, kind) {
  const meta = INSPECT_TAB_META[tabId]
  if (!meta) return false
  if (!meta.kinds?.length) return true
  return meta.kinds.includes(kind)
}

/**
 * Merge live detail groups with browse tab catalog so users see what each tab is for,
 * even before data arrives.
 */
export function buildBrowseTabs(groups, inspect, { loading = false } = {}) {
  const kind = inspect?.kind || ''
  const byId = new Map((groups || []).map((g) => [g.id, g]))

  return DETAIL_TAB_ORDER.map((tab) => {
    const meta = INSPECT_TAB_META[tab.id] || {}
    const live = byId.get(tab.id)
    const sections = live?.sections || []
    const applies = tabAppliesToKind(tab.id, kind)
    const hasContent = sections.length > 0

    let state = 'empty'
    if (!applies) state = 'unavailable'
    else if (loading && !hasContent) state = 'loading'
    else if (hasContent) state = 'ready'

    return {
      id: tab.id,
      label: tab.label,
      sections,
      applies,
      hasContent,
      state,
      description: meta.description || '',
      whenAvailable: meta.whenAvailable || '',
      emptyHint: meta.emptyHint || '',
      scanFor: meta.scanFor || '',
    }
  }).filter((tab) => tab.applies || tab.hasContent)
}

export function activeTabGuide(tab, { loading = false } = {}) {
  if (!tab) return null
  const meta = INSPECT_TAB_META[tab.id] || {}
  const state = tab.state || (loading && !tab.hasContent ? 'loading' : tab.hasContent ? 'ready' : 'empty')

  return {
    state,
    description: tab.description || meta.description || '',
    whenAvailable: tab.whenAvailable || meta.whenAvailable || '',
    emptyHint: tab.emptyHint || meta.emptyHint || '',
    scanFor: tab.scanFor || meta.scanFor || '',
  }
}
