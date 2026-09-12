/** Copy for investigation-only surfaces when the user is browsing without Investigate. */

export const INVESTIGATION_SURFACES = {
  overview: {
    id: 'overview',
    tone: 'overview',
    title: 'Overview',
    headline: 'Start an investigation to build the brief',
    description: 'Overview correlates signals, events, logs, and resource state into a single investigation brief.',
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
    whenLive: 'Findings update as Klew correlates new signals and events.',
    browseNote: 'Use Resources to browse the cluster without starting an investigation.',
  },
  patterns: {
    id: 'patterns',
    tone: 'patterns',
    title: 'Patterns',
    headline: 'Patterns need an active investigation',
    description: 'Patterns rank recurring log lines and Kubernetes events from workloads in your investigation scope.',
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
    whenLive: 'Log and event templates appear as streams arrive; filters in the sidebar narrow the list.',
    browseNote: 'Resources shows live object details without log pattern mining.',
  },
  failures: {
    id: 'failures',
    tone: 'failures',
    title: 'Failures',
    headline: 'Failures triage requires investigation scope',
    description: 'Failures ranks unhealthy Pods in the namespace you are investigating and surfaces container reasons and related events.',
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
    whenLive: 'Pod health refreshes during the investigation; use sidebar filters to narrow by severity or failure type.',
    browseNote: 'Inspect individual Pods from Resources for live status without triage ranking.',
  },
  evidence: {
    id: 'evidence',
    tone: 'evidence',
    title: 'Evidence',
    headline: 'Evidence fills in during an investigation',
    description: 'Evidence collects correlated log lines, Kubernetes events, object changes, and scored claims that support the overview verdict.',
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
    whenLive: 'Observations and claims accumulate while Investigate is running; sidebar filters slice by source type.',
    browseNote: 'Resources inspector tabs show live API details for one object at a time.',
  },
}

export function getInvestigationSurface(id) {
  return INVESTIGATION_SURFACES[id] || null
}
