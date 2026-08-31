import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { StatusBadge } from '../components/incident/StatusBadge'
import { KindIcon } from '../components/KindIcon'
import { k8sIconUrl } from '../lib/k8sIcons'
import { normalizeHealth } from '../lib/investigationViews'

const R = 26
const MIN_K = 0.15
const MAX_K = 3

const KIND_ORDER = [
  'Ingress', 'Service', 'HPA', 'HorizontalPodAutoscaler',
  'Deployment', 'StatefulSet', 'DaemonSet', 'Job', 'CronJob',
  'ReplicaSet', 'Pod', 'Container', 'ConfigMap', 'Secret', 'PVC', 'Node',
]

/**
 * Icon-centric K8s relation graph — names live in the side list + hover pills.
 */
export function GraphView({ view, graphRelations }) {
  const layout = view?.graph || {}
  const rawNodes = layout.nodes || []
  const allEdges = layout.edges || []
  const rels = graphRelations || {}
  const edges = allEdges.filter((e) => {
    const key = e.relation === 'routesTo' ? 'routesTo' : e.relation
    if (rels[key] === false) return false
    if (e.relation === 'references' && rels.references === false) return false
    return true
  })

  const [selectedId, setSelectedId] = useState(null)
  const [hoverTip, setHoverTip] = useState(null) // { id, kind, name, x, y }
  const [mode, setMode] = useState('flow') // flow | radial
  const [camera, setCamera] = useState({ x: 0, y: 0, k: 1 })
  const [listFilter, setListFilter] = useState('')

  const stageRef = useRef(null)
  const cameraRef = useRef(camera)
  const dragRef = useRef(null)
  const rafRef = useRef(0)
  const fittedKeyRef = useRef('')

  cameraRef.current = camera

  const adjacency = useMemo(() => buildAdjacency(edges), [edges])
  const byId = useMemo(() => Object.fromEntries(rawNodes.map((n) => [n.id, n])), [rawNodes])

  const graphKey = useMemo(() => {
    const ids = rawNodes.map((n) => n.id).sort().join('|')
    return `${mode}:${ids}`
  }, [rawNodes, mode])

  const nodes = useMemo(() => {
    if (mode === 'radial') return layoutRadial(rawNodes)
    return rawNodes
  }, [mode, rawNodes])

  const nodeMap = useMemo(() => Object.fromEntries(nodes.map((n) => [n.id, n])), [nodes])

  const highlight = useMemo(
    () => (selectedId ? neighborhood(selectedId, adjacency, 3) : null),
    [selectedId, adjacency],
  )

  const selected = selectedId ? byId[selectedId] : null
  const related = useMemo(() => {
    if (!selectedId || !adjacency[selectedId]) return []
    return adjacency[selectedId]
      .map((link) => ({ ...link, node: byId[link.id] }))
      .filter((r) => r.node)
  }, [selectedId, adjacency, byId])

  const componentGroups = useMemo(() => {
    const q = listFilter.trim().toLowerCase()
    const filtered = rawNodes.filter((n) => {
      if (!q) return true
      return `${n.kind}/${n.name}`.toLowerCase().includes(q)
    })
    const groups = new Map()
    for (const n of filtered) {
      const kind = n.kind || 'Other'
      if (!groups.has(kind)) groups.set(kind, [])
      groups.get(kind).push(n)
    }
    for (const list of groups.values()) {
      list.sort((a, b) => {
        const hr = healthRank(b.health) - healthRank(a.health)
        if (hr) return hr
        return String(a.name).localeCompare(String(b.name))
      })
    }
    return [...groups.entries()].sort((a, b) => {
      const ia = KIND_ORDER.indexOf(a[0])
      const ib = KIND_ORDER.indexOf(b[0])
      return (ia === -1 ? 99 : ia) - (ib === -1 ? 99 : ib) || a[0].localeCompare(b[0])
    })
  }, [rawNodes, listFilter])

  const showTip = useCallback((n) => {
    if (!n) {
      setHoverTip(null)
      return
    }
    const t = cameraRef.current
    setHoverTip({
      id: n.id,
      kind: n.kind,
      name: n.name,
      x: t.x + (n.x || 0) * t.k,
      y: t.y + (n.y || 0) * t.k - (R + 8) * t.k,
    })
  }, [])

  const commitCamera = useCallback((next) => {
    cameraRef.current = next
    cancelAnimationFrame(rafRef.current)
    rafRef.current = requestAnimationFrame(() => {
      setCamera({ ...cameraRef.current })
    })
  }, [])

  const setCameraNow = useCallback((next) => {
    cameraRef.current = next
    setCamera(next)
  }, [])

  const zoomAt = useCallback((nextK, ox, oy) => {
    const t = cameraRef.current
    const k = clamp(nextK, MIN_K, MAX_K)
    if (k === t.k) return
    const scale = k / t.k
    commitCamera({
      k,
      x: ox - (ox - t.x) * scale,
      y: oy - (oy - t.y) * scale,
    })
  }, [commitCamera])

  const zoomBy = useCallback((factor) => {
    const el = stageRef.current
    if (!el) return
    const { width, height } = el.getBoundingClientRect()
    const t = cameraRef.current
    const k = clamp(t.k * factor, MIN_K, MAX_K)
    const scale = k / t.k
    setCameraNow({
      k,
      x: width / 2 - (width / 2 - t.x) * scale,
      y: height / 2 - (height / 2 - t.y) * scale,
    })
  }, [setCameraNow])

  const fitView = useCallback(() => {
    const el = stageRef.current
    if (!el || !nodes.length) return
    const { width, height } = el.getBoundingClientRect()
    if (width < 40 || height < 40) return

    const b = nodeBounds(nodes)
    const pad = 80
    const bw = Math.max(b.maxX - b.minX, R * 4)
    const bh = Math.max(b.maxY - b.minY, R * 4)
    const k = clamp(
      Math.min((width - pad * 2) / (bw + R * 2), (height - pad * 2) / (bh + R * 2 + 24)),
      MIN_K,
      1.4,
    )
    const cx = (b.minX + b.maxX) / 2
    const cy = (b.minY + b.maxY) / 2
    setCameraNow({
      k,
      x: width / 2 - cx * k,
      y: height / 2 - cy * k,
    })
  }, [nodes, setCameraNow])

  // Fit once per graph identity / layout mode — never on live poll ticks.
  useEffect(() => {
    if (!nodes.length) return undefined
    if (fittedKeyRef.current === graphKey) return undefined
    const id = requestAnimationFrame(() => {
      fitView()
      fittedKeyRef.current = graphKey
    })
    return () => cancelAnimationFrame(id)
  }, [graphKey, nodes.length, fitView])

  useEffect(() => {
    const el = stageRef.current
    if (!el) return undefined

    const onWheel = (e) => {
      e.preventDefault()
      e.stopPropagation()
      const rect = el.getBoundingClientRect()
      const mx = e.clientX - rect.left
      const my = e.clientY - rect.top
      const dy = e.deltaY
      // Discrete mouse wheels often report ±100; trackpads report small values.
      const factor = dy > 0 ? 0.9 : 1.1
      zoomAt(cameraRef.current.k * factor, mx, my)
    }

    el.addEventListener('wheel', onWheel, { passive: false })
    return () => el.removeEventListener('wheel', onWheel)
  }, [zoomAt])

  useEffect(() => () => cancelAnimationFrame(rafRef.current), [])

  const onPointerDown = (e) => {
    if (e.button !== 0) return
    if (e.target.closest('.rg-zoom')) return
    if (e.target.closest('[data-node-id]')) return
    setHoverTip(null)
    dragRef.current = {
      pid: e.pointerId,
      x: e.clientX,
      y: e.clientY,
      ox: cameraRef.current.x,
      oy: cameraRef.current.y,
      moved: false,
    }
    e.currentTarget.setPointerCapture?.(e.pointerId)
  }

  const onPointerMove = (e) => {
    const d = dragRef.current
    if (!d || d.pid !== e.pointerId) return
    const dx = e.clientX - d.x
    const dy = e.clientY - d.y
    if (Math.abs(dx) + Math.abs(dy) > 3) d.moved = true
    commitCamera({
      ...cameraRef.current,
      x: d.ox + dx,
      y: d.oy + dy,
    })
  }

  const onPointerUp = (e) => {
    const d = dragRef.current
    if (!d || d.pid !== e.pointerId) return
    dragRef.current = null
    if (!d.moved) setSelectedId(null)
    try {
      e.currentTarget.releasePointerCapture?.(e.pointerId)
    } catch {
      /* ignore */
    }
  }

  if (!rawNodes.length) {
    return (
      <div className="inv-page">
        <div className="inv-empty muted">
          Relationship graph not ready yet — waiting for matched objects and ownership edges.
        </div>
      </div>
    )
  }

  const counts = healthCounts(rawNodes)
  const zoomPct = Math.round(camera.k * 100)
  const worldTransform = `translate(${camera.x}, ${camera.y}) scale(${camera.k})`

  return (
    <div className="rg-page">
      <div className="rg-toolbar">
        <div className="rg-stats">
          <span className="rg-stat"><strong>{rawNodes.length}</strong> nodes</span>
          <span className="rg-stat"><strong>{edges.length}</strong> relations</span>
          {counts.critical > 0 && <span className="rg-stat tone-critical">{counts.critical} critical</span>}
          {counts.warning > 0 && <span className="rg-stat tone-warning">{counts.warning} warning</span>}
          <span className="rg-stat tone-healthy">{counts.healthy} healthy</span>
        </div>
        <div className="rg-mode" role="radiogroup" aria-label="Graph layout">
          <button
            type="button"
            role="radio"
            aria-checked={mode === 'flow'}
            className={`rg-mode-btn ${mode === 'flow' ? 'active' : ''}`}
            onClick={() => { fittedKeyRef.current = ''; setMode('flow') }}
          >
            Flow
          </button>
          <button
            type="button"
            role="radio"
            aria-checked={mode === 'radial'}
            className={`rg-mode-btn ${mode === 'radial' ? 'active' : ''}`}
            onClick={() => { fittedKeyRef.current = ''; setMode('radial') }}
          >
            Radial
          </button>
        </div>
      </div>

      <div className="rg-body">
        <div
          ref={stageRef}
          className="rg-stage"
          onPointerDown={onPointerDown}
          onPointerMove={onPointerMove}
          onPointerUp={onPointerUp}
          onPointerCancel={() => { dragRef.current = null }}
        >
          <svg className="rg-svg" role="img" aria-label="Kubernetes relationship graph">
            <defs>
              <marker id="rg-arrow" markerWidth="7" markerHeight="7" refX="6" refY="3" orient="auto">
                <path d="M0,0 L6,3 L0,6 Z" fill="currentColor" />
              </marker>
              <marker id="rg-arrow-hi" markerWidth="7" markerHeight="7" refX="6" refY="3" orient="auto">
                <path d="M0,0 L6,3 L0,6 Z" fill="var(--accent)" />
              </marker>
            </defs>
            <g transform={worldTransform}>
              {edges.map((e, i) => {
                const a = nodeMap[e.from]
                const b = nodeMap[e.to]
                if (!a || !b) return null
                const edgeHi = !highlight
                  || (highlight.has(e.from) && highlight.has(e.to) && isDirectEdge(adjacency, e.from, e.to))
                const dim = Boolean(highlight) && !edgeHi
                const hot = Boolean(highlight) && edgeHi
                const pts = edgeEndpoints(a, b, R + 2)
                return (
                  <g key={`${e.from}-${e.to}-${i}`} className={`rg-edge ${hot ? 'hot' : ''} ${dim ? 'dim' : ''}`}>
                    <line
                      x1={pts.x1}
                      y1={pts.y1}
                      x2={pts.x2}
                      y2={pts.y2}
                      markerEnd={hot ? 'url(#rg-arrow-hi)' : 'url(#rg-arrow)'}
                    />
                    {e.relation && hot && (
                      <text x={(pts.x1 + pts.x2) / 2} y={(pts.y1 + pts.y2) / 2 - 5} className="rg-edge-label">
                        {e.relation}
                      </text>
                    )}
                  </g>
                )
              })}

              {nodes.map((n) => {
                const hi = !highlight || highlight.has(n.id)
                const isSelected = n.id === selectedId
                const health = normalizeHealth(n.health)
                return (
                  <g
                    key={n.id}
                    data-node-id={n.id}
                    className={`rg-node health-${health} ${isSelected ? 'selected' : ''} ${!hi ? 'dim' : ''}`}
                    transform={`translate(${n.x || 0}, ${n.y || 0})`}
                    onClick={(ev) => {
                      ev.stopPropagation()
                      setSelectedId(n.id)
                    }}
                    onPointerEnter={() => showTip(n)}
                    onPointerLeave={() => setHoverTip((t) => (t?.id === n.id ? null : t))}
                  >
                    <circle className="rg-node-ring" r={R} />
                    <circle className="rg-node-disc" r={R - 3} />
                    <image
                      href={k8sIconUrl(n.kind)}
                      x={-14}
                      y={-14}
                      width={28}
                      height={28}
                      preserveAspectRatio="xMidYMid meet"
                    />
                  </g>
                )
              })}
            </g>
          </svg>

          {hoverTip && (
            <div
              className="rg-tip"
              style={{ left: hoverTip.x, top: hoverTip.y }}
              role="tooltip"
            >
              <KindIcon kind={hoverTip.kind} size={14} />
              <span className="rg-tip-kind">{hoverTip.kind}</span>
              <span className="rg-tip-name mono">{hoverTip.name}</span>
            </div>
          )}

          <div
            className="rg-zoom"
            role="group"
            aria-label="Zoom controls"
            onPointerDown={(e) => e.stopPropagation()}
          >
            <button type="button" className="rg-zoom-btn" onClick={() => zoomBy(1.25)} title="Zoom in" aria-label="Zoom in">+</button>
            <button
              type="button"
              className="rg-zoom-pct"
              onClick={() => {
                const el = stageRef.current
                if (!el) return
                const { width, height } = el.getBoundingClientRect()
                const t = cameraRef.current
                const k = 1
                const scale = k / t.k
                setCameraNow({
                  k,
                  x: width / 2 - (width / 2 - t.x) * scale,
                  y: height / 2 - (height / 2 - t.y) * scale,
                })
              }}
              title="Reset to 100%"
            >
              {zoomPct}%
            </button>
            <button type="button" className="rg-zoom-btn" onClick={() => zoomBy(0.8)} title="Zoom out" aria-label="Zoom out">−</button>
            <button
              type="button"
              className="rg-zoom-fit"
              onClick={() => {
                fittedKeyRef.current = graphKey
                fitView()
              }}
              title="Fit graph"
            >
              Fit
            </button>
          </div>

          <p className="rg-hint muted">Hover for name · click to select · scroll zoom</p>
        </div>

        <aside className="rg-side">
          {selected && (
            <div className="rg-selected-card">
              <div className="rg-side-head">
                <div className="rg-side-icon">
                  <KindIcon kind={selected.kind} size={28} />
                </div>
                <div className="rg-side-title">
                  <span className="inspect-category">{selected.kind}</span>
                  <h4 className="rg-side-name mono" title={selected.name}>{selected.name}</h4>
                  <StatusBadge status={normalizeHealth(selected.health)} />
                </div>
                <button
                  type="button"
                  className="rg-side-clear"
                  onClick={() => setSelectedId(null)}
                  aria-label="Clear selection"
                >
                  ×
                </button>
              </div>
              {related.length > 0 && (
                <>
                  <p className="rg-side-sub muted">
                    {related.length} relation{related.length === 1 ? '' : 's'}
                  </p>
                  <ul className="rg-rel-list">
                    {related.map((r) => (
                      <li key={`${r.dir}-${r.relation}-${r.id}`}>
                        <button type="button" className="rg-rel-row" onClick={() => setSelectedId(r.id)}>
                          <span className="rg-rel-dir">{r.dir === 'out' ? '→' : '←'}</span>
                          <span className="rg-rel-kind muted">{r.relation || 'related'}</span>
                          <KindIcon kind={r.node.kind} size={16} />
                          <span className="rg-rel-name mono" title={r.node.name}>{r.node.name}</span>
                        </button>
                      </li>
                    ))}
                  </ul>
                </>
              )}
            </div>
          )}

          <div className="rg-comp-head">
            <h4>Components</h4>
            <span className="muted">{rawNodes.length}</span>
          </div>
          <input
            className="rg-comp-filter"
            type="search"
            value={listFilter}
            onChange={(e) => setListFilter(e.target.value)}
            placeholder="Filter kind or name…"
            aria-label="Filter components"
          />
          <div className="rg-comp-scroll">
            {componentGroups.length === 0 ? (
              <p className="muted rg-comp-empty">No components match.</p>
            ) : (
              componentGroups.map(([kind, list]) => (
                <div key={kind} className="rg-comp-group">
                  <div className="rg-comp-kind">
                    <KindIcon kind={kind} size={14} />
                    <span>{kind}</span>
                    <span className="muted">{list.length}</span>
                  </div>
                  <ul className="rg-comp-list">
                    {list.map((n) => (
                      <li key={n.id}>
                        <button
                          type="button"
                          className={`rg-comp-row ${selectedId === n.id ? 'selected' : ''}`}
                          onClick={() => setSelectedId(n.id)}
                          onMouseEnter={() => {
                            const laid = nodeMap[n.id]
                            if (laid) showTip(laid)
                          }}
                          onMouseLeave={() => setHoverTip((t) => (t?.id === n.id ? null : t))}
                          title={`${n.kind}/${n.name}`}
                        >
                          <span className={`rg-comp-dot health-${normalizeHealth(n.health)}`} />
                          <span className="rg-comp-name mono">{n.name}</span>
                        </button>
                      </li>
                    ))}
                  </ul>
                </div>
              ))
            )}
          </div>
        </aside>
      </div>
    </div>
  )
}

