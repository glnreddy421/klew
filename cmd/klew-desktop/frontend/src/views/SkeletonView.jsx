export function SkeletonView({ title, rows, wide }) {
  return (
    <section className={`skeleton-view ${wide ? 'wide' : ''}`}>
      <div className="skeleton-canvas">
        <div className="skeleton-placeholder">
          <span className="skeleton-label">{title}</span>
          <span className="skeleton-hint">UI skeleton — wire to api.View next</span>
        </div>
        <ul className="skeleton-list">
          {rows.map((r) => (
            <li key={r}><span className="skeleton-dot" />{r}</li>
          ))}
        </ul>
      </div>
    </section>
  )
}
