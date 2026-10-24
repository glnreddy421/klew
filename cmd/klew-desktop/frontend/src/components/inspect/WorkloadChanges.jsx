import { useState } from 'react'
import {
  absoluteTime,
  actorBadge,
  diffRows,
  historyFooter,
  podRevisionLine,
  recentChangeBadge,
  relativeTime,
  replicaSetRevision,
  revisionLabel,
  sourceLabel,
} from '../../lib/workloadChanges'
import { InlineLoading } from '../LoadingSpinner.jsx'

/** Field-level diff table (Field, Before, After). Secret refs arrive as refs. */
export function ChangeDiffTable({ change }) {
  const rows = diffRows(change)
  if (!rows.length) return <p className="muted changes-diff-empty">No template field changes.</p>
  return (
    <div className="inspect-table-wrap">
      <table className="inspect-table changes-diff-table">
        <thead>
          <tr><th>Field</th><th>Before</th><th>After</th></tr>
        </thead>
        <tbody>
          {rows.map((r, i) => (
            <tr key={`${r[0]}-${i}`}>
              <td className="mono" title={r[0]}>{r[0]}</td>
              <td className="mono changes-before" title={r[1]}>{r[1]}</td>
              <td className="mono changes-after" title={r[2]}>{r[2]}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function ChangeRow({ change, now }) {
  const [open, setOpen] = useState(false)
  const actor = actorBadge(change.actor)
  const source = sourceLabel(change.source)
  return (
    <li className={`changes-row ${change.current ? 'is-current' : ''} ${open ? 'is-open' : ''}`}>
      <span className="changes-dot" aria-hidden="true" />
      <button type="button" className="changes-row-head" aria-expanded={open} onClick={() => setOpen((v) => !v)}>
        <span className="changes-row-meta">
          <time dateTime={change.time} title={absoluteTime(change.time)}>{relativeTime(change.time, now)}</time>
          <span className="mono">{revisionLabel(change)}</span>
          {change.current && <span className="changes-current">current</span>}
          {source !== 'rollout' && <span className={`changes-source is-${source}`}>{source}</span>}
          {actor && <span className={`changes-actor is-${actor}`} title={change.actor}>{actor === 'other' ? change.actor : actor}</span>}
        </span>
        <span className="changes-summary">{change.summary}</span>
        {change.changeCause && <span className="changes-cause muted" title={change.changeCause}>{change.changeCause}</span>}
      </button>
      {open && <ChangeDiffTable change={change} />}
    </li>
  )
}

/** Inspector → Changes tab. */
export function ChangesTab({ state, now = new Date() }) {
  const { data, loading, error } = state || {}
  if (!data && loading) return <InlineLoading message="Loading revision history…" className="muted" />
  if (error && !data) return <p className="changes-state tone-warn">{error}</p>
  if (!data) return null
  if (data.accessState === 'denied') return <p className="changes-state muted">{data.accessMessage}</p>
  const changes = data.changes || []
  if (!changes.length) return <p className="changes-state muted">No changes recorded by the cluster for this workload.</p>
  return (
    <section className="changes-tab">
      {data.helmNote && <p className="changes-note muted">{data.helmNote}</p>}
      <ol className="changes-timeline">
        {changes.map((c, i) => <ChangeRow key={`${c.revision}-${c.helmRevision ?? ''}-${i}`} change={c} now={now} />)}
      </ol>
      <p className="changes-footer muted">{historyFooter(data)}</p>
    </section>
  )
}

/** Summary-tab badge: "Changed 12m ago (rev 14)" → opens the Changes tab. */
export function RecentChangeBadge({ state, onOpen, now = new Date() }) {
  const text = recentChangeBadge(state?.data, now)
  if (!text) return null
  return (
    <button type="button" className="changes-recent-badge" onClick={onOpen} title="Open the Changes tab">
      {text}
    </button>
  )
}

/** Pod Summary line; highlighted when the Pod runs an older revision. */
export function PodRevisionLine({ state, now = new Date() }) {
  const line = podRevisionLine(state?.data, now)
  if (!line) return null
  return <p className={`changes-pod-line ${line.behind ? 'is-behind' : ''}`}>{line.text}</p>
}

/** ReplicaSet Summary: its revision and what it changed vs the previous one. */
export function ReplicaSetRevision({ state, rsName }) {
  const info = replicaSetRevision(state?.data, rsName)
  if (!info) return null
  return (
    <div className="changes-rs">
      <p className="changes-rs-line">{info.text}</p>
      {info.change.summary && <p className="muted changes-rs-summary">{info.change.summary}</p>}
      <ChangeDiffTable change={info.change} />
    </div>
  )
}
