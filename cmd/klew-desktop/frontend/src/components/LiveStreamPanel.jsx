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
 * Live tail — multipod container logs only.
 * Log / Event Patterns live on the Patterns nav page.
 */
export function LiveStreamPanel({
  evidence,
  snapshotPods,
  query = '',
  running = false,
  dropped = 0,
  updatedAt,
  lastEventAt,
  panelState,
  height,
  mode,
  search,
  selectedPods = [],
  follow,
  paused = false,
  streamFontSize = 12,
  streamDense = false,
  streamWrapLines = false,
  onModeChange,
  onSearchChange,
  onTogglePod,
  onSelectMatched,
  onSelectAllPods,
  onFollowChange,
  onPausedChange,
  onTogglePaused,
  onMinimize,
  onMaximize,
  onRestore,
  onClose,
  onOpen,
  onResizeStart,
}) {
  const scrollRef = useRef(null)
  const prevRowCount = useRef(0)
  const [filterOpen, setFilterOpen] = useState(false)
  /** Snapshot of groups at the moment the user paused the tail. */
  const [frozen, setFrozen] = useState(null)

  const { all, matched, other } = useMemo(
    () => collectStreamPods(evidence, snapshotPods, query),
    [evidence, snapshotPods, query],
  )

  const selected = Array.isArray(selectedPods) ? selectedPods : []
  const safeMode = mode === StreamMode.Patterns ? StreamMode.Logs : mode
  const filterKey = `${safeMode}|${search}|${selected.join(',')}`

  useEffect(() => {
    if (mode === StreamMode.Patterns) onModeChange?.(StreamMode.Logs)
  }, [mode, onModeChange])

  const streamMeta = useMemo(
    () => ({
      updatedAt,
      lastEventAt,
      pods: selected,
    }),
    [updatedAt, lastEventAt, selected],
  )

  const liveStream = useMemo(() => {
    if (!running) {
      return { groups: [], rowCount: 0 }
    }
    return buildStreamGroups(evidence, safeMode, search, streamMeta)
  }, [evidence, safeMode, search, streamMeta, running, filterKey])

  // Capture freeze when pausing; clear on resume / investigation stop.
  // Filter changes while paused refresh the snapshot from current live lines.
  const wasPaused = useRef(false)
  useEffect(() => {
    if (!running) {
      setFrozen(null)
      wasPaused.current = false
      if (paused) onPausedChange?.(false)
      return
    }
    if (paused) {
      const justPaused = !wasPaused.current
      wasPaused.current = true
      setFrozen((prev) => {
        if (justPaused || !prev) {
          return { groups: liveStream.groups, rowCount: liveStream.rowCount, filterKey }
        }
        // Same filters → keep frozen lines (ignore new arrivals).
        if (prev.filterKey === filterKey) return prev
        return { groups: liveStream.groups, rowCount: liveStream.rowCount, filterKey }
      })
    } else {
      wasPaused.current = false
      setFrozen(null)
    }
  }, [paused, running, liveStream, filterKey, onPausedChange])

  const groups = paused && frozen ? frozen.groups : liveStream.groups
  const rowCount = paused && frozen ? frozen.rowCount : liveStream.rowCount

  const filtersActive = hasActiveStreamFilters(safeMode, selected, matched, search)

  useEffect(() => {
    const el = scrollRef.current
    if (!el || panelState === PANEL_MINIMIZED || panelState === PANEL_CLOSED) {
      return
    }
    if (paused) return
    if (follow || rowCount > prevRowCount.current) {
      el.scrollTop = el.scrollHeight
    }
    prevRowCount.current = rowCount
  }, [groups, rowCount, follow, panelState, paused])

  if (panelState === PANEL_CLOSED) {
    return (
      <button type="button" className="stream-reopen" onClick={() => onOpen(PANEL_NORMAL)}>
        <svg className="stream-doc-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden="true">
          <path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z" />
          <path d="M14 2v6h6" />
        </svg>
        Live tail
        {!running && <span className="stream-live-badge idle">IDLE</span>}
        {running && !paused && <span className="stream-live-badge">LIVE</span>}
        {running && paused && <span className="stream-live-badge paused">PAUSED</span>}
        {running && rowCount > 0 && <span className="stream-count">{rowCount}</span>}
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
        expanded ? 'expanded' : 'collapsed',
        maximized ? 'maximized' : '',
        streamDense ? 'stream-dense' : '',
        streamWrapLines ? 'stream-wrap' : '',
      ].filter(Boolean).join(' ')}
      style={{
        '--stream-font-size': `${streamFontSize}px`,
        ...(panelState === PANEL_NORMAL ? { '--stream-h': `${height}px` } : null),
      }}
    >
      {panelState === PANEL_NORMAL && (
        <div
          className="stream-resize-handle"
          role="separator"
          aria-orientation="horizontal"
          aria-label="Resize live log tail"
          onMouseDown={onResizeStart}
        />
      )}

      <header className="stream-header">
        <div className="stream-title">
          <svg className="stream-doc-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden="true">
            <path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z" />
            <path d="M14 2v6h6" />
          </svg>
          <span>Live tail</span>
          {!running && <span className="stream-live-badge idle">IDLE</span>}
          {running && !paused && <span className="stream-live-badge">LIVE</span>}
          {running && paused && <span className="stream-live-badge paused">PAUSED</span>}
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
          {running && rowCount > 0 && <span className="stream-count">{rowCount}</span>}
        </div>

        <div className="stream-toolbar">
          {expanded && running && (
            <>
              <button
                type="button"
                className={`stream-icon-btn ${paused ? 'paused' : 'live'}`}
                onClick={() => onTogglePaused?.()}
                title={paused ? 'Resume live tail' : 'Pause live tail'}
                aria-label={paused ? 'Resume live tail' : 'Pause live tail'}
                aria-pressed={paused}
              >
                {paused ? <IconPlay /> : <IconPause />}
              </button>
              {!paused && (
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
        </div>
      </header>

      {expanded && (
        <div className="stream-body">
          <div className="stream-search-row">
            <input
              className="stream-search"
              placeholder={
                !running
                  ? 'Start an investigation to begin live log tail'
                  : 'Regex or text filter on log lines…'
              }
              value={search}
              onChange={(e) => onSearchChange(e.target.value)}
              onFocus={() => running && onFollowChange(false)}
              disabled={!running}
            />
            <StreamFilterMenu
              open={filterOpen}
              onToggle={setFilterOpen}
              active={filtersActive}
              disabled={!running}
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
                disabled={!running}
                onClick={() => onModeChange?.(m.id)}
              >
                {m.label}
              </button>
            ))}
          </div>
          {running && (
            <p className="stream-live-note">
              {selected.length
                ? `Multipod container logs (${selected.length} pod${selected.length === 1 ? '' : 's'} selected). Open Patterns for log and event templates.`
                : 'Multipod container logs (all pods). Open Patterns for log and event templates.'}
            </p>
          )}
          <div className="stream-scroll" ref={scrollRef} key={filterKey}>
            {!running && (
              <div className="empty-stream">
                <strong>Live log tail</strong>
                <span>
                  Investigate a workload to stream container logs from pods matching your search.
                  Nothing is archived — the panel clears when the investigation stops.
                </span>
              </div>
            )}
            {running && !groups.length && (
              <div className="empty-stream">
                {filtersActive || search.trim()
                  ? 'No log lines match the current filters.'
                  : paused
                    ? 'Paused — resume to continue the live tail.'
                    : 'Tailing container logs…'}
              </div>
            )}
            {running && groups.map((group) => (
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
        </div>
      )}
    </section>
  )
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
