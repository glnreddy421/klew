/**
 * Top-level cluster + investigation scope context for Overview.
 */
export function ClusterContextBar({ context, onNavigate, onChipAction }) {
  if (!context?.showBar) return null

  const {
    context: ctx,
    clusterName,
    namespace,
    query,
    chips,
  } = context

  const scopeParts = [
    ctx && { key: 'ctx', text: ctx, mono: true },
    clusterName && clusterName !== ctx && { key: 'cluster', text: clusterName, mono: true },
    namespace && { key: 'ns', text: namespace, mono: true, prefix: 'ns' },
  ].filter(Boolean)

  return (
    <section className="cluster-context-bar" aria-label="Cluster and investigation scope">
      <div className="cluster-context-scope">
        {scopeParts.map((part, i) => (
          <span key={part.key} className="cluster-context-scope-part">
            {i > 0 && <span className="cluster-context-sep muted" aria-hidden="true">/</span>}
            {part.prefix && <span className="cluster-context-prefix muted">{part.prefix} </span>}
            <span className={part.mono ? 'mono' : ''}>{part.text}</span>
          </span>
        ))}
        {query && (
          <span className="cluster-context-query muted mono" title={query}>
            · {query}
          </span>
        )}
      </div>

      {chips?.length > 0 && (
        <div className="cluster-context-chips">
          {chips.map((chip) => (
            <ContextChip
              key={chip.id}
              chip={chip}
              onNavigate={onNavigate}
              onChipAction={onChipAction}
            />
          ))}
        </div>
      )}
    </section>
  )
}

function ContextChip({ chip, onNavigate, onChipAction }) {
  const className = [
    'cluster-context-chip',
    chip.tone ? `tone-${chip.tone}` : '',
    chip.scope ? 'is-scope' : '',
    chip.clusterWide ? 'is-cluster' : '',
    chip.navTab || chip.action ? 'is-link' : '',
  ].filter(Boolean).join(' ')

  const handleClick = () => {
    if (chip.action) onChipAction?.(chip.action)
    else if (chip.navTab) {
      onNavigate?.(chip.nodesMode
        ? { tab: chip.navTab, nodesMode: chip.nodesMode }
        : chip.navTab)
    }
  }

  if (chip.navTab || chip.action) {
    return (
      <button
        type="button"
        className={className}
        title={chip.title}
        onClick={handleClick}
      >
        {chip.label}
      </button>
    )
  }

  return (
    <span className={className} title={chip.title}>
      {chip.label}
    </span>
  )
}
