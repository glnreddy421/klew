export const NAV_ITEM_HOME = {
  id: 'home',
  label: 'Home',
  navLabel: 'Home',
  hint: 'Local clusters — pick where to connect',
}

export const NAV_ITEM_RESOURCES = {
  id: 'resources',
  label: 'Resources',
  navLabel: 'Resources',
  hint: 'Browse and inspect Kubernetes catalog',
  primary: true,
}

export const NAV_ITEMS_INVESTIGATION = [
  { id: 'incident', label: 'Overview', navLabel: 'Overview', hint: 'Investigation command center' },
  { id: 'patterns', label: 'Patterns', navLabel: 'Patterns', hint: 'Repeated and correlated behavior' },
  { id: 'failures', label: 'Failures', navLabel: 'Failures', hint: 'Concrete runtime failures' },
  { id: 'evidence', label: 'Evidence', navLabel: 'Evidence', hint: 'Raw facts behind conclusions' },
]

export const NAV_ITEMS = [NAV_ITEM_RESOURCES, ...NAV_ITEMS_INVESTIGATION]

export const NAV_ITEM_GRAPH = { id: 'graph', label: 'Graph', navLabel: 'Graph', hint: 'Resource relationships' }

/** Investigation results, shown as tabs on the Resources surface. */
export const NAV_ITEMS_INVESTIGATION_INLINE = [...NAV_ITEMS_INVESTIGATION, NAV_ITEM_GRAPH]

/** Tabs that belong to the Resources surface: browsing plus investigation results. */
export const RESOURCES_SURFACE_TABS = new Set([NAV_ITEM_RESOURCES.id, ...NAV_ITEMS_INVESTIGATION_INLINE.map((i) => i.id)])

export const NAV_ITEMS_SECONDARY = [
  { id: 'terminal', label: 'Terminal', navLabel: 'Terminal', hint: 'Context-aware cluster shell' },
  { id: 'live-logs', label: 'Live logs', navLabel: 'Live logs', hint: 'Stream container logs from selected pods (⌘⇧L)' },
]

export const SURFACE_META = {
  incident: {
    title: 'Overview',
    subtitle: 'Investigation brief — what Klew found and where to go next',
  },
  patterns: {
    title: 'Patterns',
    subtitle: 'Repeated and correlated behavior',
  },
  failures: {
    title: 'Failures',
    subtitle: 'Concrete failures in this investigation',
  },
  resources: {
    title: 'Resources',
    subtitle: 'Browse and inspect Kubernetes objects',
  },
  workloads: {
    title: 'Workloads',
    subtitle: 'Health, counts, and capacity across workload types',
  },
  evidence: {
    title: 'Evidence',
    subtitle: 'Correlated signals and supporting facts',
  },
  graph: {
    title: 'Graph',
    subtitle: 'How affected resources relate',
  },
  'live-logs': {
    title: 'Live logs',
    subtitle: 'Stream container logs from the pods you select',
  },
  terminal: {
    title: 'Terminal',
    subtitle: 'Context-aware shell for the selected cluster and namespace',
  },
  nodes: {
    title: 'Nodes',
    subtitle: 'Cluster inventory and investigation-scoped node context',
  },
  home: {
    title: 'Home',
    subtitle: 'Local clusters from kubeconfig',
  },
  settings: {
    title: 'Settings',
    subtitle: 'Preferences and cluster access',
  },
  'settings-help': {
    title: 'Help',
    subtitle: 'Shortcuts and documentation',
  },
}

/** @deprecated use NAV_ITEMS */
export const LEGACY_NAV = NAV_ITEMS

export const emptyView = () => ({
  summary: {},
  hypothesis: '',
  evidence: [],
  signals: [],
  state: {},
})
