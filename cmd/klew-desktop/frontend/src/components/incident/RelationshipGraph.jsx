import { useEffect, useMemo, useRef, useState } from 'react'
import { KindIcon } from '../KindIcon'
import {
  buildRelationshipGraph,
  layoutRelationshipGraph,
  roleTone,
  shortenNodeName,
  shortenRoleLabel,
} from '../../lib/relationshipGraph'

export function RelationshipGraphPanel({
  center,
  items = [],
  sections = [],
  inspectNamespace = '',
  onInspect,
  title = 'Relationships',
  showTitle = true,
  compact = false,
}) {
  const graph = useMemo(
    () => buildRelationshipGraph({ center, items, sections, inspectNamespace }),
    [center, items, sections, inspectNamespace],
  )

  if (graph.nodes.length <= 1) return null

  return (
    <section className={`inspect-section inspect-relationships ${compact ? 'is-compact' : ''}`}>
      {showTitle && <h5 className="inspect-section-label">{title}</h5>}
      <RelationshipGraph graph={graph} onInspect={onInspect} compact={compact} />
    </section>
  )
}

export function RelationshipGraph({ graph, onInspect, compact = false }) {
  const hostRef = useRef(null)
  const [width, setWidth] = useState(360)
  const [activeId, setActiveId] = useState(null)
  const [focusRole, setFocusRole] = useState(null)

  useEffect(() => {
    const el = hostRef.current
    if (!el) return undefined
    const ro = new ResizeObserver((entries) => {
      const rect = entries[0]?.contentRect
      if (!rect?.width) return
      setWidth(Math.max(rect.width, 280))
    })
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  const laidOut = useMemo(
    () => layoutRelationshipGraph(graph, width, 0, { compact }),
    [graph, width, compact],
  )

  const centerNode = laidOut.nodes.find((n) => n.isCenter)
  const satellites = laidOut.nodes.filter((n) => !n.isCenter)
  const hoverId = activeId

  const edgeActive = (edge) => {
    if (!hoverId && !focusRole) return true
    if (focusRole && edge.role === focusRole) return true
    if (hoverId && (edge.from === hoverId || edge.to === hoverId)) return true
    return false
  }

  const nodeDimmed = (node) => {
    if (node.isCenter) return false
    if (!hoverId && !focusRole) return false
    if (focusRole) {
      const edge = laidOut.edges.find((e) => e.to === node.id)
      return edge?.role !== focusRole
    }
    return hoverId !== node.id && !laidOut.edges.some((e) =>
      (e.from === hoverId && e.to === node.id) || (e.to === hoverId && e.from === node.id),
    )
  }

  const activeNode = laidOut.nodes.find((n) => n.id === activeId) || centerNode

  return (
    <div
      ref={hostRef}
      className={`rel-graph ${compact ? 'is-compact' : ''}`}
      onMouseLeave={() => setActiveId(null)}
    >
      <div className="rel-graph-canvas" style={{ height: laidOut.height }}>
        {centerNode && (
          <header className="rel-graph-hud">
            <KindIcon kind={centerNode.kind} size={compact ? 14 : 16} />
            <div className="rel-graph-hud-text">
              <span className="rel-graph-hud-kind">{centerNode.kind}</span>
              <span className="rel-graph-hud-name mono" title={centerNode.name}>{centerNode.name}</span>
            </div>
          </header>
        )}

        <div className="rel-graph-field" aria-hidden="true">
          <span className="rel-graph-field-ring ring-1" />
          <span className="rel-graph-field-ring ring-2" />
          <span className="rel-graph-field-ring ring-3" />
        </div>

        <svg
          className="rel-graph-svg"
          viewBox={`0 0 ${laidOut.width} ${laidOut.height}`}
          preserveAspectRatio="xMidYMid meet"
          aria-hidden="true"
        >
          <defs>
            {laidOut.edges.map((edge) => (
              <linearGradient
                key={`grad-${edge.id}`}
                id={`rel-grad-${edge.id}`}
                gradientUnits="userSpaceOnUse"
                x1={centerNode?.x || 0}
                y1={centerNode?.y || 0}
                x2={laidOut.nodes.find((n) => n.id === edge.to)?.x || 0}
                y2={laidOut.nodes.find((n) => n.id === edge.to)?.y || 0}
              >
                <stop offset="0%" stopColor="var(--rel-tone-current, var(--accent))" stopOpacity="0.85" />
                <stop offset="100%" stopColor="var(--rel-tone-edge, var(--accent))" stopOpacity="0.35" />
              </linearGradient>
            ))}
          </defs>

          {centerNode && (
            <>
              <circle cx={centerNode.x} cy={centerNode.y} r={laidOut.orb.centerR + 18} className="rel-graph-hub-glow" />
              <circle cx={centerNode.x} cy={centerNode.y} r={laidOut.orb.centerR + 10} className="rel-graph-hub-ring" />
            </>
          )}

          {laidOut.edges.map((edge) => (
            <g
              key={edge.id}
              className={`rel-graph-edge tone-${edge.tone} ${edgeActive(edge) ? 'is-active' : 'is-dimmed'}`}
              style={{ '--rel-tone-edge': `var(--rel-tone-${edge.tone}, var(--accent))` }}
            >
              <path d={edge.path} className="rel-graph-edge-beam" />
              <path d={edge.path} className="rel-graph-edge-core" />
              <circle
                cx={laidOut.nodes.find((n) => n.id === edge.to)?.x}
                cy={laidOut.nodes.find((n) => n.id === edge.to)?.y}
                r="2.5"
                className="rel-graph-edge-node"
              />
            </g>
          ))}
        </svg>

        <div className="rel-graph-layer">
          {centerNode && (
            <HubOrb
              node={centerNode}
              layout={laidOut}
              compact={compact}
            />
          )}
          {satellites.map((node) => (
            <SatelliteOrb
              key={node.id}
              node={node}
              edge={laidOut.edges.find((e) => e.to === node.id)}
              layout={laidOut}
              compact={compact}
              dimmed={nodeDimmed(node)}
              active={activeId === node.id}
              onActivate={setActiveId}
              onInspect={onInspect}
            />
          ))}
        </div>

        {activeNode && !activeNode.isCenter && (
          <footer className="rel-graph-callout">
            <KindIcon kind={activeNode.kind} size={14} />
            <span className="mono">{activeNode.name}</span>
            <span className="muted">{activeNode.kind}</span>
            <button type="button" className="rel-graph-callout-go" onClick={() => onInspect?.(activeNode.key)}>
              Open
            </button>
          </footer>
        )}
      </div>

      {graph.roles?.length > 0 && (
        <div className="rel-graph-legend" aria-label="Relationship types">
          {graph.roles.map((role) => (
            <button
              key={role}
              type="button"
              className={[
                'rel-graph-legend-chip',
                `tone-${roleTone(role)}`,
                focusRole === role ? 'is-active' : '',
              ].filter(Boolean).join(' ')}
              onClick={() => setFocusRole((r) => (r === role ? null : role))}
            >
              {shortenRoleLabel(role)}
            </button>
          ))}
          {focusRole && (
            <button type="button" className="rel-graph-legend-clear" onClick={() => setFocusRole(null)}>
              Clear
            </button>
          )}
        </div>
      )}
    </div>
  )
}

function HubOrb({ node, layout, compact }) {
  const left = `${(node.x / layout.width) * 100}%`
  const top = `${(node.y / layout.height) * 100}%`
  const size = (node.r || layout.orb.centerR) * 2

  return (
    <div
      className="rel-graph-hub"
      style={{ left, top, width: size, height: size }}
      aria-hidden="true"
    >
      <span className="rel-graph-hub-core tone-center">
        <KindIcon kind={node.kind} size={compact ? 18 : 22} />
      </span>
    </div>
  )
}

function SatelliteOrb({
  node,
  edge,
  layout,
  compact,
  dimmed,
  active,
  onActivate,
  onInspect,
}) {
  const tone = edge?.tone || roleTone(node.role)
  const left = `${(node.x / layout.width) * 100}%`
  const top = `${(node.y / layout.height) * 100}%`
  const size = (node.r || layout.orb.satelliteR) * 2
  const labelLeft = `${(node.labelX / layout.width) * 100}%`
  const labelTop = `${(node.labelY / layout.height) * 100}%`

  return (
    <>
      <button
        type="button"
        className={[
          'rel-graph-sat',
          `tone-${tone}`,
          dimmed ? 'is-dimmed' : '',
          active ? 'is-active' : '',
        ].filter(Boolean).join(' ')}
        style={{ left, top, width: size, height: size }}
        onMouseEnter={() => onActivate?.(node.id)}
        onFocus={() => onActivate?.(node.id)}
        onClick={() => onInspect?.(node.key)}
        title={`${node.kind}/${node.name}`}
        aria-label={`Inspect ${node.kind} ${node.name}`}
      >
        <span className="rel-graph-sat-glow" aria-hidden="true" />
        <KindIcon kind={node.kind} size={compact ? 14 : 16} />
      </button>
      <div
        className={[
          'rel-graph-sat-label',
          `align-${node.labelAlign || 'center'}`,
          dimmed ? 'is-dimmed' : '',
          active ? 'is-active' : '',
        ].filter(Boolean).join(' ')}
        style={{ left: labelLeft, top: labelTop }}
      >
        <span className="rel-graph-sat-role">{shortenRoleLabel(edge?.role)}</span>
        <span className="rel-graph-sat-name mono">{shortenNodeName(node.name, compact ? 18 : 22)}</span>
      </div>
    </>
  )
}
