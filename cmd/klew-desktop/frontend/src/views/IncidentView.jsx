import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { StatusStrip } from '../components/incident/StatusStrip'
import { ScopePanel } from '../components/incident/ScopePanel'
import { ComponentInspectPanel } from '../components/incident/ComponentInspectPanel'
import { CollectingMatchesSplash } from '../components/incident/CollectingMatchesSplash'
import {
  deriveMatchRows,
  getMatchedObjects,
  pickDefaultFocus,
  scopeStatus,
} from '../lib/matches'
import { buildChainRows, buildFocusScope } from '../lib/focusScope'
import { buildComponentInspect } from '../lib/componentInspect'
import { mergeInspect, normalizeObjectDetail } from '../lib/objectDetails'
import { GetObjectDetails } from '../../wailsjs/go/main/App'
import {
  inspectShowsFocusCta,
  listChromeForMode,
  loadLayoutMode,
  loadListWidth,
  saveListWidth,
} from '../lib/incidentLayout'

export function IncidentView({
  view,
  focusKey,
  focusPinned,
  onFocusChange,
  onClearFocus,
  collecting,
  layoutMode: layoutModeProp,
}) {
  const s = view.summary || {}
  const allMatches = getMatchedObjects(view)
  const allRows = useMemo(() => deriveMatchRows(view, allMatches), [view, allMatches])
  const scope = useMemo(() => scopeStatus(allRows), [allRows])

  const layoutMode = layoutModeProp || loadLayoutMode()
  const [inspectKey, setInspectKey] = useState(null)
  const [listWidth, setListWidth] = useState(() => loadListWidth(layoutMode))
  const splitRef = useRef(null)
  const dragRef = useRef(null)

  const startColResize = useCallback((e) => {
    e.preventDefault()
    const startX = e.clientX
    const startW = listWidth
    const containerW = splitRef.current?.clientWidth || 800
    dragRef.current = { startX, startW, containerW }

    function onMove(ev) {
      if (!dragRef.current) return
      const delta = ev.clientX - dragRef.current.startX
      const max = Math.max(220, Math.floor(dragRef.current.containerW * 0.55))
      const next = Math.min(max, Math.max(200, dragRef.current.startW + delta))
      setListWidth(next)
    }

    function onUp() {
      dragRef.current = null
      window.removeEventListener('mousemove', onMove)
      window.removeEventListener('mouseup', onUp)
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
      setListWidth((w) => {
        saveListWidth(w)
        return w
      })
    }

    document.body.style.cursor = 'col-resize'
    document.body.style.userSelect = 'none'
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', onUp)
  }, [listWidth])

  // Keep list width inside the split when the window scales.
  useEffect(() => {
    function clampWidth() {
      const containerW = splitRef.current?.clientWidth
      if (!containerW) return
      const max = Math.max(220, Math.floor(containerW * 0.55))
      setListWidth((w) => {
        const next = Math.min(max, Math.max(200, w))
        if (next !== w) saveListWidth(next)
        return next
      })
    }
    clampWidth()
    window.addEventListener('resize', clampWidth)
    return () => window.removeEventListener('resize', clampWidth)
  }, [])

  useEffect(() => {
    if (focusPinned) return
    if (!allRows.length) {
      setInspectKey(null)
      return
    }
    if (inspectKey && allRows.some((r) => r.key === inspectKey)) return
    const preferred = focusKey && allRows.some((r) => r.key === focusKey)
      ? focusKey
      : pickDefaultFocus(allRows)
    setInspectKey(preferred)
    if (!focusKey) onFocusChange?.(preferred, { pinned: false })
  }, [allRows, focusPinned, focusKey, inspectKey, onFocusChange])

  useEffect(() => {
    if (focusPinned && focusKey) setInspectKey(focusKey)
  }, [focusPinned, focusKey])

  const focusRow = allRows.find((r) => r.key === focusKey) || allRows[0] || null
  const drillDown = useMemo(
    () => (focusPinned && focusRow ? buildFocusScope(view, focusRow) : null),
    [focusPinned, focusRow, view],
  )

  const rows = useMemo(
    () => (drillDown?.active ? buildChainRows(view, drillDown, allRows) : allRows),
    [drillDown, view, allRows],
  )

  useEffect(() => {
    if (!rows.length) return
    if (inspectKey && rows.some((r) => r.key === inspectKey)) return
    setInspectKey(focusPinned ? (focusKey || rows[0].key) : (pickDefaultFocus(rows) || rows[0].key))
  }, [rows, inspectKey, focusPinned, focusKey])

  const inspectRow = useMemo(() => {
    return rows.find((r) => r.key === inspectKey)
      || rows.find((r) => r.key === focusKey)
      || rows[0]
      || null
  }, [rows, inspectKey, focusKey])

  const snapshotInspect = useMemo(
    () => (inspectRow ? buildComponentInspect(view, inspectRow) : null),
    [view, inspectRow],
  )

  const [liveDetail, setLiveDetail] = useState(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detailError, setDetailError] = useState(null)

  useEffect(() => {
    if (!inspectRow) {
      setLiveDetail(null)
      setDetailError(null)
      setDetailLoading(false)
      return
    }
    const kind = inspectRow.kind || inspectRow.ref?.kind
    const name = inspectRow.name || inspectRow.ref?.name
    const ns = inspectRow.namespace || inspectRow.ref?.namespace || ''
    if (!kind || !name) return

    let cancelled = false
    setDetailLoading(true)
    setDetailError(null)
    GetObjectDetails(kind, name, ns)
      .then((detail) => {
        if (cancelled) return
        setLiveDetail(normalizeObjectDetail(detail, inspectRow))
        setDetailLoading(false)
      })
      .catch((err) => {
        if (cancelled) return
        setLiveDetail(null)
        setDetailError(String(err?.message || err || 'Failed to load details'))
        setDetailLoading(false)
      })
    return () => { cancelled = true }
    // Do NOT depend on view.updatedAt — live evidence ticks would re-fetch details
    // on every log line and freeze the UI across panels.
  }, [inspectRow?.key, inspectRow?.kind, inspectRow?.name, inspectRow?.namespace])

  const inspect = useMemo(
    () => mergeInspect(liveDetail, snapshotInspect),
    [liveDetail, snapshotInspect],
  )

  const listChrome = listChromeForMode(layoutMode)
  const showFocusCta = inspectShowsFocusCta(layoutMode)

  const displayRows = useMemo(() => {
    if (layoutMode !== 'signal-first' || focusPinned) return rows
    const rank = { critical: 0, degraded: 1, warning: 1, healthy: 2, unknown: 3 }
    return [...rows].sort((a, b) => (rank[a.status] ?? 3) - (rank[b.status] ?? 3))
  }, [rows, layoutMode, focusPinned])

  const handleInspect = (key) => {
    setInspectKey(key)
    if (!focusPinned) onFocusChange?.(key, { pinned: false })
  }

  const handleFocus = (key) => {
    onFocusChange?.(key, { pinned: true })
  }

  if (allMatches.length === 0 && !allRows.length) {
    return (
      <div className="incident-page">
        {collecting ? (
          <CollectingMatchesSplash />
        ) : (
          <div className="incident-empty">
            <p>No matches in namespace for this query.</p>
            <p className="muted">Try a different search or use deploy/name to target one kind.</p>
          </div>
        )}
      </div>
    )
  }

  return (
    <div className={`incident-page layout-${layoutMode}`}>
      <div className="incident-page-chrome">
        <StatusStrip
          scope={scope}
          summary={s}
          rows={allRows}
          drillDown={drillDown}
          focusRow={focusRow}
          view={view}
        />

        {layoutMode === 'unified-select' && inspectRow && (
          <div className="incident-unified-chrome">
            <span className="muted">
              Inspecting <strong>{inspectRow.kind}/{inspectRow.name}</strong>
            </span>
            {focusPinned ? (
              <button
                type="button"
                className="btn btn-outline btn-sm"
                onClick={() => onClearFocus?.()}
              >
                Clear Focus
              </button>
            ) : (
              <button
                type="button"
                className="btn btn-outline btn-sm"
                onClick={() => handleFocus(inspectRow.key)}
              >
                Focus chain
              </button>
            )}
          </div>
        )}
      </div>

      <div
        ref={splitRef}
        className={`incident-two-col inspecting mode-${layoutMode}`}
        style={{ '--incident-list-w': `${listWidth}px` }}
      >
        <section className="card incident-card incident-list-card scope-list-card">
          <div className="card-body card-body-flush incident-list-scroll scope-list-scroll">
            <ScopePanel
              rows={displayRows}
              view={view}
              focusKey={focusKey}
              inspectKey={inspectKey}
              mode={focusPinned ? 'chain' : 'match'}
              onInspect={handleInspect}
              onFocus={handleFocus}
              showFocusButton={listChrome.showFocusButton}
              title={focusPinned ? 'Focus chain' : 'Scope'}
            />
          </div>
        </section>

        <div
          className="incident-col-resize"
          role="separator"
          aria-orientation="vertical"
          aria-label="Resize matched components panel"
          title="Drag to resize"
          onMouseDown={startColResize}
        />

        <section className="card incident-card incident-inspect-card">
          <div className="card-title-row">
            <h3>Signals & details</h3>
            {inspect && layoutMode === 'current' && (
              <span className="muted matched-hint">{inspect.kind}/{inspect.name}</span>
            )}
          </div>
          <div className="card-body inspect-card-body">
            <ComponentInspectPanel
              inspect={inspect}
              layoutMode={layoutMode}
              focusPinned={focusPinned}
              showFocusCta={showFocusCta}
              onFocus={handleFocus}
              loading={detailLoading}
              error={detailError}
              emptyHint={
                focusPinned
                  ? 'Select a component in the focus chain to inspect it.'
                  : 'Select a matched component to see signals and details. Use Focus to isolate its related resources.'
              }
            />
          </div>
        </section>
      </div>
    </div>
  )
}

export function incidentMetaLine(view, cluster, matchCount) {
  const s = view.summary || {}
  const rawQuery = s.query ?? ''
  const query = rawQuery.trim() ? rawQuery : '(all)'
  const ns = s.namespace || cluster?.selectedNamespace || '—'
  const n = matchCount ?? getMatchedObjects(view).length
  if (n === 0) {
    return `${query} · ${ns} · no matches`
  }
  return `${query} · ${ns} · ${n} match${n !== 1 ? 'es' : ''}`
}
