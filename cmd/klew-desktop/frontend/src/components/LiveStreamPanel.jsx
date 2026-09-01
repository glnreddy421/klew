import { useEffect, useMemo, useRef, useState } from 'react'
import {
  buildStreamGroups,
  collectStreamPods,
  hasActiveStreamFilters,
  StreamMode,
  STREAM_MODES,
} from '../lib/streamView'
import {
  PANEL_CLOSED,
  PANEL_MAXIMIZED,
  PANEL_MINIMIZED,
  PANEL_NORMAL,
} from '../hooks/useStreamPanel'
import { StreamFilterMenu } from './StreamFilterMenu'
import { StreamSourceIcon } from './KindIcon'

/**
 * Container logs — on-demand multipod tail from investigation scope.
 * User selects pods and optional line filter before gathering.
 */
export function LiveStreamPanel({
  evidence,
  snapshotPods,
  query = '',
  running = false,
  logTailEngaged = false,
  logTailPaused = false,
  tailPods = [],
  dropped = 0,
  updatedAt,
  lastEventAt,
  panelState,
  height,
  mode,
  search,
  selectedPods = [],
  follow,
  streamFontSize = 12,
  streamDense = false,
  streamWrapLines = false,
  onModeChange,
  onSearchChange,
  onTogglePod,
  onSelectMatched,
  onSelectAllPods,
  onFollowChange,
  onToggleLogTailPause,
  onMinimize,
  onMaximize,
  onRestore,
  onClose,
  onOpen,
  onResizeStart,
  onStartGather,
  onStopGather,
  onClearLogs,
  gatherBusy = false,
  gatherError = '',
  embedded = false,
}) {
  const scrollRef = useRef(null)
  const prevRowCount = useRef(0)
  const [filterOpen, setFilterOpen] = useState(false)
  /** Snapshot of groups at the moment the user paused the tail. */
  const [frozen, setFrozen] = useState(null)
  const [requestSearch, setRequestSearch] = useState('')
  const [requestPods, setRequestPods] = useState(() => new Set())
  /** Last gather request — restored when clearing or stopping tail. */
  const lastRequestRef = useRef({ pods: null, search: '' })
  const prevRunningRef = useRef(false)
  const prevTailRef = useRef(false)
  const logTailStreaming = logTailEngaged && !logTailPaused

  const scopePods = useMemo(
    () => (snapshotPods || []).filter((p) => p?.name).sort((a, b) => a.name.localeCompare(b.name)),
    [snapshotPods],
  )

  // Default all pods selected once per investigation — never clobber manual edits on refresh.
  useEffect(() => {
    if (running && !prevRunningRef.current && scopePods.length > 0) {
      setRequestPods(new Set(scopePods.map((p) => p.name)))
      setRequestSearch('')
      lastRequestRef.current = { pods: null, search: '' }
    }
    prevRunningRef.current = running
    if (!running) {
      prevRunningRef.current = false
      prevTailRef.current = false
      setRequestSearch('')
      setRequestPods(new Set())
      lastRequestRef.current = { pods: null, search: '' }
    }
  }, [running, scopePods])

  // Drop pods that left scope; do not re-select everything on snapshot refresh.
  useEffect(() => {
    if (!running || logTailEngaged || !scopePods.length) return
    const scopeNames = new Set(scopePods.map((p) => p.name))
    setRequestPods((prev) => {
      const next = new Set([...prev].filter((n) => scopeNames.has(n)))
      return next.size ? next : prev
    })
  }, [scopePods, running, logTailEngaged])

  // Restore last gather selection when leaving the live tail view.
  useEffect(() => {
    if (prevTailRef.current && !logTailEngaged && running) {
      const saved = lastRequestRef.current
      if (saved.pods && saved.pods.size > 0) {
        setRequestPods(new Set(saved.pods))
        setRequestSearch(saved.search || '')
      }
    }
    prevTailRef.current = logTailEngaged
  }, [logTailEngaged, running])

  const { all, matched, other } = useMemo(
    () => collectStreamPods(evidence, snapshotPods, query),
    [evidence, snapshotPods, query],
  )

  const selected = Array.isArray(selectedPods) ? selectedPods : []
  const activeTailPods = logTailEngaged && Array.isArray(tailPods) && tailPods.length ? tailPods : selected
  const safeMode = mode === StreamMode.Patterns ? StreamMode.Logs : mode
  const filterKey = `${safeMode}|${search}|${activeTailPods.join(',')}`

  useEffect(() => {
    if (mode === StreamMode.Patterns) onModeChange?.(StreamMode.Logs)
  }, [mode, onModeChange])

  const streamMeta = useMemo(
    () => ({
      updatedAt,
      lastEventAt,
      pods: activeTailPods,
    }),
    [updatedAt, lastEventAt, activeTailPods],
  )

  const liveStream = useMemo(() => {
    if (!running || !logTailEngaged) {
      return { groups: [], rowCount: 0 }
    }
    return buildStreamGroups(evidence, safeMode, search, streamMeta)
  }, [evidence, safeMode, search, streamMeta, running, logTailEngaged, filterKey])

  function toggleRequestPod(name) {
    setRequestPods((prev) => {
      const next = new Set(prev)
      if (next.has(name)) next.delete(name)
      else next.add(name)
      return next
    })
  }

  function handleGather() {
    if (!scopePods.length || requestPods.size === 0) return
    lastRequestRef.current = {
      pods: new Set(requestPods),
      search: requestSearch.trim(),
    }
    onStartGather?.({
      podNames: [...requestPods],
      lineSearch: requestSearch.trim(),
    })
  }

  // Capture freeze when pausing; clear on resume / investigation stop.
  // Filter changes while paused refresh the snapshot from current live lines.
  const wasPaused = useRef(false)
  useEffect(() => {
    if (!running) {
      setFrozen(null)
      wasPaused.current = false
      return
    }
    if (logTailPaused) {
      const justPaused = !wasPaused.current
      wasPaused.current = true
      setFrozen((prev) => {
        if (justPaused || !prev) {
          return { groups: liveStream.groups, rowCount: liveStream.rowCount, filterKey }
        }
        if (prev.filterKey === filterKey) return prev
        return { groups: liveStream.groups, rowCount: liveStream.rowCount, filterKey }
      })
    } else {
      wasPaused.current = false
      setFrozen(null)
    }
  }, [logTailPaused, running, liveStream, filterKey])

  const groups = logTailPaused && frozen ? frozen.groups : liveStream.groups
  const rowCount = logTailPaused && frozen ? frozen.rowCount : liveStream.rowCount

  const filtersActive = hasActiveStreamFilters(safeMode, selected, matched, search)

  useEffect(() => {
    const el = scrollRef.current
    if (!el || panelState === PANEL_MINIMIZED || panelState === PANEL_CLOSED) {
      return
    }
    if (logTailPaused) return
    if (follow || rowCount > prevRowCount.current) {
      el.scrollTop = el.scrollHeight
    }
    prevRowCount.current = rowCount
  }, [groups, rowCount, follow, panelState, logTailPaused])

  if (!embedded && panelState === PANEL_CLOSED) {
    return (
      <button type="button" className="stream-reopen" onClick={() => onOpen(PANEL_NORMAL)}>
        <svg className="stream-doc-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden="true">
          <path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z" />
          <path d="M14 2v6h6" />
        </svg>
        Container logs
        {!running && <span className="stream-live-badge idle">IDLE</span>}
        {running && !logTailEngaged && <span className="stream-live-badge idle">READY</span>}
        {running && logTailStreaming && <span className="stream-live-badge">LIVE</span>}
        {running && logTailEngaged && logTailPaused && <span className="stream-live-badge paused">PAUSED</span>}
        {running && logTailEngaged && rowCount > 0 && <span className="stream-count">{rowCount}</span>}
      </button>
    )
  }

  const expanded = panelState === PANEL_NORMAL || panelState === PANEL_MAXIMIZED
  const maximized = panelState === PANEL_MAXIMIZED
  const crispChips = []
  if (search.trim()) crispChips.push({ key: 'search', label: `/${search.trim()}/`, mono: true })
  if (dropped > 0) crispChips.push({ key: 'dropped', label: `−${dropped}`, warn: true })

  return (
    <section
      className={[
        'stream-panel',
        embedded ? 'stream-panel-embedded' : '',
        expanded ? 'expanded' : 'collapsed',
        maximized && !embedded ? 'maximized' : '',
        streamDense ? 'stream-dense' : '',
        streamWrapLines ? 'stream-wrap' : '',
      ].filter(Boolean).join(' ')}
      style={{
        '--stream-font-size': `${streamFontSize}px`,
        ...((panelState === PANEL_NORMAL || embedded) ? { '--stream-h': `${height}px` } : null),
      }}
    >
      {(panelState === PANEL_NORMAL || embedded) && !maximized && (
        <div
          className="stream-resize-handle"
          role="separator"
          aria-orientation="horizontal"
          aria-label="Resize live log tail"
          onMouseDown={onResizeStart}
        />
      )}

      <header
        className={`stream-header ${expanded ? '' : 'stream-header-expandable'}`}
        onClick={!expanded ? () => onOpen?.(PANEL_NORMAL) : undefined}
        onKeyDown={!expanded ? (e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault()
            onOpen?.(PANEL_NORMAL)
          }
        } : undefined}
        role={!expanded ? 'button' : undefined}
        tabIndex={!expanded ? 0 : undefined}
        aria-expanded={expanded}
      >
        <div className="stream-title">
          <svg className="stream-doc-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden="true">
            <path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z" />
            <path d="M14 2v6h6" />
          </svg>
          <span>Container logs</span>
          {!embedded && !running && <span className="stream-live-badge idle">IDLE</span>}
          {!embedded && running && !logTailEngaged && <span className="stream-live-badge idle">READY</span>}
          {!embedded && running && logTailStreaming && <span className="stream-live-badge">LIVE</span>}
          {!embedded && running && logTailEngaged && logTailPaused && <span className="stream-live-badge paused">PAUSED</span>}
          {crispChips.length > 0 && (
            <span className="stream-chips">
              {crispChips.map((chip) => (
                <span
                  key={chip.key}
                  className={[
                    'stream-chip',
                    chip.mono ? 'mono' : '',
                    chip.warn ? 'warn' : '',
                  ].filter(Boolean).join(' ')}
                >
                  {chip.label}
                </span>
              ))}
            </span>
          )}
          {running && logTailEngaged && rowCount > 0 && <span className="stream-count">{rowCount}</span>}
        </div>

        <div className="stream-toolbar" onClick={(e) => e.stopPropagation()}>
          {expanded && running && logTailEngaged && (
            <>
              <button
                type="button"
                className="btn btn-ghost btn-sm stream-stop-btn"
                onClick={() => onStopGather?.()}
                title="Stop gathering logs"
              >
                Stop
              </button>
              <button
                type="button"
                className="stream-icon-btn"
                onClick={() => onClearLogs?.()}
                title="Clear all log lines"
                aria-label="Clear all log lines"
              >
                <IconClear />
              </button>
              <button
                type="button"
                className={`stream-icon-btn ${logTailPaused ? 'paused' : 'live'}`}
                onClick={() => onToggleLogTailPause?.()}
                title={logTailPaused ? 'Resume log gather (reopens GetLogs streams)' : 'Pause log gather (closes GetLogs streams)'}
                aria-label={logTailPaused ? 'Resume log gather' : 'Pause log gather'}
                aria-pressed={logTailPaused}
              >
                {logTailPaused ? <IconPlay /> : <IconPause />}
              </button>
              {logTailStreaming && (
                <button
                  type="button"
                  className={`stream-icon-btn ${follow ? 'active' : ''}`}
                  onClick={() => onFollowChange(!follow)}
                  title={follow ? 'Following new lines — click to hold scroll' : 'Scroll held — click to follow again'}
                  aria-label={follow ? 'Hold scroll' : 'Follow new lines'}
                  aria-pressed={follow}
                >
                  {follow ? <IconFollowOn /> : <IconFollowOff />}
                </button>
              )}
            </>
          )}

          {!embedded && (
          <div className="stream-window-controls">
            {maximized ? (
              <WindowBtn label="Restore panel" onClick={onRestore}>
                <IconRestore />
              </WindowBtn>
            ) : (
              <WindowBtn label="Maximize panel" onClick={onMaximize}>
                <IconMaximize />
              </WindowBtn>
            )}
            <WindowBtn
              label={expanded ? 'Minimize panel' : 'Expand panel'}
              onClick={expanded ? onMinimize : () => onOpen(PANEL_NORMAL)}
            >
              {expanded ? <IconMinimize /> : <IconExpand />}
            </WindowBtn>
            <WindowBtn label="Close panel" onClick={onClose}>
              <IconClose />
            </WindowBtn>
          </div>
          )}
        </div>
      </header>

      {(expanded || embedded) && (
        <div className="stream-body">
          {running && !logTailEngaged ? (
            <div className="log-request">
              <div className="log-request-intro">
                <strong>What logs do you need?</strong>
                <p className="muted">
                  Pods from your investigation scope — Deployments, StatefulSets, DaemonSets, Jobs, and any other
                  workload that matched. Select pods, optionally filter by text, then gather.
                </p>
              </div>
              <label className="log-request-field">
                <span className="log-request-label">Search for text in log lines</span>
                <input
                  className="stream-search"
                  placeholder="Optional — e.g. error, timeout, OOM"
                  value={requestSearch}
                  onChange={(e) => setRequestSearch(e.target.value)}
                />
              </label>
              <div className="log-request-toolbar">
                <span className="log-request-count">
                  {scopePods.length
                    ? `${requestPods.size} of ${scopePods.length} pod${scopePods.length === 1 ? '' : 's'} selected`
                    : 'No pods in scope yet'}
                </span>
                <div className="log-request-actions">
                  <button
                    type="button"
                    className="btn btn-ghost btn-sm"
                    disabled={!scopePods.length}
                    onClick={() => setRequestPods(new Set(scopePods.map((p) => p.name)))}
                  >
                    Select all
                  </button>
                  <button
                    type="button"
                    className="btn btn-ghost btn-sm"
                    disabled={!scopePods.length}
                    onClick={() => setRequestPods(new Set())}
                  >
                    Clear
                  </button>
                </div>
              </div>
              <ul className="log-request-pods">
                {scopePods.map((pod) => (
                  <li key={pod.name}>
                    <label className="log-request-pod">
                      <input
                        type="checkbox"
                        checked={requestPods.has(pod.name)}
                        onChange={() => toggleRequestPod(pod.name)}
                      />
                      <span className="log-request-pod-name mono">{pod.name}</span>
                      <span className="log-request-pod-owner muted">{podOwnerLabel(pod)}</span>
                    </label>
                  </li>
                ))}
                {!scopePods.length && (
                  <li className="muted log-request-empty">Waiting for pods in investigation snapshot…</li>
                )}
              </ul>
              {gatherError && <p className="log-request-error">{gatherError}</p>}
              <div className="log-request-footer">
                <button
                  type="button"
                  className="btn btn-primary"
                  disabled={!scopePods.length || requestPods.size === 0 || gatherBusy}
                  onClick={handleGather}
                >
                  {gatherBusy ? 'Starting…' : 'Gather logs'}
                </button>
              </div>
            </div>
          ) : (
            <>
          <div className="stream-search-row">
            <input
              className="stream-search"
              placeholder={
                !running
                  ? 'Start an investigation to request container logs'
                  : 'Regex or text filter on log lines…'
              }
              value={search}
              onChange={(e) => onSearchChange(e.target.value)}
              onFocus={() => running && onFollowChange(false)}
              disabled={!running || !logTailEngaged}
            />
            <StreamFilterMenu
              open={filterOpen}
              onToggle={setFilterOpen}
              active={filtersActive}
              disabled={!running || !logTailEngaged}
              selectedPods={selected}
              matchedPods={matched}
              otherPods={other}
              onTogglePod={(name) => onTogglePod?.(name, all)}
              onSelectMatched={() => onSelectMatched?.(matched)}
              onSelectAllPods={() => onSelectAllPods?.()}
              onReset={() => {
                onSelectAllPods?.()
                onSearchChange?.('')
                setFilterOpen(false)
              }}
            />
          </div>
          <div className="stream-mode-tabs" role="tablist" aria-label="Live tail mode">
            {STREAM_MODES.map((m) => (
              <button
                key={m.id}
                type="button"
                role="tab"
                aria-selected={safeMode === m.id}
                className={`stream-mode-btn ${safeMode === m.id ? 'active' : ''}`}
                title={m.hint}
                disabled={!running || !logTailEngaged}
                onClick={() => onModeChange?.(m.id)}
              >
                {m.label}
              </button>
            ))}
          </div>
          {running && logTailEngaged && (
            <p className="stream-live-note">
              {logTailPaused
                ? `Gather paused — ${activeTailPods.length} pod${activeTailPods.length === 1 ? '' : 's'} selected. Resume to reopen log streams.`
                : activeTailPods.length
                  ? `Gathering from ${activeTailPods.length} pod${activeTailPods.length === 1 ? '' : 's'}. Open Patterns for log and event templates.`
                  : 'Gathering from all pods in scope. Open Patterns for log and event templates.'}
            </p>
          )}
          <div className="stream-scroll" ref={scrollRef} key={filterKey}>
            {!running && (
              <div className="empty-stream">
                <strong>Container logs</strong>
                <span>
                  Investigate a workload, then choose which pods to gather logs from.
                  Nothing is archived — the panel clears when the investigation stops.
                </span>
              </div>
            )}
            {running && logTailEngaged && !groups.length && (
              <div className="empty-stream">
                {filtersActive || search.trim()
                  ? 'No log lines match the current filters.'
                  : logTailPaused
                    ? 'Paused — GetLogs streams closed. Resume to continue gathering.'
                    : 'Gathering container logs…'}
              </div>
            )}
            {running && logTailEngaged && groups.map((group) => (
              <div key={group.key} className="stream-group">
                <div className="stream-group-header">
                  <StreamSourceIcon source={group.kind} />
                  <span className="stream-group-source">{group.key}</span>
                </div>
                {group.rows.map((row) => (
                  <div
                    key={row.id}
                    className={[
                      'stream-row',
                      `sev-${row.severity}`,
                      row.fresh ? 'fresh' : '',
                    ].filter(Boolean).join(' ')}
                  >
                    <span className="stream-col time mono">{row.time}</span>
                    <span className="stream-col type">{row.type}</span>
                    <span className="stream-col object mono" title={row.object}>{row.object}</span>
                    <span className="stream-col marker">{severityMark(row.severity)}</span>
                    <span className="stream-col message" title={row.message}>{row.message}</span>
                  </div>
                ))}
              </div>
            ))}
          </div>
            </>
          )}
        </div>
      )}
    </section>
  )
}

