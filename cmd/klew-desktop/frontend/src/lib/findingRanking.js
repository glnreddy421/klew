/**
 * Key findings ranking: importance first (severity), then recency, then volume.
 * Routine lifecycle events stay in Patterns and Evidence but never lead Key findings.
 */

/** Known failure reasons and how severe they are. */
export const FAILURE_REASONS = {
  OOMKilled: 'critical',
  OOMKilling: 'critical',
  FailedScheduling: 'critical',
  FailedMount: 'critical',
  FailedAttachVolume: 'critical',
  ErrImagePull: 'critical',
  ImagePullBackOff: 'critical',
  CrashLoopBackOff: 'critical',
  Evicted: 'critical',
  NodeNotReady: 'critical',
  FailedCreate: 'high',
  FailedCreatePodSandBox: 'high',
  AdmissionDenied: 'high',
  BackOff: 'high',
  Unhealthy: 'high',
  Failed: 'high',
  FailedKillPod: 'high',
  ProbeWarning: 'warning',
  FailedSync: 'warning',
  NetworkNotReady: 'warning',
}

/** Normal lifecycle reasons excluded from Key findings by default. */
export const ROUTINE_REASONS = new Set([
  'Pulled', 'Pulling', 'Scheduled', 'Created', 'Started', 'SuccessfulCreate',
  'SuccessfulDelete', 'ScalingReplicaSet', 'Completed', 'Killing', 'SawCompletedJob',
  'SuccessfulRescale', 'NoPods', 'SandboxChanged',
])

const SEVERITY_RANK = { critical: 5, fatal: 5, error: 4, high: 4, warning: 3, warn: 3, degraded: 3, medium: 2, info: 1, low: 1 }

export function severityRank(sev) {
  return SEVERITY_RANK[String(sev || '').toLowerCase()] || 0
}

/** "[Pulled] Container image …" → "Pulled"; "BackOff" → "BackOff". */
export function reasonOf(text) {
  const s = String(text || '').trim()
  const m = s.match(/^\[([A-Za-z]+)\]/)
  if (m) return m[1]
  const word = s.match(/^([A-Z][A-Za-z]+)(\b|$)/)
  return word ? word[1] : ''
}

/** Killing is routine unless the message points at a failure (probe, eviction, OOM). */
export function isRoutine(reason, text = '') {
  if (!ROUTINE_REASONS.has(reason)) return false
  if (reason === 'Killing' && /liveness|readiness|startup probe|failed|evict|oom/i.test(text)) return false
  return true
}

/** Severity for a finding: known failure reasons win over the source's own label. */
export function findingSeverity(reason, fallback) {
  return FAILURE_REASONS[reason] || fallback || 'info'
}

/**
 * Sort findings by severity, then recency (lastSeen ms), then count, then title.
 * Drops routine reasons and info-level items that are not known failures.
 */
export function rankFindings(candidates) {
  return (candidates || [])
    .filter((f) => !isRoutine(f.reason, f.title))
    .filter((f) => severityRank(f.severity) >= severityRank('warning'))
    .sort((a, b) => severityRank(b.severity) - severityRank(a.severity)
      || (b.lastSeen || 0) - (a.lastSeen || 0)
      || (b.count || 0) - (a.count || 0)
      || String(a.title).localeCompare(String(b.title)))
}

/** Newest sample timestamp of a pattern template, in ms (0 when unknown). */
export function templateLastSeen(tpl) {
  let max = 0
  for (const s of tpl?.samples || []) {
    const t = new Date(s?.timestamp || 0).getTime()
    if (Number.isFinite(t) && t > max) max = t
  }
  return max
}
