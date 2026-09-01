export const NAV_ITEMS = [
  { id: 'incident', label: 'Overview', navLabel: 'Overview', hint: 'Investigation command center' },
  { id: 'patterns', label: 'Patterns', navLabel: 'Patterns', hint: 'Repeated and correlated behavior' },
  { id: 'failures', label: 'Failures', navLabel: 'Failures', hint: 'Concrete runtime failures' },
  { id: 'resources', label: 'Resources', navLabel: 'Resources', hint: 'Kubernetes catalog and entities' },
  { id: 'evidence', label: 'Evidence', navLabel: 'Evidence', hint: 'Raw facts behind conclusions' },
]

export const NAV_ITEMS_SECONDARY = [
  { id: 'graph', label: 'Graph', navLabel: 'Graph', hint: 'Resource relationships' },
  { id: 'terminal', label: 'Terminal', navLabel: 'Terminal', hint: 'Context-aware cluster shell' },
  { id: 'live-logs', label: 'Live logs', navLabel: 'Live logs', hint: 'Tail container logs from investigation pods' },
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
    subtitle: 'Kubernetes objects in scope',
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
    subtitle: 'Stream container logs from pods in scope',
  },
  terminal: {
    title: 'Terminal',
    subtitle: 'Context-aware shell for the selected cluster and namespace',
  },
  nodes: {
    title: 'Nodes',
    subtitle: 'Cluster inventory and investigation-scoped node context',
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
