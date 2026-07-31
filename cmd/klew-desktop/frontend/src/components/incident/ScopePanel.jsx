import { useEffect, useMemo, useState } from 'react'
import { KindIcon } from '../KindIcon'
import { RowStatusBadge } from './StatusBadge'
import {
  formatReady,
  groupRowsByKind,
  podsForMatch,
  WORKLOAD_ROOT_KINDS,
} from '../../lib/matches'

const ROOT_KEY = '__all__'

/**
 * Scope UX — searchable hierarchical matched components (kind → items → related pods).
 */
export function ScopePanel({
  rows = [],
  view,
  focusKey,
  inspectKey,
  mode = 'match',
  onInspect,
  onFocus,
  showFocusButton = true,
  title,
}) {
  const [query, setQuery] = useState('')
  const [expanded, setExpanded] = useState(() => new Set([ROOT_KEY]))

  const pods = view?.state?.snapshot?.pods || []
  const chain = mode === 'chain'
  const total = rows.length

  // Rebuild tree only when match/pod identity actually changes — not on every log tick.
  const rowSig = useMemo(
    () => rows.map((r) => `${r.key}:${r.status}:${r.ready}:${r.total}:${r.restarts}`).join('|'),
    [rows],
  )
  const podSig = useMemo(
    () => pods.map((p) => `${p.name}:${p.ready ? 1 : 0}:${p.restartCount || 0}`).join('|'),
    [pods],
  )

  const tree = useMemo(
    () => buildScopeTree(rows, pods, { nestPods: !chain }),
    // rowSig/podSig gate rebuilds; rows/pods provide the values.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [rowSig, podSig, chain],
  )

  // Open root + kind groups by default (only when new keys appear — avoid re-render thrash).
  useEffect(() => {
    setExpanded((prev) => {
      let changed = false
      const next = new Set(prev)
      if (!next.has(ROOT_KEY)) {
        next.add(ROOT_KEY)
        changed = true
      }
      for (const g of tree.groups) {
        const k = `kind:${g.kind}`
        if (!next.has(k)) {
          next.add(k)
          changed = true
        }
      }
      return changed ? next : prev
    })
  }, [tree])

  const filtered = useMemo(
    () => filterScopeTree(tree, query),
    [tree, query],
  )

  const toggle = (key) => {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }

  // Ensure visible groups stay open when searching.
  const isOpen = (key) => {
    if (query.trim()) return true
    return expanded.has(key)
  }

  if (!rows.length) {
    return (
      <div className="scope-panel">
        <header className="scope-header">
          <h3 className="scope-title">{title || 'Scope'}</h3>
        </header>
        <div className="scope-empty muted">No matched components in scope</div>
      </div>
    )
  }

  return (
    <div className="scope-panel">
      <header className="scope-header">
        <h3 className="scope-title">
          {title || (chain ? 'Focus chain' : 'Scope')}
          <span className="scope-title-count muted">
            · {total} match{total === 1 ? '' : 'es'}
          </span>
        </h3>
      </header>

      <div className="scope-search-wrap">
        <svg className="scope-search-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
          <circle cx="11" cy="11" r="7" />
          <path d="M20 20l-3.5-3.5" strokeLinecap="round" />
        </svg>
        <input
          className="scope-search"
          type="search"
          placeholder="Search components"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          aria-label="Search components"
        />
      </div>

      <div className="scope-tree" role="tree" aria-label={chain ? 'Focus chain' : 'Matched scope'}>
        {!chain && (
          <ScopeBranch
            open={isOpen(ROOT_KEY)}
            onToggle={() => toggle(ROOT_KEY)}
            depth={0}
            icon={null}
            label="All matches"
            count={filtered.count}
            kind="Folder"
          >
            {filtered.groups.map((group) => (
              <ScopeBranch
                key={group.kind}
                open={isOpen(`kind:${group.kind}`)}
                onToggle={() => toggle(`kind:${group.kind}`)}
                depth={1}
                icon={<KindIcon kind={group.kind} size={14} />}
                label={group.label}
                count={group.count}
                kind={group.kind}
              >
                {group.items.map((node) => (
                  <ScopeItemNode
                    key={node.row.key}
                    node={node}
                    depth={2}
                    focusKey={focusKey}
                    inspectKey={inspectKey}
                    showFocusButton={showFocusButton && !chain}
                    onInspect={onInspect}
                    onFocus={onFocus}
                    expanded={isOpen(`item:${node.row.key}`)}
                    onToggleExpand={() => toggle(`item:${node.row.key}`)}
                  />
                ))}
              </ScopeBranch>
            ))}
          </ScopeBranch>
        )}

        {chain && filtered.groups.flatMap((g) => g.items).map((node) => (
          <ScopeItemNode
            key={node.row.key}
            node={node}
            depth={0}
            focusKey={focusKey}
            inspectKey={inspectKey}
            showFocusButton={false}
            onInspect={onInspect}
            onFocus={onFocus}
            expanded={false}
            onToggleExpand={() => {}}
            forceLeaf
          />
        ))}

        {query.trim() && filtered.count === 0 && (
          <div className="scope-empty muted">No components match “{query.trim()}”</div>
        )}
      </div>
    </div>
  )
}

