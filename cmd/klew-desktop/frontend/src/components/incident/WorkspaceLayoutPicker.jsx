import { WORKSPACE_LAYOUTS } from '../../lib/incidentLayout'

export function WorkspaceLayoutPicker({ value, onChange }) {
  return (
    <div className="workspace-layout-picker" role="radiogroup" aria-label="Workspace layout">
      {WORKSPACE_LAYOUTS.map((layout) => {
        const active = value === layout.id
        return (
          <button
            key={layout.id}
            type="button"
            role="radio"
            aria-checked={active}
            className={`workspace-layout-card ${active ? 'active' : ''}`}
            onClick={() => onChange?.(layout.id)}
          >
            <span className="workspace-layout-card-top">
              <span className="workspace-layout-letter">{layout.letter}</span>
              <span className="workspace-layout-hint">{layout.hint}</span>
            </span>
            <span className="workspace-layout-name">{layout.label}</span>
            <span className="workspace-layout-desc">{layout.description}</span>
          </button>
        )
      })}
    </div>
  )
}
