/** Copy for investigation-only surfaces when the user is browsing without Investigate. */

export const INVESTIGATION_SURFACES = {
  overview: {
    id: 'overview',
    tone: 'overview',
    title: 'Overview',
    headline: 'Your investigation brief starts here',
    description: 'Klew correlates signals, events, logs, and resource state into one live brief.',
    outcomes: [
      { label: 'Verdict', hint: 'Status and confidence' },
      { label: 'Failure chain', hint: 'What led here' },
      { label: 'Impact', hint: 'Affected resources' },
      { label: 'Next checks', hint: 'What to do next' },
    ],
    journey: [
      { label: 'Set scope', detail: 'One namespace' },
      { label: 'Investigate', detail: 'Start correlation' },
      { label: 'Read the brief', detail: 'Updates live' },
    ],
    // Kept for explorer + tests that still read steps/provides.
    provides: [
      'Verdict and key findings',
      'Failure chain and timeline',
      'Affected resources you can open in Resources',
      'Evidence preview and recommended next steps',
    ],
    steps: [
      'In the top bar, set Investigate scope to one namespace.',
      'Click Investigate to start correlation.',
      'Return here for the live brief while the session runs.',
    ],
    whenLive: 'The brief fills in as Klew correlates new signals.',
    browseNote: 'Or browse the cluster from Resources without investigating.',
    cue: 'Click Investigate in the bar above',
  },
  patterns: {
    id: 'patterns',
    tone: 'patterns',
    title: 'Patterns',
    headline: 'Recurring signals appear once you investigate',
    description: 'Patterns mines log lines and Kubernetes events from workloads in scope.',
    outcomes: [
      { label: 'Log templates', hint: 'Recurring lines' },
      { label: 'Event templates', hint: 'Pod and node noise' },
      { label: 'Trends', hint: 'Rising or new' },
      { label: 'Filters', hint: 'Narrow the list' },
    ],
    journey: [
      { label: 'Set scope', detail: 'One namespace' },
      { label: 'Investigate', detail: 'Tail logs & events' },
      { label: 'Open Patterns', detail: 'Explore templates' },
    ],
    provides: [
      'Log templates and top words from tailed container logs',
      'Event templates from Pod, Node, and PVC events',
      'Recurring and emerging pattern filters',
    ],
    steps: [
      'Choose one namespace in the top bar (Investigate scope).',
      'Click Investigate — Klew tails logs and watches events.',
      'Open Patterns after the first correlation pass to explore templates.',
    ],
    whenLive: 'Templates appear as streams arrive; sidebar filters narrow the list.',
    browseNote: 'Resources shows live object details without pattern mining.',
    cue: 'Click Investigate in the bar above',
  },
  failures: {
    id: 'failures',
    tone: 'failures',
    title: 'Failures',
    headline: 'Pod triage needs an investigation scope',
    description: 'Failures ranks unhealthy pods and surfaces container reasons with related events.',
    outcomes: [
      { label: 'Ranked pods', hint: 'Critical first' },
      { label: 'Dominant signal', hint: 'Top failure mode' },
      { label: 'Container state', hint: 'Exits, OOM, probes' },
      { label: 'Recent events', hint: 'For the selected pod' },
    ],
    journey: [
      { label: 'Set scope', detail: 'Failing namespace' },
      { label: 'Investigate', detail: 'Snapshot pods' },
      { label: 'Open Failures', detail: 'Triage and drill in' },
    ],
    provides: [
      'Severity-ranked pod list (critical, warning, stable)',
      'Dominant failure signal for the scope',
      'Container exit reasons, probes, OOM, and image pull failures',
      'Recent events for the selected pod',
    ],
    steps: [
      'Set Investigate scope to the namespace with failing workloads.',
      'Click Investigate to snapshot pods in that scope.',
      'Open Failures to triage unhealthy pods and drill into reasons.',
    ],
    whenLive: 'Pod health refreshes during the investigation; filter by severity in the sidebar.',
    browseNote: 'Inspect individual Pods from Resources for live status without triage ranking.',
    cue: 'Click Investigate in the bar above',
  },
  evidence: {
    id: 'evidence',
    tone: 'evidence',
    title: 'Evidence',
    headline: 'Supporting observations fill in during Investigate',
    description: 'Evidence collects correlated logs, events, changes, and scored claims behind the verdict.',
    outcomes: [
      { label: 'Evidence board', hint: 'Event ↔ logs' },
      { label: 'Observations', hint: 'By source type' },
      { label: 'Claims', hint: 'Scored hypotheses' },
      { label: 'Gaps', hint: 'What was not checked' },
    ],
    journey: [
      { label: 'Investigate', detail: 'Start correlation' },
      { label: 'Wait a pass', detail: 'Overview shows activity' },
      { label: 'Open Evidence', detail: 'Review support' },
    ],
    provides: [
      'Pattern evidence board linked to log templates',
      'Observations grouped by logs, events, changes, and metrics',
      'Scored claims and alternative hypotheses',
      'Gaps and recommended next checks',
    ],
    steps: [
      'Start an investigation on a single namespace.',
      'Wait for the first correlation pass (Overview will show activity).',
      'Open Evidence to review supporting observations and claims.',
    ],
    whenLive: 'Observations and claims accumulate while Investigate is running.',
    browseNote: 'Resources inspector tabs show live API details for one object at a time.',
    cue: 'Click Investigate in the bar above',
  },
}

export function getInvestigationSurface(id) {
  return INVESTIGATION_SURFACES[id] || null
}
