import { Fragment, useEffect, useMemo, useRef, useState } from 'react'
import { GetRevisionCompare } from '../../../wailsjs/go/main/App'
import { compareOptions, defaultPair, diffLines, diffStats, foldUnchanged } from '../../lib/yamlDiff.js'
import { relativeTime } from '../../lib/workloadChanges.js'
import { InlineLoading } from '../LoadingSpinner.jsx'

// Revisions never change once stored, so a pair fetched once is kept for the
// session. Bounded so a long session cannot grow it without limit.
const compareCache = new Map()
const MAX_CACHED = 40

function cacheKey(target, from, to) {
  return [target.kind, target.namespace || '', target.name, from, to].join('|')
}

function useRevisionCompare(target, pair) {
  const key = target && pair ? cacheKey(target, pair.from, pair.to) : null
  const [result, setResult] = useState(() => (key ? compareCache.get(key) : null) || null)
  const [error, setError] = useState(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!key) return undefined
    const cached = compareCache.get(key)
    if (cached) {
      setResult(cached)
      setError(null)
      setLoading(false)
      return undefined
    }
    let cancelled = false
    setLoading(true)
    setError(null)
    GetRevisionCompare(target.kind, target.namespace || '', target.name, pair.from, pair.to)
      .then((data) => {
        if (cancelled) return
        if (compareCache.size >= MAX_CACHED) compareCache.delete(compareCache.keys().next().value)
        if (data?.accessState === 'allowed') compareCache.set(key, data)
        setResult(data)
      })
      .catch((err) => {
        if (!cancelled) setError(String(err?.message || err))
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
    // target fields are folded into key
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key])

  return { result: key && result && cacheKey(target, result.from?.revision, result.to?.revision) === key ? result : null, error, loading }
}

function optionLabel(c, now) {
  const parts = [`rev ${c.revision}`]
  if (c.current) parts.push('current')
  const when = relativeTime(c.time, now)
  if (when) parts.push(when)
  if (c.summary) parts.push(c.summary.length > 48 ? `${c.summary.slice(0, 47)}…` : c.summary)
  return parts.join(' · ')
}

function RevisionSelect({ label, value, options, onChange, now }) {
  return (
    <label className="compare-pick">
      <span className="compare-pick-label">{label}</span>
      <select
        className="compare-select mono"
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        aria-label={`${label} revision`}
      >
        {options.map((c) => (
          <option key={c.revision} value={c.revision}>{optionLabel(c, now)}</option>
        ))}
      </select>
    </label>
  )
}

function DiffRow({ row }) {
  const sign = row.type === 'add' ? '+' : row.type === 'del' ? '−' : ' '
  return (
    <div className={`compare-line is-${row.type}`}>
      <span className="compare-ln">{row.oldNo ?? ''}</span>
      <span className="compare-ln">{row.newNo ?? ''}</span>
      <span className="compare-sign" aria-hidden="true">{sign}</span>
      <span className="compare-code">{row.text || ' '}</span>
    </div>
  )
}

function DiffView({ rows, foldContext }) {
  const [opened, setOpened] = useState(() => new Set())
  const groups = useMemo(() => (foldContext ? foldUnchanged(rows, 3) : [{ kind: 'rows', rows }]), [rows, foldContext])
  useEffect(() => setOpened(new Set()), [rows])
  return (
    <div className="compare-diff mono" role="table" aria-label="YAML differences">
      {groups.map((g, gi) => {
        if (g.kind === 'rows' || opened.has(gi)) {
          return <Fragment key={gi}>{g.rows.map((r, i) => <DiffRow key={`${gi}-${i}`} row={r} />)}</Fragment>
        }
        return (
          <button
            key={gi}
            type="button"
            className="compare-gap"
            onClick={() => setOpened((s) => new Set(s).add(gi))}
          >
            Show {g.rows.length} unchanged lines
          </button>
        )
      })}
    </div>
  )
}

/**
 * Inspector → Compare tab: two stored revisions of a workload's pod template
 * as a YAML line diff. Read-only; data comes from GetRevisionCompare.
 */
export function CompareTab({ state, target, requestedPair, now = new Date() }) {
  const { data, loading: historyLoading, error: historyError } = state || {}
  const options = useMemo(() => compareOptions(data), [data])
  const [pair, setPair] = useState(() => defaultPair(options, requestedPair))
  const [changedOnly, setChangedOnly] = useState(true)

  // A request from the Changes tab picks the pair; a refreshed history keeps
  // the current pick while both revisions still exist.
  const lastRequest = useRef(requestedPair)
  useEffect(() => {
    const fresh = requestedPair && requestedPair !== lastRequest.current
    lastRequest.current = requestedPair
    setPair((current) => {
      const next = defaultPair(options, fresh ? requestedPair : (current || requestedPair))
      if (current && next && current.from === next.from && current.to === next.to) return current
      return next
    })
  }, [options, requestedPair])

  const { result, error, loading } = useRevisionCompare(target, pair)
  const rows = useMemo(() => (result ? diffLines(result.from?.yaml, result.to?.yaml) : null), [result])
  const stats = useMemo(() => diffStats(rows), [rows])

  if (!data && historyLoading) return <InlineLoading message="Loading revision history…" className="muted" />
  if (historyError && !data) return <p className="changes-state tone-warn">{historyError}</p>
  if (!data) return null
  if (data.accessState === 'denied') return <p className="changes-state muted">{data.accessMessage}</p>
  if (!pair) {
    return (
      <p className="changes-state muted">
        The cluster keeps only one revision of this workload, so there is nothing to compare yet.
      </p>
    )
  }

  const swap = () => setPair((p) => ({ from: p.to, to: p.from }))

  return (
    <section className="compare-tab">
      <div className="compare-controls">
        <RevisionSelect label="From" value={pair.from} options={options} now={now}
          onChange={(from) => setPair((p) => ({ ...p, from }))} />
        <button type="button" className="compare-swap" onClick={swap} title="Swap From and To" aria-label="Swap From and To">
          ⇄
        </button>
        <RevisionSelect label="To" value={pair.to} options={options} now={now}
          onChange={(to) => setPair((p) => ({ ...p, to }))} />
      </div>

      <div className="compare-bar">
        {rows && (
          <span className="compare-stats mono" aria-live="polite">
            <span className="compare-stat is-add">+{stats.added}</span>
            <span className="compare-stat is-del">−{stats.removed}</span>
            <span className="muted">lines</span>
          </span>
        )}
        {loading && <InlineLoading message="Reading revisions…" className="muted" />}
        <label className="compare-toggle">
          <input type="checkbox" checked={changedOnly} onChange={(e) => setChangedOnly(e.target.checked)} />
          Changed lines only
        </label>
      </div>

      {error && <p className="changes-state tone-warn">{error}</p>}
      {result?.accessState === 'denied' && <p className="changes-state muted">{result.accessMessage}</p>}
      {pair.from === pair.to && <p className="changes-state muted">Pick two different revisions to see what changed.</p>}
      {rows && pair.from !== pair.to && stats.added + stats.removed === 0 && (
        <p className="changes-state muted">These revisions have the same pod template.</p>
      )}
      {rows && rows.length > 0 && <DiffView rows={rows} foldContext={changedOnly} />}

      <p className="changes-footer muted">
        Pod template of each revision, as stored by the cluster. Template hash labels are left out.
        {result?.redacted > 0 && ` ${result.redacted} secret-like value${result.redacted === 1 ? ' is' : 's are'} hidden (best effort).`}
      </p>
    </section>
  )
}
