import { getNodeMarker, nodeTypeClass, formatTimeAxis } from '../../lib/investigationOverviewVisuals.js'

export function InvestigationChainNode({
  node,
  highlighted = false,
  onSelect,
  compact = false,
}) {
  const marker = getNodeMarker(node.type)

  return (
    <button
      type="button"
      className={`inv-chain-node ${nodeTypeClass(node.type)} ${highlighted ? 'is-highlighted' : ''}`}
      onClick={() => onSelect?.(node)}
      title={buildNodeTitle(node)}
    >
      <span className="inv-chain-node-marker" aria-hidden="true">{marker}</span>
      <span className="inv-chain-node-label">{compact ? node.shortLabel : node.label}</span>
      {node.timestamp && (
        <span className="inv-chain-node-time mono">{formatTimeAxis(node.timestamp)}</span>
      )}
      {node.count > 1 && (
        <span className="inv-chain-node-count muted">×{node.count}</span>
      )}
    </button>
  )
}

export function InvestigationChain({
  chain,
  highlightedNodeIds,
  onNodeSelect,
  onNavigate,
}) {
  if (!chain) return null

  const highlighted = highlightedNodeIds instanceof Set
    ? highlightedNodeIds
    : new Set(Array.isArray(highlightedNodeIds) ? highlightedNodeIds : [])

  if (chain.mode === 'primary') {
    const node = chain.nodes[0]
    return (
      <section className="inv-visual-section inv-chain-section">
        <div className="inv-section-head">
          <h2 className="inv-section-title">{chain.label}</h2>
        </div>
        <div className="inv-primary-observation">
          <InvestigationChainNode
            node={node}
            highlighted
            onSelect={onNodeSelect}
          />
          {node.meta && <p className="inv-primary-meta muted">{node.meta}</p>}
          {chain.supportingCount > 0 && (
            <p className="inv-primary-support muted">
              Supporting evidence · {chain.supportingCount}
            </p>
          )}
          {chain.moreNav && (
            <button
              type="button"
              className="text-link-btn"
              onClick={() => onNavigate?.(chain.moreNav.tab)}
            >
              {chain.moreNav.label}
            </button>
          )}
        </div>
      </section>
    )
  }

  const isHorizontal = chain.nodes.length >= 2

  return (
    <section className="inv-visual-section inv-chain-section">
      <div className="inv-section-head">
        <h2 className="inv-section-title">{chain.label}</h2>
        {chain.hasMore && chain.moreNav && (
          <button
            type="button"
            className="text-link-btn"
            onClick={() => onNavigate?.(chain.moreNav.tab)}
          >
            {chain.moreNav.label}
          </button>
        )}
      </div>

      <div className={`inv-chain ${isHorizontal ? 'inv-chain-horizontal' : 'inv-chain-vertical'}`}>
        {chain.nodes.map((node, i) => (
          <div key={node.id} className="inv-chain-segment">
            {i > 0 && (
              <div className="inv-chain-edge" aria-hidden="true">
                <span className="inv-chain-edge-line" />
                <span className="inv-chain-edge-label muted">
                  {chain.edges[i - 1]?.label}
                </span>
              </div>
            )}
            <InvestigationChainNode
              node={node}
              highlighted={highlighted.has(node.id)}
              onSelect={onNodeSelect}
              compact={isHorizontal}
            />
          </div>
        ))}
      </div>
    </section>
  )
}

function buildNodeTitle(node) {
  const parts = [node.label]
  if (node.timestamp) parts.push(formatTimeAxis(node.timestamp))
  if (node.sourceKind && node.sourceName) parts.push(`${node.sourceKind}/${node.sourceName}`)
  if (node.meta) parts.push(node.meta)
  return parts.join('\n')
}
