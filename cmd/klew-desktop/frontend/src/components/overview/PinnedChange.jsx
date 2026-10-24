import { buildInspectKey } from '../../lib/matches'
import { requestInspectorTab } from '../../lib/inspectorTabRequest'

/**
 * Investigation Overview: the workload change that likely relates to the
 * incident, e.g. "payment-api changed 12m before the incident started:
 * rev 14 (image, env)". Opens that workload's Changes tab.
 */
export function PinnedChange({ pinned, onInspectKeyChange }) {
  if (!pinned?.text) return null
  const c = pinned.change || {}
  const open = () => {
    requestInspectorTab(c.kind, c.name, 'changes')
    onInspectKeyChange?.(buildInspectKey(c.kind, c.name, c.namespace))
  }
  return (
    <button
      type="button"
      className={`inv-pinned-change ${pinned.beforeIncident ? 'is-before-incident' : ''}`}
      onClick={open}
      title="Open this workload's Changes tab"
    >
      <span className="inv-pinned-change-icon" aria-hidden="true">↺</span>
      <span className="inv-pinned-change-text">{pinned.text}</span>
      <span className="inv-pinned-change-cta">Changes →</span>
    </button>
  )
}
