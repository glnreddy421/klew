/** Compact context banner shown on investigation lenses when an entity is selected. */
export function InvestigationContextBanner({ inspectRow, onClear, onOpenResources }) {
  if (!inspectRow?.kind || !inspectRow?.name) return null
  return (
    <div className="investigation-context-banner">
      <span className="muted">
        Context: <strong>{inspectRow.kind}/{inspectRow.name}</strong>
      </span>
      <div className="investigation-context-actions">
        {onOpenResources && (
          <button type="button" className="text-link-btn" onClick={onOpenResources}>
            Open in Resources
          </button>
        )}
        {onClear && (
          <button type="button" className="text-link-btn" onClick={onClear}>
            Clear
          </button>
        )}
      </div>
    </div>
  )
}