function ScopeBranch({ open, onToggle, depth, icon, label, count, kind, children }) {
  return (
    <div className="scope-branch" role="treeitem" aria-expanded={open} data-depth={depth}>
      <button
        type="button"
        className="scope-branch-row"
        style={{ '--scope-depth': depth }}
        onClick={onToggle}
      >
        <span className={`scope-chevron ${open ? 'open' : ''}`} aria-hidden="true">
          <Chevron />
        </span>
        {icon ? <span className="scope-kind-icon">{icon}</span> : <span className="scope-kind-icon scope-folder" aria-hidden="true"><FolderIcon /></span>}
        <span className="scope-branch-label" title={label}>{label}</span>
        <span className="scope-count mono">{count}</span>
      </button>
      {open && (
        <div className="scope-branch-children" role="group" data-kind={kind}>
          {children}
        </div>
      )}
    </div>
  )
}

function ScopeItemNode({
  node,
  depth,
  focusKey,
  inspectKey,
  showFocusButton,
  onInspect,
  onFocus,
  expanded,
  onToggleExpand,
  forceLeaf = false,
}) {
  const { row, children = [] } = node
  const hasKids = !forceLeaf && children.length > 0
  const active = inspectKey === row.key
  const isRoot = focusKey === row.key
  const tone = row.status || 'unknown'

  return (
    <div className="scope-item-block" role="treeitem" aria-expanded={hasKids ? expanded : undefined} data-depth={depth}>
      <div
        className={[
          'scope-item-row',
          active ? 'selected' : '',
          isRoot ? 'focus-root' : '',
        ].filter(Boolean).join(' ')}
        style={{ '--scope-depth': depth }}
      >
        {hasKids ? (
          <button
            type="button"
            className={`scope-chevron ${expanded ? 'open' : ''}`}
            aria-label={expanded ? 'Collapse' : 'Expand'}
            onClick={onToggleExpand}
          >
            <Chevron />
          </button>
        ) : (
          <span className="scope-chevron spacer" aria-hidden="true" />
        )}
        <button
          type="button"
          className="scope-item-main"
          onClick={() => onInspect?.(row.key)}
          title={`${row.kind}/${row.name}`}
        >
          <KindIcon kind={row.kind} size={14} />
          <span className="scope-item-name mono" title={row.name}>{row.name}</span>
          <span className="scope-item-ready muted">{formatReady(row.ready, row.total)}</span>
          <RowStatusBadge status={tone} />
        </button>
        {showFocusButton && (
          <button
            type="button"
            className="scope-focus-btn"
            title={`Focus ${row.kind}/${row.name}`}
            aria-label={`Focus ${row.name}`}
            onClick={(e) => {
              e.stopPropagation()
              onFocus?.(row.key)
            }}
          >
            Focus
          </button>
        )}
      </div>
      {hasKids && expanded && (
        <div className="scope-branch-children">
          {children.map((child) => (
            <ScopeItemNode
              key={child.row.key}
              node={child}
              depth={depth + 1}
              focusKey={focusKey}
              inspectKey={inspectKey}
              showFocusButton={false}
              onInspect={onInspect}
              onFocus={onFocus}
              expanded={false}
              onToggleExpand={() => {}}
              forceLeaf
            />
          ))}
        </div>
      )}
    </div>
  )
}

