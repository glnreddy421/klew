import { InvestigateIcon } from '../InvestigateIcon.jsx'
import { NamespacePopover } from './ClusterNamespacePopover.jsx'
import { TargetPopover } from './TargetPopover.jsx'
import { NAV_ITEM_RESOURCES, NAV_ITEMS_INVESTIGATION_INLINE } from '../../lib/constants.js'
import { targetLabel } from '../../lib/investigateTarget.js'
import { useHeaderSlotRef } from '../../context/HeaderSlotContext.jsx'

export const MODE_WATCH = 'watch'
export const MODE_INVESTIGATE = 'investigate'

/**
 * One bar on the Resources surface. Watch browses; Investigate picks one
 * namespace, optionally narrows to resources in it, and runs an
 * investigation whose results show as tabs on the same screen.
 */
export function InvestigateBar({
  mode,
  onModeChange,
  cluster,
  scope,
  onScopeChange,
  scopeLocked = false,
  investigationNs = '',
  groups = [],
  unavailable = [],
  matchesLoading = false,
  matchesError = '',
  targets = [],
  onTargetsChange,
  includeLogs = true,
  onIncludeLogsChange,
  running = false,
  starting = false,
  changed = false,
  canRun = false,
  onRun,
  onStop,
}) {
  const investigating = mode === MODE_INVESTIGATE
  const busy = running || starting
  const namespace = scope?.namespace || ''
  const contextSlot = useHeaderSlotRef('context')
  const toolsSlot = useHeaderSlotRef('tools')

  return (
    <div className={`investigate-bar ${investigating ? 'is-investigating' : ''}`} role="toolbar" aria-label="Watch or investigate">
      <div className="investigate-bar-lens-row">
        <div className="scope-lens-toggle investigate-bar-mode" role="group" aria-label="Mode">
          <button
            type="button"
            className={`scope-lens-btn ${!investigating ? 'is-active' : ''}`}
            aria-pressed={!investigating}
            onClick={() => onModeChange?.(MODE_WATCH)}
            title="Browse resources across namespaces"
          >
            Watch
          </button>
          <button
            type="button"
            className={`scope-lens-btn ${investigating ? 'is-active' : ''}`}
            aria-pressed={investigating}
            onClick={() => onModeChange?.(MODE_INVESTIGATE)}
            title="Investigate one namespace or object (⌘K)"
          >
            <InvestigateIcon
              size={13}
              className={`investigate-mode-glyph ${busy ? 'is-busy' : ''}`}
            />
            Investigate
          </button>
        </div>
      </div>

      {!investigating && busy && (
        <button type="button" className="investigate-bar-chip" onClick={() => onModeChange?.(MODE_INVESTIGATE)}>
          <InvestigateIcon size={13} className="investigate-mode-glyph is-busy" />
          Investigating {investigationNs || namespace} · view results
        </button>
      )}

      {investigating && (
        <form
          className="investigate-bar-form"
          onSubmit={(e) => {
            e.preventDefault()
            if (canRun) onRun?.()
          }}
        >
          <div className="ib-query" role="group" aria-label="What to investigate">
            <QuerySegment label="Namespace" icon={<NamespaceGlyph />} locked={scopeLocked}>
              <NamespacePopover
                cluster={cluster}
                scope={scope}
                onScopeChange={onScopeChange}
                disabled={starting}
                locked={scopeLocked}
                lockedNamespace={investigationNs}
                compact
                variant="investigate"
              />
            </QuerySegment>
            <QuerySegment label="Target" icon={<TargetGlyph />}>
              <TargetPopover
                groups={groups}
                unavailable={unavailable}
                selected={targets}
                onApply={onTargetsChange}
                disabled={starting}
                loading={matchesLoading}
                error={matchesError}
              />
            </QuerySegment>
            <button
              type="button"
              role="switch"
              aria-checked={includeLogs}
              className={`ib-seg ib-logs ${includeLogs ? 'is-on' : ''}`}
              onClick={() => onIncludeLogsChange?.(!includeLogs)}
              disabled={starting}
              title="Read recent container logs. Off is faster and keeps log text out of the investigation."
            >
              <span className="ib-seg-icon" aria-hidden="true"><LogsGlyph /></span>
              <span className="ib-seg-text">
                <span className="ib-seg-label">Logs</span>
                <span className="ib-seg-value">{includeLogs ? 'Included' : 'Off'}</span>
              </span>
              <span className="ib-switch" aria-hidden="true"><span /></span>
            </button>
          </div>
          <div className="investigate-bar-actions">
            <button
              type="submit"
              className="investigate-bar-run"
              disabled={!canRun}
              title={runTitle({ namespace, running, starting, changed, targets })}
            >
              {starting ? 'Starting…' : running ? (changed ? 'Re-run' : 'Running') : 'Investigate'}
            </button>
            {busy && (
              <button type="button" className="scope-toolbar-btn investigate-bar-stop" onClick={onStop} disabled={starting}>
                Stop
              </button>
            )}
          </div>
        </form>
      )}

      {/* Filled by the page header and the resource toolbar (HeaderSlot). */}
      <div className="investigate-bar-slot investigate-bar-slot-context" ref={contextSlot} />
      <div className="investigate-bar-slot investigate-bar-slot-tools" ref={toolsSlot} />
    </div>
  )
}