function podOwnerLabel(pod) {
  const raw = pod?.ownerRefs ?? pod?.OwnerRefs ?? []
  const refs = Array.isArray(raw) ? raw : []
  const kinds = ['Deployment', 'StatefulSet', 'DaemonSet', 'Job', 'ReplicaSet', 'CronJob']
  const root = refs.find((r) => kinds.includes(r?.kind || r?.Kind))
  if (root) {
    const kind = root.kind || root.Kind
    const name = root.name || root.Name
    return `${kind}/${name}`
  }
  return 'Pod'
}

function WindowBtn({ label, onClick, children }) {
  return (
    <button type="button" className="stream-window-btn" aria-label={label} title={label} onClick={onClick}>
      {children}
    </button>
  )
}

function IconMaximize() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <rect x="3.5" y="3.5" width="9" height="9" rx="1" />
    </svg>
  )
}

function IconRestore() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <rect x="5" y="2.5" width="8.5" height="8.5" rx="1" />
      <path d="M2.5 5v8.5H11" />
    </svg>
  )
}

function IconMinimize() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <path d="M3.5 8h9" />
    </svg>
  )
}

function IconExpand() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <path d="M8 11V5M5.5 7.5L8 5l2.5 2.5" />
    </svg>
  )
}

function IconClose() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <path d="M4.5 4.5l7 7M11.5 4.5l-7 7" />
    </svg>
  )
}