function buildScopeTree(rows, pods, { nestPods }) {
  const list = Array.isArray(rows) ? rows : []
  if (!nestPods) {
    const groups = groupRowsByKind(list).map((g) => ({
      kind: g.kind,
      label: g.label,
      count: g.items.length,
      items: g.items.map((row) => ({ row, children: [] })),
    }))
    return { count: list.length, groups }
  }

  const nestedPodKeys = new Set()
  const groups = groupRowsByKind(list).map((g) => {
    const items = g.items.map((row) => {
      const children = []
      if (WORKLOAD_ROOT_KINDS.includes(row.kind) || row.kind === 'Deployment') {
        const related = podsForMatch(row.ref || row, pods)
        for (const p of related) {
          const key = `Pod/${p.name}`
          nestedPodKeys.add(key)
          // Prefer existing match row for that pod when present
          const existing = list.find((r) => r.key === key)
          if (existing) {
            children.push({ row: existing, children: [] })
          } else {
            children.push({
              row: {
                key,
                kind: 'Pod',
                name: p.name,
                ref: { kind: 'Pod', name: p.name, namespace: p.namespace },
                ready: p.ready ? 1 : 0,
                total: 1,
                restarts: p.restartCount || 0,
                status: p.ready ? 'healthy' : 'degraded',
              },
              children: [],
            })
          }
        }
      }
      return { row, children }
    })
    return {
      kind: g.kind,
      label: g.label,
      count: items.length,
      items,
    }
  })

  // Drop top-level Pod group entries that are already nested under a workload
  const pruned = groups
    .map((g) => {
      if (g.kind !== 'Pod') return g
      const items = g.items.filter((n) => !nestedPodKeys.has(n.row.key))
      return { ...g, items, count: items.length }
    })
    .filter((g) => g.count > 0)

  return { count: list.length, groups: pruned }
}

function filterScopeTree(tree, query) {
  const q = String(query || '').trim().toLowerCase()
  if (!q) return tree

  const groups = []
  let count = 0
  for (const g of tree.groups) {
    const items = []
    for (const node of g.items) {
      const selfHit = matchesQuery(node.row, q) || g.label.toLowerCase().includes(q)
      const kids = (node.children || []).filter((c) => matchesQuery(c.row, q))
      if (selfHit || kids.length) {
        items.push({
          row: node.row,
          children: selfHit ? (node.children || []) : kids,
        })
        count += 1
      }
    }
    if (g.label.toLowerCase().includes(q) && !items.length) {
      // kind label hit but no items — show full group
      groups.push({ ...g })
      count += g.items.length
    } else if (items.length) {
      groups.push({ ...g, items, count: items.length })
    }
  }
  return { count, groups }
}

function matchesQuery(row, q) {
  const hay = `${row.kind || ''} ${row.name || ''} ${row.key || ''}`.toLowerCase()
  return hay.includes(q)
}

function Chevron() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" aria-hidden="true">
      <path d="M9 6l6 6-6 6" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}

function FolderIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" aria-hidden="true">
      <path d="M3 7a2 2 0 012-2h4l2 2h8a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V7z" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}