/**
 * One field of the query bar: icon and label beside a picker. Clicking
 * anywhere in the field opens the picker.
 */
function QuerySegment({ label, icon, locked = false, children }) {
  return (
    <div
      className={`ib-seg ${locked ? 'is-locked' : ''}`}
      onClick={(e) => {
        if (e.target.closest('button, input, [role="dialog"], [role="listbox"]')) return
        e.currentTarget.querySelector('button')?.click()
      }}
    >
      <span className="ib-seg-icon" aria-hidden="true">{icon}</span>
      <span className="ib-seg-text">
        <span className="ib-seg-label">{label}</span>
        <span className="ib-seg-picker">{children}</span>
      </span>
    </div>
  )
}

function NamespaceGlyph() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round">
      <rect x="2" y="2.75" width="12" height="10.5" rx="2" strokeDasharray="2.2 1.6" />
      <rect x="5" y="5.75" width="6" height="4.5" rx="1" />
    </svg>
  )
}

function TargetGlyph() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.5">
      <circle cx="8" cy="8" r="5.5" />
      <circle cx="8" cy="8" r="2.25" />
      <circle cx="8" cy="8" r=".6" fill="currentColor" />
    </svg>
  )
}

function LogsGlyph() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
      <path d="M3 4.5h10M3 8h7M3 11.5h8.5" />
    </svg>
  )
}

function runTitle({ namespace, running, starting, changed, targets }) {
  if (!namespace) return 'Pick a namespace to investigate'
  if (starting) return 'Starting investigation…'
  if (running && !changed) return 'Already investigating this target; change the namespace, target or Logs to re-run'
  return `Investigate ${targetLabel(namespace, targets)}`
}

/** Inline result tabs shown under the bar while an investigation runs. */
export function InvestigationTabs({ tab, onSelect, counts = {} }) {
  const items = [NAV_ITEM_RESOURCES, ...NAV_ITEMS_INVESTIGATION_INLINE]
  return (
    <nav className="investigation-tabs" aria-label="Investigation results">
      {items.map((item) => (
        <button
          key={item.id}
          type="button"
          className={`investigation-tab ${tab === item.id ? 'is-active' : ''}`}
          aria-current={tab === item.id ? 'page' : undefined}
          onClick={() => onSelect?.(item.id)}
          title={item.hint}
        >
          {item.label}
          {counts[item.id] > 0 && <span className="investigation-tab-count">{counts[item.id]}</span>}
        </button>
      ))}
    </nav>
  )
}
