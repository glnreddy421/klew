import { useEffect, useMemo, useRef, useState } from 'react'
import { isBlankInvestigationQuery } from '../../lib/investigationQuery'
import {
  groupByKind,
  matchKey,
  normalizeMatches,
  smartDefaultSelectedKeys,
  WORKLOAD_ROOT_KINDS,
} from '../../lib/matches'
import { filterScopeMatches, kindFiltersForMatches } from '../../lib/scopeSearch'
import { KindIcon } from '../KindIcon'

export function ScopePickerModal({
  open,
  query,
  namespace,
  contextLabel,
  matches,
  mode,
  onConfirm,
  onCancel,
}) {
  const [selected, setSelected] = useState(() => new Set())
  const [filter, setFilter] = useState('')
  const [kindFilter, setKindFilter] = useState('')
  const searchRef = useRef(null)

  const list = normalizeMatches(matches)
  const filteredList = useMemo(
    () => filterScopeMatches(list, { text: filter, kind: kindFilter }),
    [list, filter, kindFilter],
  )
  const groups = useMemo(() => groupByKind(filteredList), [filteredList])
  const kindFilters = useMemo(() => kindFiltersForMatches(list), [list])
  const visibleKeys = useMemo(
    () => filteredList.map((m) => matchKey(m.ref)).filter(Boolean),
    [filteredList],
  )

  useEffect(() => {
    if (!open) return
    setFilter('')
    setKindFilter('')
    setSelected(smartDefaultSelectedKeys(list, query))
    const t = window.setTimeout(() => searchRef.current?.focus(), 0)
    return () => window.clearTimeout(t)
  }, [open, matches, query])

  if (!open) return null

  const n = list.length
  const visibleN = filteredList.length

  function toggle(key) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }

  function selectVisible() {
    setSelected((prev) => {
      const next = new Set(prev)
      for (const key of visibleKeys) next.add(key)
      return next
    })
  }

  function selectWorkloads() {
    const keys = list
      .filter((m) => WORKLOAD_ROOT_KINDS.includes(m.ref?.kind))
      .map((m) => matchKey(m.ref))
    setSelected(new Set(keys))
  }

  function clearSelection() {
    setSelected(new Set())
  }

  return (
    <div className="modal-overlay" role="dialog" aria-modal="true" aria-labelledby="scope-picker-title">
      <div className="modal scope-picker">
        <header className="scope-picker-header">
          <h2 id="scope-picker-title">Select investigation scope</h2>
          <p className="scope-picker-meta">
            {isBlankInvestigationQuery(query) ? (
              <>
                {n} resource{n !== 1 ? 's' : ''} in {namespace}
              </>
            ) : (
              <>
                Query &quot;{query}&quot; matched {n} resource{n !== 1 ? 's' : ''} in {namespace}
              </>
            )}
            {contextLabel ? ` · ${contextLabel}` : ''}
            {filter || kindFilter ? (
              <span> · showing {visibleN}</span>
            ) : null}
          </p>
        </header>

        <div className="scope-picker-toolbar">
          <input
            ref={searchRef}
            type="search"
            className="scope-picker-search"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder="Search resources by name or kind…"
            spellCheck={false}
            autoComplete="off"
            aria-label="Filter resources"
          />
          {kindFilters.length > 1 && (
            <div className="scope-picker-kind-filters" role="tablist" aria-label="Filter by kind">
              <button
                type="button"
                className={`scope-picker-kind-chip ${kindFilter === '' ? 'active' : ''}`}
                onClick={() => setKindFilter('')}
              >
                All
              </button>
              {kindFilters.map((kind) => (
                <button
                  key={kind}
                  type="button"
                  className={`scope-picker-kind-chip ${kindFilter === kind ? 'active' : ''}`}
                  onClick={() => setKindFilter(kindFilter === kind ? '' : kind)}
                >
                  {kind}
                </button>
              ))}
            </div>
          )}
          <div className="scope-picker-bulk-actions">
            <button type="button" className="btn btn-ghost btn-sm" onClick={selectWorkloads}>
              Workloads
            </button>
            <button type="button" className="btn btn-ghost btn-sm" onClick={selectVisible}>
              Select visible
            </button>
            <button type="button" className="btn btn-ghost btn-sm" onClick={clearSelection}>
              Clear
            </button>
          </div>
        </div>

        <div className="scope-picker-body">
          {groups.map((g) => (
            <section key={g.kind} className="scope-group">
              <h3>
                <KindIcon kind={g.kind} size={13} />
                <span>{g.label}</span>
                <span className="scope-group-count muted">{g.items.length}</span>
              </h3>
              <ul className="scope-items">
                {g.items.map((m) => {
                  const key = matchKey(m.ref)
                  return (
                    <li key={key}>
                      <label className="scope-item">
                        <input
                          type="checkbox"
                          checked={selected.has(key)}
                          onChange={() => toggle(key)}
                        />
                        <KindIcon kind={m.ref?.kind} size={13} />
                        <span className="scope-item-name">{m.ref.name}</span>
                        {!isBlankInvestigationQuery(query) && m.score != null && (
                          <span className="scope-item-score muted">{Math.round(m.score * 100)}%</span>
                        )}
                      </label>
                    </li>
                  )
                })}
              </ul>
            </section>
          ))}
          {!groups.length && (
            <p className="muted">
              {filter || kindFilter
                ? 'No resources match the current filters.'
                : isBlankInvestigationQuery(query)
                  ? 'No resources found in this namespace.'
                  : 'No resources matched this query in the namespace.'}
            </p>
          )}
        </div>

        {!isBlankInvestigationQuery(query) && (
          <p className="scope-picker-hint muted">
            Tip: use deploy/name in the top bar to narrow before investigating
          </p>
        )}

        <footer className="scope-picker-actions">
          <button
            type="button"
            className="btn btn-primary"
            disabled={selected.size === 0}
            onClick={() => onConfirm({ selectedKeys: selected, investigateAll: false })}
          >
            Investigate selected ({selected.size})
          </button>
          <button
            type="button"
            className="btn btn-outline"
            onClick={() => onConfirm({ selectedKeys: null, investigateAll: true })}
          >
            Investigate all ({n})
          </button>
          <button type="button" className="btn btn-ghost" onClick={onCancel}>
            {mode === 'narrow' ? 'Back' : 'Cancel'}
          </button>
        </footer>
      </div>
    </div>
  )
}
