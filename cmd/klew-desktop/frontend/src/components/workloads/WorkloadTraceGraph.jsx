import { KindIcon } from '../KindIcon.jsx'
import { InlineLoading } from '../LoadingSpinner.jsx'
import { shortenNodeName } from '../../lib/relationshipGraph.js'

export function WorkloadTraceGraph({
  trace,
  loading = false,
  onInspect,
  compact = false,
}) {
  const nodes = trace?.nodes || []
  const edges = trace?.edges || []

  if (loading && nodes.length <= 1) {
    return (
      <section className={`workload-trace ${compact ? 'is-compact' : ''}`} aria-busy="true">
        <header className="workload-trace-head">
          <h5 className="workload-trace-title">Connection trace</h5>
          <InlineLoading label="Loading related resources…" />
        </header>
        <div className="workload-trace-skeleton" aria-hidden="true">
          <span /><span /><span /><span />
        </div>
      </section>
    )
  }

  if (nodes.length <= 1) return null

  return (
    <section className={`workload-trace ${compact ? 'is-compact' : ''}`}>
      <header className="workload-trace-head">
        <div className="workload-trace-head-copy">
          <h5 className="workload-trace-title">Connection trace</h5>
          <p className="workload-trace-subtitle muted">
            How this workload connects from ingress through services to pods
            {trace.podCount > 1 ? ` · ${trace.podCount} pods in scope` : ''}
          </p>
        </div>
      </header>

      <div className="workload-trace-scroll">
        <div className="workload-trace-track" role="list">
          {nodes.map((node, index) => (
            <div key={node.key} className="workload-trace-step" role="listitem">
              {index > 0 && (
                <TraceConnector label={edges[index - 1]?.label} />
              )}
              <button
                type="button"
                className={[
                  'workload-trace-node',
                  node.isFocus ? 'is-focus' : '',
                  node.badge?.tone === 'failure' ? 'tone-crit' : node.badge?.tone === 'warn' ? 'tone-warn' : '',
                ].filter(Boolean).join(' ')}
                onClick={() => onInspect?.(node.key)}
                title={`${node.kind}/${node.name}`}
              >
                <span className="workload-trace-node-icon">
                  <KindIcon kind={node.kind} size={16} />
                </span>
                <span className="workload-trace-node-kind">{node.kind}</span>
                <span className="workload-trace-node-name mono">{shortenNodeName(node.name, 22)}</span>
                {node.extraCount > 0 && (
                  <span className="workload-trace-node-more">+{node.extraCount}</span>
                )}
                {node.badge && (
                  <span className={`workload-trace-node-badge tone-${node.badge.tone}`}>
                    {node.badge.text}
                  </span>
                )}
                {node.isFocus && (
                  <span className="workload-trace-node-focus">selected</span>
                )}
              </button>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}

function TraceConnector({ label }) {
  return (
    <div className="workload-trace-connector" aria-hidden="true">
      {label ? <span className="workload-trace-connector-label">{label}</span> : null}
      <span className="workload-trace-connector-line">
        <span className="workload-trace-connector-arrow" />
      </span>
    </div>
  )
}
