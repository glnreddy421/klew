import { useEffect, useMemo, useRef, useState } from 'react'
import { ChevronDown, PopoverPortal, usePopover, usePopoverFilter } from './ClusterNamespacePopover.jsx'
import { highlightRuns } from '../../lib/fuzzy.js'
import { kindHeading, kindsFromGroups, objectsOfKind, searchTargets, targetsLabel } from '../../lib/investigateTarget.js'
import { KindIcon } from '../KindIcon'

const STATUS_TEXT = {
  'no-permission': 'no permission',
  'not-installed': 'not installed',
  failed: 'could not be listed',
}

const GROUP_PREVIEW = 6
const MAX_RESULTS = 100

/**
 * Multi-select of what to investigate: all resources in the namespace, or any resources
 * in it. Browsing shows resources grouped by type; typing fuzzy-searches
 * names and types ("bil", "svc gw", "deploy/led"). Choices are a draft until
 * Apply. Types the engine could not list are named with the reason.
 */
export function TargetPopover({
  groups = [],
  unavailable = [],
  selected = [],
  onApply,
  disabled = false,
  loading = false,
  error = '',
}) {
  const { open, setOpen, anchorRef, panelRef } = usePopover()
  const { filter, setFilter, inputRef } = usePopoverFilter(open)
  const [draft, setDraft] = useState(() => new Set(selected))
  const [expanded, setExpanded] = useState(() => new Set())
  const [active, setActive] = useState(0)
  const [showGaps, setShowGaps] = useState(false)
  const listRef = useRef(null)

  useEffect(() => {
    if (!open) return
    setDraft(new Set(selected))
    setExpanded(new Set())
    setShowGaps(false)
  }, [open]) // eslint-disable-line react-hooks/exhaustive-deps -- reset the draft only when opening

  const query = filter.trim()
  const searching = query.length > 0
  const kinds = useMemo(() => kindsFromGroups(groups), [groups])
  const results = useMemo(() => (searching ? searchTargets(groups, query) : []), [groups, query, searching])
  const sections = useMemo(() => kinds.map((k) => {
    const objects = objectsOfKind(groups, k.kind)
    const full = expanded.has(k.kind) || objects.length <= GROUP_PREVIEW + 1
    return { ...k, objects, shown: full ? objects : objects.slice(0, GROUP_PREVIEW) }
  }), [kinds, groups, expanded])

  // Rows reachable with the arrow keys, in display order.
  const rows = useMemo(() => (searching
    ? results.slice(0, MAX_RESULTS)
    : sections.flatMap((s) => s.shown)), [searching, results, sections])

  useEffect(() => setActive(0), [query])
  useEffect(() => {
    listRef.current?.querySelector('[data-active="true"]')?.scrollIntoView({ block: 'nearest' })
  }, [active])

  const toggle = (key) => setDraft((prev) => {
    const next = new Set(prev)
    if (next.has(key)) next.delete(key)
    else next.add(key)
    return next
  })
  const setMany = (objects, on) => setDraft((prev) => {
    const next = new Set(prev)
    for (const o of objects) {
      if (on) next.add(o.key)
      else next.delete(o.key)
    }
    return next
  })
  const apply = () => {
    onApply?.([...draft])
    setOpen(false)
  }

  function onKeyDown(e) {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      if (!rows.length) return
      const step = e.key === 'ArrowDown' ? 1 : -1
      setActive((i) => (i + step + rows.length) % rows.length)
    } else if (e.key === 'Enter') {
      e.preventDefault()
      if (e.metaKey || e.ctrlKey) apply()
      else if (rows[active]) toggle(rows[active].key)
    }
  }

  const gaps = unavailable.filter((u) => STATUS_TEXT[u.status])
  const chosen = [...draft]
  const total = kinds.reduce((n, k) => n + k.count, 0)
  let rowIndex = -1

  const renderRow = (o, { withKind = false } = {}) => {
    rowIndex += 1
    const index = rowIndex
    const on = draft.has(o.key)
    return (
      <li key={o.key}>
        <button
          type="button"
          role="option"
          aria-selected={on}
          data-active={index === active}
          className={`target-picker-row ${on ? 'is-on' : ''} ${index === active ? 'is-active' : ''}`}
          onClick={() => toggle(o.key)}
          onMouseMove={() => setActive(index)}
        >
          <span className="target-picker-check" aria-hidden="true" />
          <span className="target-picker-name mono"><Highlight text={o.name} hits={o.nameHits} /></span>
          {withKind && <span className="target-picker-kind"><Highlight text={o.kind} hits={o.kindHits} /></span>}
        </button>
      </li>
    )
  }

  return (
    <div className="shell-popover-anchor shell-scope-anchor shell-scope-anchor-compact" ref={anchorRef}>
      <button
        type="button"
        className={`shell-scope-btn shell-scope-btn-compact target-picker-btn ${selected.length ? 'has-targets' : ''}`}
        onClick={() => setOpen((v) => !v)}
        disabled={disabled}
        aria-expanded={open}
        aria-haspopup="listbox"
        title={selected.length ? `Investigating ${selected.join(', ')}` : 'Investigate all resources in the namespace, or pick some'}
      >
        <span className="shell-scope-label muted">Target</span>
        <span className="shell-scope-value mono">{loading ? 'Loading…' : targetsLabel(selected)}</span>
        <ChevronDown />
      </button>
      <PopoverPortal
        open={open}
        anchorRef={anchorRef}
        panelRef={panelRef}
        className="shell-scope-popover target-picker"
        role="dialog"
        ariaLabel="Investigation target"
      >
        <div className="target-picker-search">
          <SearchIcon />
          <input
            ref={inputRef}
            type="text"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            onKeyDown={onKeyDown}
            placeholder={total ? `Search ${total} resources…` : 'Search resources…'}
            spellCheck={false}
            autoComplete="off"
            aria-label="Fuzzy search resources"
            aria-controls="target-picker-list"
          />
          {filter && (
            <button type="button" className="target-picker-clear-search" onClick={() => setFilter('')} aria-label="Clear search">×</button>
          )}
        </div>

        {chosen.length > 0 && (
          <div className="target-picker-chips" aria-label="Selected resources">
            {chosen.map((key) => (
              <button key={key} type="button" className="target-picker-chip" onClick={() => toggle(key)} title={`Remove ${key}`}>
                <span className="mono">{key.split('/').pop()}</span>
                <span className="target-picker-chip-kind">{key.split('/')[0]}</span>
                <span aria-hidden="true">×</span>
              </button>
            ))}
            <button type="button" className="target-picker-link" onClick={() => setDraft(new Set())}>Clear</button>
          </div>
        )}

        <ul className="target-picker-list" id="target-picker-list" role="listbox" aria-multiselectable="true" ref={listRef}>
          {!searching && (
            <li>
              <button
                type="button"
                className={`target-picker-row target-picker-whole ${draft.size === 0 ? 'is-on' : ''}`}
                onClick={() => setDraft(new Set())}
              >
                <span className="target-picker-radio" aria-hidden="true" />
                <span className="target-picker-whole-text">
                  <span className="target-picker-whole-title">All resources</span>
                  <span className="target-picker-whole-hint">Everything in the namespace Klew can read</span>
                </span>
              </button>
            </li>
          )}

          {searching && rows.map((o) => renderRow(o, { withKind: true }))}
          {searching && results.length > MAX_RESULTS && (
            <li className="target-picker-note">Showing {MAX_RESULTS} of {results.length}. Keep typing to narrow.</li>
          )}

          {!searching && sections.map((s) => {
            const allOn = s.objects.every((o) => draft.has(o.key))
            return (
              <li key={s.kind} className="target-picker-section">
                <div className="target-picker-heading">
                  <span className="target-picker-heading-icon" aria-hidden="true"><KindIcon kind={s.kind} size={13} /></span>
                  <span>{kindHeading(s.kind)}</span>
                  <span className="target-picker-count">{s.count}</span>
                  {s.custom && <span className="target-picker-tag">custom</span>}
                  <button type="button" className="target-picker-link" onClick={() => setMany(s.objects, !allOn)}>
                    {allOn ? 'None' : 'All'}
                  </button>
                </div>
                <ul>
                  {s.shown.map((o) => renderRow(o))}
                </ul>
                {s.shown.length < s.objects.length && (
                  <button
                    type="button"
                    className="target-picker-link target-picker-more"
                    onClick={() => setExpanded((prev) => new Set(prev).add(s.kind))}
                  >
                    Show {s.objects.length - s.shown.length} more
                  </button>
                )}
              </li>
            )
          })}

          {!loading && !rows.length && (
            <li className="target-picker-note">
              {searching ? `Nothing matches "${query}"` : 'No resources listed in this namespace'}
            </li>
          )}
        </ul>

        {error && <p className="target-picker-warn">Could not list resources: {error}</p>}
        {gaps.length > 0 && (
          <div className="target-picker-gaps" role="note">
            <button type="button" className="target-picker-gaps-toggle" onClick={() => setShowGaps((v) => !v)} aria-expanded={showGaps}>
              <span aria-hidden="true">⚠</span>
              {gaps.length} {gaps.length === 1 ? 'type' : 'types'} not listed
              <span className="muted">{showGaps ? 'hide' : 'why?'}</span>
            </button>
            {showGaps && (
              <ul>
                {gaps.map((u) => (
                  <li key={u.collector}>
                    <span className="mono">{u.collector}</span> · {STATUS_TEXT[u.status]}
                    {u.detail ? <span className="muted"> ({u.detail})</span> : null}
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}

        <div className="target-picker-footer">
          <span className="target-picker-keys muted">↑↓ move · ↵ select · ⌘↵ apply</span>
          <button type="button" className="shell-popover-footer-btn" onClick={() => setOpen(false)}>Cancel</button>
          <button type="button" className="shell-popover-footer-btn primary" onClick={apply}>
            Apply
            {draft.size > 0 && <span className="target-picker-apply-count">{draft.size}</span>}
          </button>
        </div>
      </PopoverPortal>
    </div>
  )
}

function Highlight({ text, hits }) {
  return highlightRuns(text, hits).map((r, i) => (r.hit ? <mark key={i}>{r.text}</mark> : <span key={i}>{r.text}</span>))
}

function SearchIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 16 16" fill="none" aria-hidden="true">
      <circle cx="7" cy="7" r="4.5" stroke="currentColor" strokeWidth="1.5" />
      <path d="M10.5 10.5L14 14" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  )
}
