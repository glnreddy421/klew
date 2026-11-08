import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { StreamLiveBadge } from './StreamLiveBadge'
import { TerminalFindBar } from './TerminalFindBar'
import { buildLogFindMatches, findSubstringRanges } from '../lib/logFind'
import {
  describeSources,
  filterLines,
  formatLineTime,
  podColor,
  podTargets,
  sourceKey,
  streamSources,
} from '../lib/logStream'
import { isEditableTarget } from '../lib/keyboard'

/**
 * Container logs — a stern-style merged stream of the pods the user picked.
 * Runs with or without an investigation (see hooks/useLogStream).
 */
export function LiveStreamPanel({
  logStream,
  running = false,
  investigationPods = [],
  investigationNamespace = '',
  tailLines = 0,
  follow: controlledFollow,
  defaultFollow = true,
  streamFontSize = 12,
  streamDense = false,
  streamWrapLines = false,
  onFollowChange: controlledOnFollowChange,
  dockControls = null,
  /** Unified bottom panel: title row lives on ConsoleDock; only toolbar + body here. */
  ideDock = false,
  /** When inside ConsoleDock, false when another tab is selected (keeps filter state, skips drawing rows). */
  activeTab = true,
  /** Open find with this query once (e.g. jump from Patterns). */
  logsFindFromPatterns = null,
  onLogsFindFromPatternsHandled,
}) {
  const { status, lines, busy, error } = logStream
  const scrollRef = useRef(null)
  const [muted, setMuted] = useState(() => new Set())
  const [showTime, setShowTime] = useState(true)
  const [internalFollow, setInternalFollow] = useState(() => Boolean(defaultFollow))
  const follow = controlledFollow !== undefined ? controlledFollow : internalFollow
  const onFollowChange = useCallback((next) => {
    setInternalFollow(Boolean(next))
    controlledOnFollowChange?.(Boolean(next))
  }, [controlledOnFollowChange])
  const [findOpen, setFindOpen] = useState(false)
  const [findQuery, setFindQuery] = useState('')
  const [findIndex, setFindIndex] = useState(-1)

  const active = Boolean(status.active)
  const paused = Boolean(status.paused)
  const streaming = active && !paused

  // A new stream starts with every source visible.
  const sourcesKey = (status.sources || []).map((s) => `${s.namespace}/${s.label}`).join('|')
  useEffect(() => {
    setMuted(new Set())
  }, [sourcesKey])

  const sources = useMemo(() => streamSources(lines), [lines])
  const visible = useMemo(
    () => filterLines(lines, { muted }),
    [lines, muted],
  )

  const findMatches = useMemo(
    () => buildLogFindMatches(visible, (l) => l.text, findQuery.trim()),
    [visible, findQuery],
  )

  const findCount = findMatches.length

  useEffect(() => {
    if (findCount === 0 && findIndex !== -1) setFindIndex(-1)
    else if (findCount > 0 && findIndex >= findCount) setFindIndex(findCount - 1)
  }, [findCount, findIndex])

  useEffect(() => {
    if (!findOpen || !findQuery.trim()) return
    if (findCount === 0) return
    if (findIndex < 0) setFindIndex(0)
  }, [findOpen, findQuery, findCount, findIndex])

  const activeMatch = findIndex >= 0 && findIndex < findCount ? findMatches[findIndex] : null

  useEffect(() => {
    const el = scrollRef.current
    if (!el || !activeMatch) return
    const rowEl = el.querySelector(`[data-log-seq="${activeMatch.row.seq}"]`)
    rowEl?.scrollIntoView?.({ block: 'nearest', behavior: 'smooth' })
  }, [activeMatch])

  useEffect(() => {
    const el = scrollRef.current
    if (el && follow && streaming && activeTab) el.scrollTop = el.scrollHeight
  }, [visible, follow, streaming, activeTab])

  useEffect(() => {
    if (!active || paused || (ideDock && !activeTab)) return undefined
    function onKeyDown(e) {
      if (!(e.metaKey || e.ctrlKey) || e.key.toLowerCase() !== 'f') return
      if (e.altKey) return
      if (isEditableTarget(e.target) && !e.target?.closest?.('.terminal-find-bar')) return
      e.preventDefault()
      e.stopPropagation()
      setFindOpen(true)
    }
    window.addEventListener('keydown', onKeyDown, true)
    return () => window.removeEventListener('keydown', onKeyDown, true)
  }, [active, paused, ideDock, activeTab])

  useEffect(() => {
    if (!logsFindFromPatterns?.id || (ideDock && !activeTab)) return
    const q = String(logsFindFromPatterns.query || '').trim()
    setFindQuery(q)
    setFindOpen(true)
    setFindIndex(-1)
    onLogsFindFromPatternsHandled?.()
  }, [logsFindFromPatterns, onLogsFindFromPatternsHandled, ideDock, activeTab])

  const closeFind = useCallback(() => {
    setFindOpen(false)
    setFindQuery('')
    setFindIndex(-1)
  }, [])

  const findNext = useCallback(() => {
    if (findCount === 0) return
    setFindIndex((i) => (i + 1) % findCount)
  }, [findCount])

  const findPrevious = useCallback(() => {
    if (findCount === 0) return
    setFindIndex((i) => (i - 1 + findCount) % findCount)
  }, [findCount])

  const handleFindQueryChange = useCallback((q) => {
    setFindQuery(q)
    setFindIndex(-1)
  }, [])

  function toggleMuted(key) {
    setMuted((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }

  const chips = []
  if (status.dropped > 0) {
    chips.push({
      key: 'dropped',
      label: `−${status.dropped}`,
      warn: true,
      title: 'Lines dropped because they arrived faster than the panel could take them',
    })
  }

  return (
    <section
      className={[
        'stream-panel',
        'stream-panel-embedded',
        ideDock ? 'stream-panel-ide-dock' : '',
        'expanded',
        streamDense ? 'stream-dense' : '',
        streamWrapLines ? 'stream-wrap' : '',
      ].filter(Boolean).join(' ')}
      style={{ '--stream-font-size': `${streamFontSize}px` }}
    >
      {ideDock ? (
        <header className="stream-header stream-header-ide-sub">
          <div className="stream-ide-sub-meta">
            <StreamLiveBadge running={active} logTailEngaged={active} logTailPaused={paused} />
            {active && (
              <span className="log-stream-sources" title={(status.sources || []).map((s) => `${s.label} (${s.namespace})`).join('\n')}>
                {describeSources(status.sources)}
              </span>
            )}
            {!active && <span className="stream-ide-sub-hint muted">No stream — pick pods and Stream logs, or ⌘⇧L</span>}
            {chips.length > 0 && (
              <span className="stream-chips">
                {chips.map((chip) => (
                  <span
                    key={chip.key}
                    title={chip.title}
                    className={['stream-chip', chip.mono ? 'mono' : '', chip.warn ? 'warn' : ''].filter(Boolean).join(' ')}
                  >
                    {chip.label}
                  </span>
                ))}
              </span>
            )}
            {active && lines.length > 0 && <span className="stream-count">{visible.length}</span>}
          </div>
          <div className="stream-toolbar">
            {active && (
              <>
                <button
                  type="button"
                  className="btn btn-ghost btn-sm stream-stop-btn"
                  onClick={() => logStream.stop()}
                  disabled={busy}
                  title="Stop streaming and discard the lines"
                >
                  Stop
                </button>
                <button
                  type="button"
                  className="stream-icon-btn"
                  onClick={() => logStream.clear()}
                  title="Clear the panel (the stream keeps running)"
                  aria-label="Clear lines"
                >
                  <IconClear />
                </button>
                <button
                  type="button"
                  className={`stream-icon-btn ${paused ? 'paused' : 'live'}`}
                  onClick={() => (paused ? logStream.resume() : logStream.pause())}
                  disabled={busy}
                  title={paused ? 'Resume — replays what was logged while paused' : 'Pause — closes the log connections'}
                  aria-label={paused ? 'Resume stream' : 'Pause stream'}
                  aria-pressed={paused}
                >
                  {paused ? <IconPlay /> : <IconPause />}
                </button>
                <button
                  type="button"
                  className={`stream-icon-btn ${showTime ? 'active' : ''}`}
                  onClick={() => setShowTime((v) => !v)}
                  title={showTime ? 'Hide timestamps' : 'Show timestamps'}
                  aria-label="Timestamps"
                  aria-pressed={showTime}
                >
                  <IconClock />
                </button>
                {streaming && (
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
                <button
                  type="button"
                  className="btn btn-ghost btn-sm"
                  onClick={() => setFindOpen(true)}
                  title="Find in logs (⌘F)"
                >
                  Find
                </button>
              </>
            )}
            {dockControls}
          </div>
        </header>
      ) : (
        <header className="stream-header">
          <div className="stream-title">
            <svg className="stream-doc-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden="true">
              <path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z" />
              <path d="M14 2v6h6" />
            </svg>
            <span>Container logs</span>
            <StreamLiveBadge running={active} logTailEngaged={active} logTailPaused={paused} />
            {active && (
              <span className="log-stream-sources" title={(status.sources || []).map((s) => `${s.label} (${s.namespace})`).join('\n')}>
                {describeSources(status.sources)}
              </span>
            )}
            {chips.length > 0 && (
              <span className="stream-chips">
                {chips.map((chip) => (
                  <span
                    key={chip.key}
                    title={chip.title}
                    className={['stream-chip', chip.mono ? 'mono' : '', chip.warn ? 'warn' : ''].filter(Boolean).join(' ')}
                  >
                    {chip.label}
                  </span>
                ))}
              </span>
            )}
            {active && lines.length > 0 && <span className="stream-count">{visible.length}</span>}
          </div>

          <div className="stream-toolbar">
            {active && (
              <>
                <button
                  type="button"
                  className="btn btn-ghost btn-sm stream-stop-btn"
                  onClick={() => logStream.stop()}
                  disabled={busy}
                  title="Stop streaming and discard the lines"
                >
                  Stop
                </button>
                <button
                  type="button"
                  className="stream-icon-btn"
                  onClick={() => logStream.clear()}
                  title="Clear the panel (the stream keeps running)"
                  aria-label="Clear lines"
                >
                  <IconClear />
                </button>
                <button
                  type="button"
                  className={`stream-icon-btn ${paused ? 'paused' : 'live'}`}
                  onClick={() => (paused ? logStream.resume() : logStream.pause())}
                  disabled={busy}
                  title={paused ? 'Resume — replays what was logged while paused' : 'Pause — closes the log connections'}
                  aria-label={paused ? 'Resume stream' : 'Pause stream'}
                  aria-pressed={paused}
                >
                  {paused ? <IconPlay /> : <IconPause />}
                </button>
                <button
                  type="button"
                  className={`stream-icon-btn ${showTime ? 'active' : ''}`}
                  onClick={() => setShowTime((v) => !v)}
                  title={showTime ? 'Hide timestamps' : 'Show timestamps'}
                  aria-label="Timestamps"
                  aria-pressed={showTime}
                >
                  <IconClock />
                </button>
                {streaming && (
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
                <button
                  type="button"
                  className="btn btn-ghost btn-sm"
                  onClick={() => setFindOpen(true)}
                  title="Find in logs (⌘F)"
                >
                  Find
                </button>
              </>
            )}
            {dockControls}
          </div>
        </header>
      )}

      <div className="stream-body">
        <TerminalFindBar
          open={findOpen}
          query={findQuery}
          resultIndex={findIndex}
          resultCount={findCount}
          onQueryChange={handleFindQueryChange}
          onFindNext={findNext}
          onFindPrevious={findPrevious}
          onClose={closeFind}
          className="terminal-find-bar log-stream-find-bar"
          placeholder="Find in logs…"
          ariaLabel="Find in logs"
          roleLabel="Find in logs"
        />
        {!active ? (
          <StreamPicker
            running={running}
            pods={investigationPods}
            namespace={investigationNamespace}
            busy={busy}
            error={error}
            onStart={(names) => logStream.start({ targets: podTargets(investigationNamespace, names), tailLines })}
          />
        ) : (
          <>
            {sources.length > 1 && (
              <div className="log-stream-pods" role="group" aria-label="Pods in the stream">
                {sources.map((s) => {
                  const off = muted.has(s.key)
                  return (
                    <button
                      key={s.key}
                      type="button"
                      className={`log-stream-pod log-pod-c${podColor(s.key)} ${off ? 'is-muted' : ''}`}
                      aria-pressed={!off}
                      title={off ? `Show ${s.key}` : `Hide ${s.key}`}
                      onClick={() => toggleMuted(s.key)}
                    >
                      <span className="log-stream-pod-dot" aria-hidden="true" />
                      <span className="mono">{s.pod}</span>
                      <span className="log-stream-pod-container">{s.container}</span>
                      <span className="log-stream-pod-count">{s.count}</span>
                    </button>
                  )
                })}
              </div>
            )}
            {(status.notices || []).length > 0 && (
              <p className="stream-live-note" title={status.notices.join('\n')}>
                {status.notices[status.notices.length - 1]}
              </p>
            )}
            {error && <p className="log-request-error">{error}</p>}
            <div className="stream-scroll log-stream-scroll" ref={scrollRef}>
              {(!ideDock || activeTab) && !visible.length && (
                <div className="empty-stream">
                  {lines.length
                    ? 'No lines (all sources hidden). Show a pod above.'
                    : paused
                      ? 'Paused. Resume to reopen the log connections.'
                      : 'Waiting for log lines…'}
                </div>
              )}
              {(!ideDock || activeTab) && visible.map((l) => (
                <LogStreamRow
                  key={l.seq}
                  line={l}
                  showTime={showTime}
                  findQuery={findOpen ? findQuery.trim() : ''}
                  isFindActive={
                    Boolean(findOpen && activeMatch && activeMatch.row.seq === l.seq)
                  }
                  activeRange={
                    findOpen && activeMatch && activeMatch.row.seq === l.seq
                      ? { start: activeMatch.start, end: activeMatch.end }
                      : null
                  }
                />
              ))}
            </div>
          </>
        )}
      </div>
    </section>
  )
}

function LogStreamRow({ line: l, showTime, findQuery, isFindActive, activeRange }) {
  const ranges = findQuery ? findSubstringRanges(l.text, findQuery) : []
  return (
    <div
      data-log-seq={l.seq}
      className={[
        'log-stream-row',
        `sev-${l.severity || 'info'}`,
        showTime ? '' : 'no-time',
        isFindActive ? 'is-find-active' : '',
      ].filter(Boolean).join(' ')}
    >
      {showTime && <span className="log-stream-time mono">{formatLineTime(l.timestamp)}</span>}
      <span className={`log-stream-source mono log-pod-c${podColor(sourceKey(l))}`} title={`${l.namespace}/${l.pod} · ${l.container}`}>
        {l.pod}
        <span className="log-stream-container">/{l.container}</span>
      </span>
      <span className="log-stream-marker" aria-hidden="true">{severityMark(l.severity)}</span>
      <span className="log-stream-text">
        {ranges.length === 0 ? (
          l.text
        ) : (
          renderHighlightedText(l.text, ranges, activeRange)
        )}
      </span>
    </div>
  )
}

function renderHighlightedText(text, ranges, activeRange) {
  if (!ranges.length) return text
  const parts = []
  let pos = 0
  for (let i = 0; i < ranges.length; i += 1) {
    const { start, end } = ranges[i]
    if (start > pos) parts.push(<span key={`t-${pos}`}>{text.slice(pos, start)}</span>)
    const isActive = activeRange && activeRange.start === start && activeRange.end === end
    parts.push(
      <mark
        key={`m-${start}-${end}-${i}`}
        className={isActive ? 'log-find-mark log-find-mark-active' : 'log-find-mark'}
      >
        {text.slice(start, end)}
      </mark>,
    )
    pos = end
  }
    if (pos < text.length) parts.push(<span key={`t-end-${pos}`}>{text.slice(pos)}</span>)
  return parts
}

/** Shown when nothing is streaming: where to start, plus a shortcut for the investigation's pods. */
function StreamPicker({ running, pods, namespace, busy, error, onStart }) {
  const names = (pods || []).map((p) => p?.name).filter(Boolean)
  return (
    <div className="stream-scroll">
      <div className="empty-stream">
        <strong>Container logs</strong>
        <span>
          Nothing is streaming. In Resources, select a pod, workload or Service and click Stream logs,
          or use the checkboxes (or ⌘-click rows) and press ⌘⇧L.
        </span>
        {running && names.length > 0 && (
          <button
            type="button"
            className="btn btn-primary btn-sm"
            disabled={busy}
            onClick={() => onStart(names)}
            title={`Stream logs from the ${names.length} pod${names.length === 1 ? '' : 's'} in this investigation; their lines also feed Patterns`}
          >
            {busy ? 'Starting…' : `Stream the investigation's ${names.length} pod${names.length === 1 ? '' : 's'} in ${namespace}`}
          </button>
        )}
        {error && <span className="log-request-error">{error}</span>}
      </div>
    </div>
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

function IconClock() {
  return (
    <svg viewBox="0 0 16 16" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="1.5" aria-hidden="true">
      <circle cx="8" cy="8" r="5.5" />
      <path d="M8 5v3.2l2 1.3" />
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