function clamp(n, min, max) {
  return Math.min(max, Math.max(min, n))
}

function buildAdjacency(edges) {
  const map = {}
  for (const e of edges) {
    if (!map[e.from]) map[e.from] = []
    if (!map[e.to]) map[e.to] = []
    map[e.from].push({ id: e.to, relation: e.relation, dir: 'out' })
    map[e.to].push({ id: e.from, relation: e.relation, dir: 'in' })
  }
  return map
}

function isDirectEdge(adjacency, from, to) {
  return (adjacency[from] || []).some((l) => l.id === to)
}

function neighborhood(start, adjacency, depth) {
  const seen = new Set([start])
  let frontier = [start]
  for (let d = 0; d < depth; d++) {
    const next = []
    for (const id of frontier) {
      for (const link of adjacency[id] || []) {
        if (seen.has(link.id)) continue
        seen.add(link.id)
        next.push(link.id)
      }
    }
    frontier = next
    if (!frontier.length) break
  }
  return seen
}

function nodeBounds(nodes) {
  if (!nodes.length) return { minX: 0, maxX: 0, minY: 0, maxY: 0 }
  let minX = Infinity
  let maxX = -Infinity
  let minY = Infinity
  let maxY = -Infinity
  for (const n of nodes) {
    minX = Math.min(minX, n.x || 0)
    maxX = Math.max(maxX, n.x || 0)
    minY = Math.min(minY, n.y || 0)
    maxY = Math.max(maxY, n.y || 0)
  }
  return { minX, maxX, minY, maxY }
}

