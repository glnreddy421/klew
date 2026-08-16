import { useEffect, useState } from 'react'
import { isBlankInvestigationQuery } from '../../lib/investigationQuery'
import { defaultSelectedKeys, groupByKind, matchKey, normalizeMatches } from '../../lib/matches'
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
  const [selected, setSelected] = useState(() => defaultSelectedKeys(matches, undefined, { selectAll: true }))

  useEffect(() => {
    if (open) {
      setSelected(defaultSelectedKeys(matches, undefined, { selectAll: true }))
    }
  }, [open, matches])

  if (!open) return null

  const list = normalizeMatches(matches)
  const groups = groupByKind(list)
  const n = list.length

  function toggle(key) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }

  return (
    <div className="modal-overlay" role="dialog" aria-modal="true" aria-labelledby="scope-picker-title">
      <div className="modal scope-picker">
        <header className="scope-picker-header">
          <h2 id="scope-picker-title">Select investigation scope</h2>
          <p className="scope-picker-meta">
            {isBlankInvestigationQuery(query) ? (
              <>
                All {n} resource{n !== 1 ? 's' : ''} in {namespace}
              </>
            ) : (
              <>
                Query &quot;{query}&quot; matched {n} resource{n !== 1 ? 's' : ''} in {namespace}
              </>
            )}
            {contextLabel ? ` · ${contextLabel}` : ''}
          </p>
        </header>

        <div className="scope-picker-body">
          {groups.length > 0 && (
            <p className="scope-picker-section-label">Resource groups</p>
          )}
          {groups.map((g) => (
            <section key={g.kind} className="scope-group">
              <h3>
                <KindIcon kind={g.kind} size={13} />
                <span>{g.label}</span>
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
                        { !isBlankInvestigationQuery(query) && m.score != null && (
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
              {isBlankInvestigationQuery(query)
                ? 'No resources found in this namespace.'
                : 'No resources matched this query in the namespace.'}
            </p>
          )}
        </div>

        {!isBlankInvestigationQuery(query) && (
          <p className="scope-picker-hint muted">
            Tip: use deploy/name to target one kind
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
            Investigate all
          </button>
          <button type="button" className="btn btn-ghost" onClick={onCancel}>
            {mode === 'narrow' ? 'Back' : 'Cancel'}
          </button>
        </footer>
      </div>
    </div>
  )
}
