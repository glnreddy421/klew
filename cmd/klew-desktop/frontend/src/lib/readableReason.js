/** Klew's internal evidence reasons → labels fit for the UI. */
export const INTERNAL_REASON_LABELS = {
  metrics_source: 'Metrics signal',
  events_watch_failed: 'Event watch interrupted',
  events_v1_watch_failed: 'Event watch interrupted',
  pods_watch_failed: 'Pod watch interrupted',
  deployments_watch_failed: 'Deployment watch interrupted',
  HypothesisChanged: 'Hypothesis changed',
}

/** Readable label for a reason; unknown reasons pass through unchanged. */
export function readableReason(reason) {
  const r = String(reason || '')
  if (INTERNAL_REASON_LABELS[r]) return INTERNAL_REASON_LABELS[r]
  // Any other snake_case identifier: "foo_bar" → "Foo bar".
  if (/^[a-z]+(_[a-z0-9]+)+$/.test(r)) {
    const words = r.replace(/_/g, ' ')
    return words.charAt(0).toUpperCase() + words.slice(1)
  }
  return r
}