function edgeEndpoints(a, b, inset) {
  const x1 = a.x || 0
  const y1 = a.y || 0
  const x2 = b.x || 0
  const y2 = b.y || 0
  const dx = x2 - x1
  const dy = y2 - y1
  const len = Math.hypot(dx, dy) || 1
  const ux = dx / len
  const uy = dy / len
  return {
    x1: x1 + ux * inset,
    y1: y1 + uy * inset,
    x2: x2 - ux * inset,
    y2: y2 - uy * inset,
  }
}

function layoutRadial(nodes) {
  if (!nodes.length) return nodes
  const layerOf = (kind) => {
    switch (kind) {
      case 'Ingress': return 0
      case 'Service': return 1
      case 'HPA':
      case 'HorizontalPodAutoscaler': return 1
      case 'Deployment':
      case 'StatefulSet':
      case 'DaemonSet':
      case 'Job':
      case 'CronJob': return 2
      case 'ReplicaSet': return 3
      case 'Pod': return 4
      case 'Container': return 5
      case 'Node': return 6
      default: return 5
    }
  }

  const rings = new Map()
  for (const n of nodes) {
    const l = layerOf(n.kind)
    if (!rings.has(l)) rings.set(l, [])
    rings.get(l).push(n)
  }

  const hub = nodes.find((n) => ['Deployment', 'StatefulSet', 'DaemonSet'].includes(n.kind))
    || nodes.find((n) => n.kind === 'Service')
    || nodes[0]

  const out = []
  const layers = [...rings.keys()].sort((a, b) => a - b)
  for (const layer of layers) {
    const group = rings.get(layer)
    group.sort((a, b) => String(a.name).localeCompare(String(b.name)))
    const radius = layer === layerOf(hub.kind) && group.length === 1 ? 0 : 90 + layer * 78
    group.forEach((n, i) => {
      const angle = (-Math.PI / 2) + (i / Math.max(group.length, 1)) * Math.PI * 2
        + (group.length === 1 ? 0 : Math.PI / group.length)
      out.push({
        ...n,
        x: Math.cos(angle) * radius,
        y: Math.sin(angle) * radius,
      })
    })
  }

  const hubOut = out.find((n) => n.id === hub.id)
  if (hubOut && rings.get(layerOf(hub.kind))?.length === 1) {
    hubOut.x = 0
    hubOut.y = 0
  }

  return out
}

function healthCounts(nodes) {
  const c = { critical: 0, warning: 0, healthy: 0, unknown: 0 }
  for (const n of nodes) c[normalizeHealth(n.health)] += 1
  return c
}

function healthRank(h) {
  switch (normalizeHealth(h)) {
    case 'critical': return 3
    case 'warning': return 2
    case 'unknown': return 1
    default: return 0
  }
}
