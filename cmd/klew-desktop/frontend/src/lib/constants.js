export const NAV_ITEMS = [
  { id: 'incident', label: 'Overview', navLabel: 'Overview', hint: 'Investigation home — matches and status' },
  { id: 'patterns', label: 'Patterns', navLabel: 'Patterns', hint: 'Log and event patterns' },
  { id: 'graph', label: 'Graph', navLabel: 'Graph', hint: 'Relations' },
  { id: 'failures', label: 'Failures', navLabel: 'Failures', hint: 'Failing runtime' },
  { id: 'resources', label: 'Resources', navLabel: 'Resources', hint: 'CPU & memory' },
  { id: 'evidence', label: 'Evidence', navLabel: 'Evidence', hint: 'Correlated signals and verdict proof' },
]

export const emptyView = () => ({
  summary: {},
  hypothesis: '',
  evidence: [],
  signals: [],
  state: {},
})
