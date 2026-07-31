
export function SignalsPanel({
  signals,
  signalMode,
  onSignalModeChange,
  drillLocked = false,
  checklist,
  healthy,
  showConfidence,
  confidence,
  hypothesis,
}) {
  const maxSignal = Math.max(1, ...signals.map((g) => g.count || 1))

  return (
    <>
      <div className="signals-toolbar">
        <SignalModeToggle
          mode={signalMode}
          onChange={onSignalModeChange}
          locked={drillLocked}
        />
      </div>
      <div className="signal-bars">
        {signals.length > 0 ? signals.map((g, i) => (
          <SignalBar
            key={`${g.label}-${i}`}
            label={g.label}
            count={g.count || 1}
            max={maxSignal}
            severity={g.severity}
            healthy={healthy}
          />
        )) : (
          <div className="muted signal-empty">Collecting live signals…</div>
        )}
      </div>
      <div className="divider" />
      {hypothesis && !healthy && (
        <div className="signal-hypothesis">
          <span className="hypo-label">Hypothesis</span>
          <span className="hypo-text">{hypothesis}</span>
          {showConfidence && confidence != null && (
            <span className="hypo-conf">{Math.round(confidence * 100)}% confidence</span>
          )}
        </div>
      )}
      {hypothesis && !healthy && <div className="divider" />}
      <ul className="signal-checklist">
        {checklist.map((item, i) => (
          <li key={i} className={`check-item check-${item.level}`}>
            <CheckIcon level={item.level} />
            <span>{item.text}</span>
          </li>
        ))}
        {!checklist.length && (
          <li className="check-item check-muted muted">Awaiting correlation…</li>
        )}
      </ul>
    </>
  )
}

function SignalModeToggle({ mode, onChange, locked }) {
  return (
    <div className="signal-mode-toggle" role="group" aria-label="Signal scope">
      <button
        type="button"
        className={mode === 'focus' ? 'active' : ''}
        onClick={() => onChange?.('focus')}
        disabled={locked}
      >
        This workload
      </button>
      <button
        type="button"
        className={mode === 'all' ? 'active' : ''}
        onClick={() => onChange?.('all')}
        disabled={locked}
        title={locked ? 'Clear focus to view all matches' : undefined}
      >
        All matches
      </button>
    </div>
  )
}

function SignalBar({ label, count, max, severity, healthy }) {
  const width = Math.max(6, Math.round((count / max) * 100))
  const tone = healthy ? 'accent' : severityTone(severity)

  return (
    <div className="signal-bar-row">
      <span className="signal-bar-label">{label}</span>
      <div className="signal-bar-track">
        <div className={`signal-bar-fill tone-${tone}`} style={{ width: `${width}%` }} />
      </div>
      <span className="signal-bar-count">{count} {healthy ? 'events' : 'obs'}</span>
    </div>
  )
}

function CheckIcon({ level }) {
  if (level === 'crit') {
    return <span className="check-icon icon-crit" aria-hidden="true">!</span>
  }
  if (level === 'warn') {
    return <span className="check-icon icon-warn" aria-hidden="true">▲</span>
  }
  return <span className="check-icon icon-ok" aria-hidden="true">✓</span>
}

function severityTone(severity) {
  switch (severity) {
    case 'critical':
      return 'crit'
    case 'high':
    case 'warning':
      return 'warn'
    default:
      return 'accent'
  }
}
