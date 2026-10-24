/**
 * Workload revision history ("Changes") — display helpers.
 * Data comes from GetWorkloadChanges / GetPodRevision; nothing here mutates the cluster.
 */

export const CHANGES_KINDS = new Set(['Deployment', 'StatefulSet', 'DaemonSet', 'HelmRelease'])
export const RECENT_CHANGE_MS = 24 * 3_600_000

function toDate(v) {
  if (!v) return null
  const d = v instanceof Date ? v : new Date(v)
  return Number.isNaN(d.getTime()) ? null : d
}

/** "just now", "12m ago", "3h ago", "2d ago". */
export function relativeTime(value, now = new Date()) {
  const d = toDate(value)
  if (!d) return ''
  const ms = toDate(now).getTime() - d.getTime()
  if (ms < 60_000) return 'just now'
  const m = Math.floor(ms / 60_000)
  if (m < 60) return `${m}m ago`
  const h = Math.floor(m / 60)
  if (h < 48) return `${h}h ago`
  return `${Math.floor(h / 24)}d ago`
}

/** Absolute time for hover: "Oct 21, 2026, 20:15:04". */
export function absoluteTime(value, { timeZone } = {}) {
  const d = toDate(value)
  if (!d) return ''
  return new Intl.DateTimeFormat('en-US', {
    year: 'numeric', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit',
    hour12: false, ...(timeZone ? { timeZone } : {}),
  }).format(d)
}

/** Badge family for an actor; null when unknown (no badge is shown). */
export function actorBadge(actor) {
  const a = String(actor || '').toLowerCase()
  if (!a) return null
  if (a === 'helm') return 'helm'
  if (a.startsWith('kubectl')) return 'kubectl'
  if (a === 'argocd') return 'argocd'
  if (a === 'flux') return 'flux'
  return 'other'
}

export function sourceLabel(source) {
  switch (source) {
    case 'rollback': return 'rollback'
    case 'restart': return 'restart'
    case 'helm': return 'helm'
    default: return 'rollout'
  }
}

/** Row heading: "rev 14", "helm rev 3", "rev 14 · helm rev 3". */
export function revisionLabel(change) {
  const parts = []
  if (change?.revision) parts.push(`rev ${change.revision}`)
  if (change?.helmRevision != null) parts.push(`helm rev ${change.helmRevision}`)
  return parts.join(' · ')
}

/** Summary-tab badge: "Changed 12m ago (rev 14)" when changed in the last 24h. */
export function recentChangeBadge(wc, now = new Date()) {
  const at = toDate(wc?.lastChangedAt)
  if (!at || wc?.accessState !== 'allowed') return null
  if (toDate(now).getTime() - at.getTime() > RECENT_CHANGE_MS) return null
  const rev = wc.currentRevision ? ` (rev ${wc.currentRevision})` : ''
  return `Changed ${relativeTime(at, now)}${rev}`
}

/** Diff table rows: [Field, Before, After]. */
export function diffRows(change) {
  return (change?.diffs || []).map((d) => [
    d.path,
    d.kind === 'added' ? '—' : (d.before || '—'),
    d.kind === 'removed' ? '—' : (d.after || '—'),
  ])
}

/** List footer: what the cluster does not keep. */
export function historyFooter(wc) {
  if (wc?.kind === 'HelmRelease') return 'Older Helm revisions are not retained by the cluster.'
  const limit = wc?.historyLimit
  return limit
    ? `Older revisions not retained by the cluster (revisionHistoryLimit = ${limit}).`
    : 'Older revisions not retained by the cluster.'
}

/**
 * Pod Summary line.
 *   behind:  "Running rev 13 · current is rev 14 (image changed 12m ago)"
 *   current: "Running current revision (rev 14)"
 * Returns null when the Pod has no revisioned owner.
 */
export function podRevisionLine(pr, now = new Date()) {
  if (!pr || pr.accessState !== 'allowed' || !pr.revision) return null
  if (!pr.behind) {
    return { text: `Running current revision (rev ${pr.revision})`, behind: false }
  }
  const what = pr.behindSummary ? `${pr.behindSummary} changed` : 'changed'
  const when = pr.currentSince ? ` ${relativeTime(pr.currentSince, now)}` : ''
  return {
    text: `Running rev ${pr.revision} · current is rev ${pr.currentRevision} (${what}${when})`,
    behind: true,
  }
}

/** ReplicaSet line: "Revision 13 of deployment/payment-api" plus its change vs the previous one. */
export function replicaSetRevision(wc, rsName) {
  const c = (wc?.changes || []).find((x) => x.objectName === rsName)
  if (!c) return null
  return {
    text: `Revision ${c.revision} of ${String(wc.kind).toLowerCase()}/${wc.name}${c.current ? ' (current)' : ''}`,
    change: c,
  }
}

const CHANGES_TAB = {
  id: 'changes',
  label: 'Changes',
  sections: [],
  applies: true,
  hasContent: true,
  state: 'ready',
}

/** Add the Changes tab after Summary for Deployment / StatefulSet / DaemonSet. */
export function withChangesTab(tabs, kind) {
  if (!CHANGES_KINDS.has(kind) || kind === 'HelmRelease' || tabs.some((t) => t.id === 'changes')) return tabs
  const out = [...tabs]
  const at = out.findIndex((t) => t.id === 'summary')
  out.splice(at >= 0 ? at + 1 : 0, 0, CHANGES_TAB)
  return out
}

/** Whose revision history the inspector needs: the workload itself, or a ReplicaSet's owning Deployment. */
export function changesTargetFor(inspect) {
  if (!inspect?.name) return null
  if (CHANGES_KINDS.has(inspect.kind) && inspect.kind !== 'HelmRelease') {
    return { kind: inspect.kind, name: inspect.name, generation: inspect.generation }
  }
  if (inspect.kind === 'ReplicaSet') {
    const owner = (inspect.relationships || []).find((r) => r.kind === 'Deployment' && r.name)
    if (owner) return { kind: 'Deployment', name: owner.name }
    // Browse mode has no relationships list: Deployment ReplicaSets are
    // "<deployment>-<pod-template-hash>". A wrong guess shows nothing, because
    // the revision line requires this ReplicaSet in the fetched history.
    const guess = inspect.name.replace(/-[a-z0-9]{5,10}$/, '')
    if (guess && guess !== inspect.name) return { kind: 'Deployment', name: guess }
  }
  return null
}