function IconClear() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <path d="M3.5 4.5h9M5.5 4.5V3.8a1 1 0 011-1h3a1 1 0 011 1v.7" />
      <path d="M5.5 6.5v5M8 6.5v5M10.5 6.5v5" />
      <path d="M5 12.5h6a1 1 0 001-1V5H4v6.5a1 1 0 001 1z" />
    </svg>
  )
}

function IconPause() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="currentColor" aria-hidden="true">
      <rect x="3.5" y="3" width="3" height="10" rx="0.8" />
      <rect x="9.5" y="3" width="3" height="10" rx="0.8" />
    </svg>
  )
}

function IconPlay() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="currentColor" aria-hidden="true">
      <path d="M5 3.2v9.6L13 8 5 3.2z" />
    </svg>
  )
}

function IconFollowOn() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <path d="M8 3v7.5M5.5 8L8 10.5 10.5 8" />
      <path d="M3.5 13h9" />
    </svg>
  )
}

function IconFollowOff() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <path d="M8 3v6" />
      <path d="M3.5 13h9" />
      <path d="M5 11.5h6" opacity="0.45" />
    </svg>
  )
}

function severityMark(severity) {
  switch (severity) {
    case 'critical':
      return '●'
    case 'high':
      return '▲'
    case 'warning':
      return '◆'
    default:
      return '·'
  }
}
